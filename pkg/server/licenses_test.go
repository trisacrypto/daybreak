package server

import (
	"context"
	"database/sql"
	"time"

	"github.com/trisacrypto/daybreak/pkg/api/v1"
	"github.com/trisacrypto/daybreak/pkg/db"
	"github.com/trisacrypto/daybreak/pkg/db/fields"
	"go.rtnl.ai/ulid"
)

// cSpell:ignore Annika Lindqvist norrskenpay Norrsken Oskar Nyberg kustbanken
func (s *Suite) TestLicenseResource() {
	s.T().Cleanup(s.ResetDB)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s.Run("ListLicenses", func() {
		require := s.Require()
		licenses, err := s.client.ListLicenses(ctx)
		require.NoError(err, "could not list licenses")
		require.Equal(8, len(licenses), "should have 8 licenses")
	})

	s.Run("CreateLicense", func() {
		before, err := s.client.ListLicenses(ctx)
		s.Require().NoError(err, "could not list licenses")

		s.Run("Success", func() {
			require := s.Require()
			license := &api.License{
				Name:        "Annika Lindqvist",
				Email:       "annika.lindqvist@norrskenpay.se",
				Description: "Production Envoy node for Norrsken Pay AB.",
				Permissions: []string{"daybreak:sync", "daybreak:download"},
				Expires:     api.Expiration(fields.Time(time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC))),
			}

			created, err := s.client.CreateLicense(ctx, license)
			require.NoError(err, "could not create license")

			require.Equal(created.Clone(), license, "created license should be the same as the input without read-only fields")
			require.False(created.ID.IsZero(), "id should be set")
			require.NotZero(created.Created, "created should be set")
			require.Equal(created.Created, created.Modified, "modified should be the created timestamp")
			require.Zero(created.Syncs, "syncs should be zero")
			require.Zero(created.Downloads, "downloads should be zero")
			require.Nil(created.LastAccess, "last access should not be set")

			retrieved, err := s.client.RetrieveLicense(ctx, created.ID)
			require.NoError(err, "could not retrieve created license")
			require.Equal(created, retrieved, "retrieved license should be the created license")
		})

		s.Run("Defaults", func() {
			require := s.Require()
			license := &api.License{
				Name:        "  Oskar Nyberg\n",
				Email:       "\toskar.nyberg@kustbanken.se ",
				Permissions: []string{" Daybreak:Sync", "COUNTERPARTIES:CREATE"},
			}

			// A license without an expiration expires one year from now, on the hour.
			earliest := time.Now().UTC().AddDate(1, 0, 0).Truncate(time.Hour)
			created, err := s.client.CreateLicense(ctx, license)
			latest := time.Now().UTC().AddDate(1, 0, 0).Truncate(time.Hour)
			require.NoError(err, "could not create license")

			require.Equal("Oskar Nyberg", created.Name, "name should be trimmed")
			require.Equal("oskar.nyberg@kustbanken.se", created.Email, "email should be trimmed")
			require.Empty(created.Description, "description should be empty")
			require.Equal([]string{"daybreak:sync", "counterparties:create"}, created.Permissions, "permissions should be normalized")
			require.False(created.Expires.Before(earliest), "expires %s is before %s", created.Expires, earliest)
			require.False(created.Expires.After(latest), "expires %s is after %s", created.Expires, latest)
		})

		s.Run("Empty", func() {
			require := s.Require()
			created, err := s.client.CreateLicense(ctx, &api.License{})
			require.Error(err, "expected error creating license with empty license")
			require.EqualError(err, "[422] 3 validation errors occurred")
			require.Nil(created, "license should be nil")
		})

		s.Run("InvalidEmail", func() {
			require := s.Require()
			created, err := s.client.CreateLicense(ctx, &api.License{
				Name:        "Annika Lindqvist",
				Email:       "not-an-email",
				Permissions: []string{"daybreak:sync"},
			})
			require.Error(err, "expected error creating license with invalid email")
			require.EqualError(err, "[422] invalid email: mail: missing '@' or angle-addr")
			require.Nil(created, "license should be nil")
		})

		s.Run("InvalidPermission", func() {
			require := s.Require()
			created, err := s.client.CreateLicense(ctx, &api.License{
				Name:        "Annika Lindqvist",
				Email:       "annika.lindqvist@norrskenpay.se",
				Permissions: []string{"daybreak:sync", "foo:bar"},
			})
			require.Error(err, "expected error creating license with invalid permission")
			require.EqualError(err, `[422] invalid permissions[1]: "foo:bar" is not a valid permission`)
			require.Nil(created, "license should be nil")
		})

		s.Run("ReadOnly", func() {
			require := s.Require()
			created, err := s.client.CreateLicense(ctx, &api.License{
				ID:          ulid.Make(),
				Name:        "Annika Lindqvist",
				Email:       "annika.lindqvist@norrskenpay.se",
				Permissions: []string{"daybreak:sync"},
				Syncs:       42,
				Downloads:   7,
			})
			require.Error(err, "expected error creating license with read-only fields")
			require.EqualError(err, "[422] 3 validation errors occurred")
			require.Nil(created, "license should be nil")
		})

		after, err := s.client.ListLicenses(ctx)
		s.Require().NoError(err, "could not list licenses")
		s.Require().Len(after, len(before)+2, "only the valid licenses should have been created")
	})

	// The subtests below operate on a license with a known ID and secret that has
	// been used, so that the credentials and usage statistics can be verified.
	// NOTE: if this setup fails all of the subtests that follow it will fail.
	licenseID := ulid.MustParse("01JQ8Z5K3N7M2W4X6Y8A0B1C2D")
	expires := time.Date(2031, time.June, 1, 0, 0, 0, 0, time.UTC)
	{
		require := s.Require()
		license := &db.License{
			ID:          licenseID,
			Secret:      []byte("0123456789abcdef0123456789abcdef"),
			Name:        "Compliance Operations",
			Email:       "compliance@norrskenpay.se",
			Description: sql.NullString{String: "Staging Envoy node for Norrsken Pay AB.", Valid: true},
			Permissions: fields.StringArray{"daybreak:sync", "daybreak:download"},
			Expires:     fields.Time(expires),
		}

		require.NoError(s.server.db.CreateLicense(ctx, license), "could not create license in database")
		require.NoError(s.server.db.IncrementLicenseSyncs(ctx, licenseID), "could not increment license syncs")
		require.NoError(s.server.db.IncrementLicenseSyncs(ctx, licenseID), "could not increment license syncs")
		require.NoError(s.server.db.IncrementLicenseDownloads(ctx, licenseID), "could not increment license downloads")
		require.NoError(s.server.db.LicenseAccessed(ctx, licenseID), "could not mark license accessed")
	}

	s.Run("RetrieveLicense", func() {
		s.Run("Found", func() {
			require := s.Require()
			license, err := s.client.RetrieveLicense(ctx, licenseID)
			require.NoError(err, "could not retrieve license")
			require.Equal(licenseID, license.ID, "license ID should match")
			require.Equal("Compliance Operations", license.Name, "license name should match")
			require.Equal("compliance@norrskenpay.se", license.Email, "license email should match")
			require.Equal("Staging Envoy node for Norrsken Pay AB.", license.Description, "license description should match")
			require.Equal([]string{"daybreak:sync", "daybreak:download"}, license.Permissions, "license permissions should match")
			require.Equal(int64(2), license.Syncs, "license syncs should match")
			require.Equal(int64(1), license.Downloads, "license downloads should match")
			require.NotNil(license.LastAccess, "last access should be set")
			require.False(license.LastAccess.IsZero(), "last access should be set")
			require.True(expires.Equal(license.Expires.Time), "license expires should match")
			require.NotZero(license.Created, "created should be set")
			require.NotZero(license.Modified, "modified should be set")
		})

		s.Run("NotFound", func() {
			require := s.Require()
			license, err := s.client.RetrieveLicense(ctx, ulid.Make())
			require.Error(err, "expected error retrieving license not found")
			require.EqualError(err, "[404] license not found")
			require.Nil(license, "license should be nil")
		})
	})

	s.Run("LicenseCredentials", func() {
		s.Run("Found", func() {
			require := s.Require()
			credentials, err := s.client.LicenseCredentials(ctx, licenseID)
			require.NoError(err, "could not retrieve license credentials")
			require.Equal("01JQ8Z5K3N7M2W4X6Y8A0B1C2D", credentials.ClientID, "client ID should be the license ID")
			require.Equal("MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=", credentials.ClientSecret, "client secret should be the base64 encoded license secret")
		})

		s.Run("NotFound", func() {
			require := s.Require()
			credentials, err := s.client.LicenseCredentials(ctx, ulid.Make())
			require.Error(err, "expected error retrieving credentials for license not found")
			require.EqualError(err, "[404] license not found")
			require.Nil(credentials, "credentials should be nil")
		})
	})

	s.Run("UpdateLicense", func() {
		original, err := s.client.RetrieveLicense(ctx, licenseID)
		s.Require().NoError(err, "could not retrieve license")

		// Timestamps are truncated to milliseconds, so wait long enough to guarantee
		// that the modified timestamp of the update is after the original.
		time.Sleep(2 * time.Millisecond)

		s.Run("Success", func() {
			require := s.Require()
			license := &api.License{
				ID:          original.ID,
				Name:        "Compliance Engineering",
				Email:       "compliance-eng@norrskenpay.se",
				Description: "Production Envoy node for Norrsken Pay AB.",
				Permissions: []string{"counterparties:admin", "daybreak:sync", "daybreak:download"},
				Expires:     api.Expiration(fields.Time(time.Date(2032, time.June, 1, 0, 0, 0, 0, time.UTC))),
			}

			updated, err := s.client.UpdateLicense(ctx, license)
			require.NoError(err, "could not update license")
			require.Equal(license.Clone(), updated.Clone(), "updated license should be the same as the input without read-only fields")
			require.Equal(original.ID, updated.ID, "id should be the same")
			require.Equal(original.Syncs, updated.Syncs, "syncs should be the same")
			require.Equal(original.Downloads, updated.Downloads, "downloads should be the same")
			require.Equal(original.LastAccess, updated.LastAccess, "last access should be the same")
			require.Equal(original.Created, updated.Created, "created should be the same")
			require.True(updated.Modified.After(original.Modified), "modified should be after the original")

			retrieved, err := s.client.RetrieveLicense(ctx, original.ID)
			require.NoError(err, "could not retrieve license")
			require.Equal(updated, retrieved, "retrieved license should be the updated license")

			// Updating a license must not change its credentials.
			credentials, err := s.client.LicenseCredentials(ctx, original.ID)
			require.NoError(err, "could not retrieve license credentials")
			require.Equal("01JQ8Z5K3N7M2W4X6Y8A0B1C2D", credentials.ClientID, "client ID should be the same")
			require.Equal("MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=", credentials.ClientSecret, "client secret should be the same")
		})

		s.Run("NotFound", func() {
			require := s.Require()
			license := &api.License{
				ID:          ulid.Make(),
				Name:        original.Name,
				Email:       original.Email,
				Description: original.Description,
				Permissions: original.Permissions,
				Expires:     original.Expires,
			}

			updated, err := s.client.UpdateLicense(ctx, license)
			require.Error(err, "expected error updating license not found")
			require.EqualError(err, "[404] license not found")
			require.Nil(updated, "license should be nil")
		})

		s.Run("Invalid", func() {
			require := s.Require()
			expected, err := s.client.RetrieveLicense(ctx, original.ID)
			require.NoError(err, "could not retrieve license")

			updated, err := s.client.UpdateLicense(ctx, &api.License{ID: original.ID})
			require.Error(err, "expected error updating license with empty license")
			require.EqualError(err, "[422] 3 validation errors occurred")
			require.Nil(updated, "license should be nil")

			// The license must not have been changed by the invalid update.
			actual, err := s.client.RetrieveLicense(ctx, original.ID)
			require.NoError(err, "could not retrieve license")
			require.Equal(expected, actual, "license should not have been updated")
		})
	})

	s.Run("DeleteLicense", func() {
		s.Run("Success", func() {
			require := s.Require()
			before, err := s.client.ListLicenses(ctx)
			require.NoError(err, "could not list licenses")

			err = s.client.DeleteLicense(ctx, licenseID)
			require.NoError(err, "could not delete license")

			license, err := s.client.RetrieveLicense(ctx, licenseID)
			require.Error(err, "expected error retrieving license not found")
			require.EqualError(err, "[404] license not found")
			require.Nil(license, "license should be nil")

			credentials, err := s.client.LicenseCredentials(ctx, licenseID)
			require.Error(err, "expected error retrieving credentials for license not found")
			require.EqualError(err, "[404] license not found")
			require.Nil(credentials, "credentials should be nil")

			after, err := s.client.ListLicenses(ctx)
			require.NoError(err, "could not list licenses")
			require.Len(after, len(before)-1, "only the deleted license should have been removed")
		})

		s.Run("NotFound", func() {
			require := s.Require()
			err := s.client.DeleteLicense(ctx, ulid.Make())
			require.Error(err, "expected error deleting license not found")
			require.EqualError(err, "[404] license not found")
		})
	})
}
