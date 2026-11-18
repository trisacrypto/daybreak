package db_test

import (
	"context"
	"database/sql"
	"time"

	"github.com/trisacrypto/daybreak/pkg/db"
	"github.com/trisacrypto/daybreak/pkg/db/fields"
	"github.com/trisacrypto/daybreak/pkg/errors"
	"go.rtnl.ai/ulid"
)

//cSpell:ignore Annika Lindqvist Norrsken

func (s *Suite) TestLicenses() {
	s.Require().NoError(s.LoadFixtures(), "could not load fixtures")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Licenses are not part of the daybreak dataset, so no license operation should
	// change the last modified timestamp of the database. Loading fixtures does not
	// touch the daybreak table, so set the timestamp to have something to compare to.
	// NOTE: this is the modified timestamp of the daybreak dataset, which is distinct
	// from the modified timestamp of a license that is changed by license writes.
	s.Require().NoError(s.db.UpdateModified(ctx), "could not set last modified")
	lastModified, err := s.db.LastModified(ctx)
	s.Require().NoError(err)
	s.Require().False(lastModified.IsZero())

	// Every subtest must call unmodified after its license operations are complete.
	unmodified := func(action string) {
		require := s.Require()

		modified, err := s.db.LastModified(ctx)
		require.NoError(err)
		require.True(lastModified.Equal(modified), "expected %s to not change the daybreak last modified timestamp", action)
	}

	// The license created in CreateLicense is shared by the subtests that follow it.
	license := &db.License{
		Name:        "Annika Lindqvist",
		Email:       "annika.lindqvist@norrskenpay.se",
		Description: sql.NullString{String: "Production Envoy node for Norrsken Pay AB.", Valid: true},
		Permissions: fields.StringArray{"daybreak:sync", "daybreak:download"},
		Expires:     fields.TimeNow().Add(365 * 24 * time.Hour),
	}

	// Timestamps are truncated to milliseconds, so wait long enough between writes
	// to guarantee that a changed timestamp is strictly after the previous one.
	tick := func() { time.Sleep(2 * time.Millisecond) }

	s.Run("ListLicenses", func() {
		require := s.Require()
		tick()

		licenses, err := s.db.ListLicenses(ctx)
		require.NoError(err)
		require.Len(licenses, 8)

		for _, license := range licenses {
			require.False(license.ID.IsZero())
			require.Len(license.Secret, 32)
		}

		unmodified("list")
	})

	// NOTE: if create license fails all of the subtests that follow it will fail.
	s.Run("CreateLicense", func() {
		require := s.Require()
		tick()

		err := s.db.CreateLicense(ctx, license)
		require.NoError(err)
		require.False(license.ID.IsZero())
		require.Len(license.Secret, 32, "expected a secret to be generated")
		require.False(license.Created.IsZero())
		require.True(license.Modified.Equal(license.Created))

		// A license with a secret that is not 32 bytes cannot be created.
		invalid := &db.License{
			Secret:  []byte("too short"),
			Name:    "Invalid Secret",
			Email:   "invalid@norrskenpay.se",
			Expires: license.Expires,
		}

		err = s.db.CreateLicense(ctx, invalid)
		require.ErrorIs(err, errors.ErrInvalidLicenseSecret)

		licenses, err := s.db.ListLicenses(ctx)
		require.NoError(err)
		require.Len(licenses, 9)

		unmodified("create")
	})

	s.Run("RetrieveLicense", func() {
		require := s.Require()
		tick()

		actual, err := s.db.RetrieveLicense(ctx, license.ID)
		require.NoError(err)

		require.Equal(license.ID, actual.ID)
		require.Equal(license.Secret, actual.Secret)
		require.Equal(license.Name, actual.Name)
		require.Equal(license.Email, actual.Email)
		require.Equal(license.Description, actual.Description)
		require.True(license.Permissions.Equal(actual.Permissions))
		require.Zero(actual.Syncs)
		require.Zero(actual.Downloads)
		require.True(actual.LastAccess.IsZero())
		require.True(license.Expires.Equal(actual.Expires))
		require.True(license.Created.Equal(actual.Created))
		require.True(license.Modified.Equal(actual.Modified))

		unmodified("retrieve")
	})

	s.Run("UpdateLicense", func() {
		require := s.Require()
		tick()

		created, previous := license.Created, license.Modified
		license.Name = "Annika Lindqvist-Berg"
		license.Permissions = fields.StringArray{"counterparties:admin", "daybreak:sync", "daybreak:download"}

		// The usage statistics cannot be changed by an update: they are only modified
		// by LicenseAccessed, IncrementLicenseSyncs, and IncrementLicenseDownloads.
		license.Syncs = 3
		license.Downloads = 1
		license.LastAccess = fields.TimeNow()

		err := s.db.UpdateLicense(ctx, license)
		require.NoError(err)
		require.True(license.Modified.After(previous))

		actual, err := s.db.RetrieveLicense(ctx, license.ID)
		require.NoError(err)
		require.Equal("Annika Lindqvist-Berg", actual.Name)
		require.True(license.Permissions.Equal(actual.Permissions))
		require.Zero(actual.Syncs, "expected update to not change syncs")
		require.Zero(actual.Downloads, "expected update to not change downloads")
		require.True(actual.LastAccess.IsZero(), "expected update to not change last access")
		require.Equal(license.Secret, actual.Secret)
		require.True(created.Equal(actual.Created))
		require.True(license.Modified.Equal(actual.Modified))

		// Updating a license that does not exist returns not found.
		err = s.db.UpdateLicense(ctx, &db.License{ID: ulid.Make(), Name: "Unknown", Email: "unknown@norrskenpay.se", Expires: license.Expires})
		require.ErrorIs(err, errors.ErrNotFound)

		// Reset the shared license to the state stored in the database.
		license.Syncs, license.Downloads, license.LastAccess = 0, 0, fields.Timestamp{}

		unmodified("update")
	})

	// usage returns the total syncs and downloads of every license except the shared
	// license, to check that the usage methods only affect the specified license.
	usage := func() (syncs, downloads int64) {
		licenses, err := s.db.ListLicenses(ctx)
		s.Require().NoError(err)

		for _, other := range licenses {
			if other.ID != license.ID {
				syncs += other.Syncs
				downloads += other.Downloads
			}
		}
		return syncs, downloads
	}

	s.Run("LicenseAccessed", func() {
		require := s.Require()
		tick()

		before := fields.TimeNow()
		err := s.db.LicenseAccessed(ctx, license.ID)
		require.NoError(err)

		actual, err := s.db.RetrieveLicense(ctx, license.ID)
		require.NoError(err)
		require.False(actual.LastAccess.IsZero(), "expected last access to be set")
		require.False(actual.LastAccess.Before(before), "expected last access to be the time of the call")
		require.True(actual.Modified.After(license.Modified), "expected access to move modified forward")
		require.True(actual.Modified.Equal(actual.LastAccess), "expected modified to be the last access timestamp")

		// Each access moves the last access and modified timestamps forward.
		tick()
		previous := actual.LastAccess
		err = s.db.LicenseAccessed(ctx, license.ID)
		require.NoError(err)

		actual, err = s.db.RetrieveLicense(ctx, license.ID)
		require.NoError(err)
		require.True(actual.LastAccess.After(previous), "expected last access to move forward")
		require.True(actual.Modified.Equal(actual.LastAccess), "expected modified to be the last access timestamp")

		// Nothing else about the license is changed.
		require.Equal(license.Name, actual.Name)
		require.Zero(actual.Syncs)
		require.Zero(actual.Downloads)
		require.True(license.Expires.Equal(actual.Expires))
		require.True(license.Created.Equal(actual.Created), "expected access to not change created")

		// Accessing a license that does not exist returns not found.
		err = s.db.LicenseAccessed(ctx, ulid.Make())
		require.ErrorIs(err, errors.ErrNotFound)

		license.LastAccess, license.Modified = actual.LastAccess, actual.Modified
		unmodified("license accessed")
	})

	s.Run("IncrementLicenseSyncs", func() {
		require := s.Require()
		tick()

		otherSyncs, otherDownloads := usage()

		// Each increment adds one to syncs and moves the modified timestamp forward.
		previous := license.Modified
		for i := int64(1); i <= 3; i++ {
			tick()
			err := s.db.IncrementLicenseSyncs(ctx, license.ID)
			require.NoError(err)

			actual, err := s.db.RetrieveLicense(ctx, license.ID)
			require.NoError(err)
			require.Equal(i, actual.Syncs, "expected syncs to be incremented by one")
			require.True(actual.Modified.After(previous), "expected syncs to move modified forward")
			previous = actual.Modified
		}

		// Nothing else about the license is changed.
		actual, err := s.db.RetrieveLicense(ctx, license.ID)
		require.NoError(err)
		require.Zero(actual.Downloads, "expected syncs to not change downloads")
		require.True(license.LastAccess.Equal(actual.LastAccess), "expected syncs to not change last access")
		require.True(license.Created.Equal(actual.Created), "expected syncs to not change created")

		// Incrementing a license that does not exist returns not found.
		err = s.db.IncrementLicenseSyncs(ctx, ulid.Make())
		require.ErrorIs(err, errors.ErrNotFound)

		// No other license is affected.
		syncs, downloads := usage()
		require.Equal(otherSyncs, syncs, "expected syncs of other licenses to be unchanged")
		require.Equal(otherDownloads, downloads, "expected downloads of other licenses to be unchanged")

		license.Syncs, license.Modified = actual.Syncs, actual.Modified
		unmodified("increment syncs")
	})

	s.Run("IncrementLicenseDownloads", func() {
		require := s.Require()
		tick()

		otherSyncs, otherDownloads := usage()

		// Each increment adds one to downloads and moves the modified timestamp forward.
		previous := license.Modified
		for i := int64(1); i <= 2; i++ {
			tick()
			err := s.db.IncrementLicenseDownloads(ctx, license.ID)
			require.NoError(err)

			actual, err := s.db.RetrieveLicense(ctx, license.ID)
			require.NoError(err)
			require.Equal(i, actual.Downloads, "expected downloads to be incremented by one")
			require.True(actual.Modified.After(previous), "expected downloads to move modified forward")
			previous = actual.Modified
		}

		// Nothing else about the license is changed.
		actual, err := s.db.RetrieveLicense(ctx, license.ID)
		require.NoError(err)
		require.Equal(license.Syncs, actual.Syncs, "expected downloads to not change syncs")
		require.True(license.LastAccess.Equal(actual.LastAccess), "expected downloads to not change last access")
		require.True(license.Created.Equal(actual.Created), "expected downloads to not change created")

		// Incrementing a license that does not exist returns not found.
		err = s.db.IncrementLicenseDownloads(ctx, ulid.Make())
		require.ErrorIs(err, errors.ErrNotFound)

		// No other license is affected.
		syncs, downloads := usage()
		require.Equal(otherSyncs, syncs, "expected syncs of other licenses to be unchanged")
		require.Equal(otherDownloads, downloads, "expected downloads of other licenses to be unchanged")

		license.Downloads, license.Modified = actual.Downloads, actual.Modified
		unmodified("increment downloads")
	})

	s.Run("DeleteLicense", func() {
		require := s.Require()
		tick()

		err := s.db.DeleteLicense(ctx, license.ID)
		require.NoError(err)

		_, err = s.db.RetrieveLicense(ctx, license.ID)
		require.ErrorIs(err, errors.ErrNotFound)

		licenses, err := s.db.ListLicenses(ctx)
		require.NoError(err)
		require.Len(licenses, 8)

		unmodified("delete")
	})
}
