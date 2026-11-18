package db_test

import (
	"context"
	"database/sql"
	"time"

	"github.com/trisacrypto/daybreak/pkg/db"
	"github.com/trisacrypto/daybreak/pkg/db/fields"
	"github.com/trisacrypto/daybreak/pkg/errors"
)

func (s *Suite) TestLicenses() {
	s.Require().NoError(s.LoadFixtures(), "could not load fixtures")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Licenses are not part of the daybreak dataset, so no license operation should
	// change the last modified timestamp of the database. Loading fixtures does not
	// touch the daybreak table, so set the timestamp to have something to compare to.
	s.Require().NoError(s.db.UpdateModified(ctx), "could not set last modified")
	lastModified, err := s.db.LastModified(ctx)
	s.Require().NoError(err)
	s.Require().False(lastModified.IsZero())

	unmodified := func(action string) {
		require := s.Require()

		modified, err := s.db.LastModified(ctx)
		require.NoError(err)
		require.True(lastModified.Equal(modified), "expected %s to not change last modified", action)
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
		require.Equal(int64(3), actual.Syncs)
		require.Equal(int64(1), actual.Downloads)
		require.True(license.LastAccess.Equal(actual.LastAccess))
		require.Equal(license.Secret, actual.Secret)
		require.True(created.Equal(actual.Created))
		require.True(license.Modified.Equal(actual.Modified))

		unmodified("update")
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
