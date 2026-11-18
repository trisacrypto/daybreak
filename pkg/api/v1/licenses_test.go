package api_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/trisacrypto/daybreak/pkg/api/v1"
	"go.rtnl.ai/ulid"
	"go.rtnl.ai/x/validation"
)

// mockLicense returns a fully valid license request payload.
// cSpell: ignore bobsvasp
func mockLicense() *api.License {
	return &api.License{
		Name:        "Bob's Discount VASP",
		Email:       "bob@bobsvasp.co.uk",
		Description: "Production Envoy node license",
		Permissions: []string{"daybreak:sync", "daybreak:download"},
		Expires:     time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
}

func TestLicenseValidate(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		tests := []struct {
			name   string
			modify func(l *api.License)
		}{
			{"Complete", func(l *api.License) {}},
			{"NoDescription", func(l *api.License) { l.Description = "" }},
			{"NoExpires", func(l *api.License) { l.Expires = time.Time{} }},
			{"EmailWithDisplayName", func(l *api.License) { l.Email = "Bob Smith <bob@bobsvasp.co.uk>" }},
			{"EmailSubaddress", func(l *api.License) { l.Email = "bob+licenses@bobsvasp.co.uk" }},
			{"ZeroLastAccess", func(l *api.License) { l.LastAccess = &time.Time{} }},
			{"CounterpartiesAdmin", func(l *api.License) { l.Permissions = []string{"counterparties:admin"} }},
			{"CounterpartiesCreate", func(l *api.License) { l.Permissions = []string{"counterparties:create"} }},
			{"DaybreakSync", func(l *api.License) { l.Permissions = []string{"daybreak:sync"} }},
			{"DaybreakDownload", func(l *api.License) { l.Permissions = []string{"daybreak:download"} }},
			{"AllPermissions", func(l *api.License) {
				l.Permissions = []string{"counterparties:admin", "counterparties:create", "daybreak:sync", "daybreak:download"}
			}},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				for _, method := range []string{"POST", "PUT", ""} {
					license := mockLicense()
					tc.modify(license)
					require.NoError(t, license.Validate(method), "expected no error for method %q", method)
				}
			})
		}
	})

	t.Run("Normalization", func(t *testing.T) {
		license := &api.License{
			Name:        "  Bob's Discount VASP\n",
			Email:       "\tbob@bobsvasp.co.uk  ",
			Description: " Production Envoy node license\t",
			Permissions: []string{" Daybreak:Sync", "COUNTERPARTIES:ADMIN\t", "daybreak:download"},
			Expires:     time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC),
		}

		require.NoError(t, license.Validate("POST"))
		require.Equal(t, "Bob's Discount VASP", license.Name)
		require.Equal(t, "bob@bobsvasp.co.uk", license.Email)
		require.Equal(t, "Production Envoy node license", license.Description)
		require.Equal(t, []string{"daybreak:sync", "counterparties:admin", "daybreak:download"}, license.Permissions)

		// Description is optional: whitespace-only description is normalized to the empty string.
		license = mockLicense()
		license.Description = "   "
		require.NoError(t, license.Validate("POST"))
		require.Equal(t, "", license.Description)
	})

	t.Run("DefaultExpires", func(t *testing.T) {
		// A zero expires is normalized to one year from now (UTC), truncated to the hour.
		for _, method := range []string{"POST", "PUT", ""} {
			license := mockLicense()
			license.Expires = time.Time{}

			earliest := time.Now().UTC().AddDate(1, 0, 0).Truncate(time.Hour)
			require.NoError(t, license.Validate(method), "expected no error for method %q", method)
			latest := time.Now().UTC().AddDate(1, 0, 0).Truncate(time.Hour)

			require.False(t, license.Expires.IsZero(), "expected expires to be set for method %q", method)
			require.False(t, license.Expires.Before(earliest), "expires %s is before %s", license.Expires, earliest)
			require.False(t, license.Expires.After(latest), "expires %s is after %s", license.Expires, latest)
			require.Equal(t, time.UTC, license.Expires.Location(), "expected expires to be in UTC")
			require.True(t, license.Expires.Equal(license.Expires.Truncate(time.Hour)), "expected expires to be truncated to the hour")
		}

		// The default is applied even when other fields fail validation.
		license := &api.License{}
		require.Error(t, license.Validate("POST"))
		require.False(t, license.Expires.IsZero(), "expected expires to be set on an invalid license")

		// A user supplied expires is not modified, including its granularity and location.
		expires := time.Date(2030, time.January, 1, 12, 34, 56, 789, time.FixedZone("CST", -6*60*60))
		license = mockLicense()
		license.Expires = expires
		require.NoError(t, license.Validate("POST"))
		require.Equal(t, expires, license.Expires)
	})

	t.Run("Invalid", func(t *testing.T) {
		tests := []struct {
			name   string
			modify func(l *api.License)
			errs   []*validation.FieldError
		}{
			// Read-only fields
			{
				"ReadOnlyID",
				func(l *api.License) { l.ID = ulid.Make() },
				[]*validation.FieldError{validation.ReadOnly("id")},
			},
			{
				"ReadOnlyCreated",
				func(l *api.License) { l.Created = time.Now() },
				[]*validation.FieldError{validation.ReadOnly("created")},
			},
			{
				"ReadOnlyModified",
				func(l *api.License) { l.Modified = time.Now() },
				[]*validation.FieldError{validation.ReadOnly("modified")},
			},
			{
				"ReadOnlySyncs",
				func(l *api.License) { l.Syncs = 42 },
				[]*validation.FieldError{validation.ReadOnly("syncs")},
			},
			{
				"ReadOnlyDownloads",
				func(l *api.License) { l.Downloads = 42 },
				[]*validation.FieldError{validation.ReadOnly("downloads")},
			},
			{
				"ReadOnlyLastAccess",
				func(l *api.License) { now := time.Now(); l.LastAccess = &now },
				[]*validation.FieldError{validation.ReadOnly("last_access")},
			},
			// Name is required
			{
				"MissingName",
				func(l *api.License) { l.Name = "" },
				[]*validation.FieldError{validation.Required("name")},
			},
			{
				"WhitespaceName",
				func(l *api.License) { l.Name = "\t\n " },
				[]*validation.FieldError{validation.Required("name")},
			},
			// Email is required and must parse as an RFC 5322 address
			{
				"MissingEmail",
				func(l *api.License) { l.Email = "" },
				[]*validation.FieldError{validation.Required("email")},
			},
			{
				"WhitespaceEmail",
				func(l *api.License) { l.Email = "   " },
				[]*validation.FieldError{validation.Required("email")},
			},
			{
				"EmailNoAtSign",
				func(l *api.License) { l.Email = "bobsvasp.co.uk" },
				[]*validation.FieldError{validation.Invalid("email", "mail: missing '@' or angle-addr")},
			},
			{
				"EmailNoDomain",
				func(l *api.License) { l.Email = "bob@" },
				[]*validation.FieldError{validation.Invalid("email", "mail: missing '@' or angle-addr")},
			},
			{
				"EmailNoLocalPart",
				func(l *api.License) { l.Email = "@bobsvasp.co.uk" },
				[]*validation.FieldError{validation.Invalid("email", "mail: missing word in phrase: mail: invalid string")},
			},
			{
				"EmailMultipleAddresses",
				func(l *api.License) { l.Email = "bob@bobsvasp.co.uk, alice@bobsvasp.co.uk" },
				[]*validation.FieldError{validation.Invalid("email", "mail: expected single address, got \", alice@bobsvasp.co.uk\"")},
			},
			{
				"EmailInternalWhitespace",
				func(l *api.License) { l.Email = "bob smith@bobsvasp.co.uk" },
				[]*validation.FieldError{validation.Invalid("email", "mail: no angle-addr")},
			},
			// At least one permission is required and all must be known values
			{
				"NilPermissions",
				func(l *api.License) { l.Permissions = nil },
				[]*validation.FieldError{validation.Required("permissions")},
			},
			{
				"EmptyPermissions",
				func(l *api.License) { l.Permissions = []string{} },
				[]*validation.FieldError{validation.Required("permissions")},
			},
			{
				"UnknownPermission",
				func(l *api.License) { l.Permissions = []string{"FOO:Bar"} },
				[]*validation.FieldError{validation.Invalid("permissions", `"foo:bar" is not a valid permission`).Index(0)},
			},
			{
				"BlankPermission",
				func(l *api.License) { l.Permissions = []string{"daybreak:sync", "  "} },
				[]*validation.FieldError{validation.Invalid("permissions", `"" is not a valid permission`).Index(1)},
			},
			{
				"PartialPermission",
				func(l *api.License) { l.Permissions = []string{"daybreak"} },
				[]*validation.FieldError{validation.Invalid("permissions", `"daybreak" is not a valid permission`).Index(0)},
			},
			{
				"MultipleUnknownPermissions",
				func(l *api.License) { l.Permissions = []string{"daybreak:sync", "foo", "daybreak:download", "bar"} },
				[]*validation.FieldError{
					validation.Invalid("permissions", `"foo" is not a valid permission`).Index(1),
					validation.Invalid("permissions", `"bar" is not a valid permission`).Index(3),
				},
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				for _, method := range []string{"POST", "PUT", ""} {
					license := mockLicense()
					tc.modify(license)

					err := license.Validate(method)
					require.Error(t, err, "expected a validation error for method %q", method)

					var verr validation.Errors
					require.True(t, errors.As(err, &verr), "expected validation.Errors")
					require.Len(t, verr, len(tc.errs), "unexpected number of validation errors: %s", err)
					for i, expected := range tc.errs {
						require.True(t, expected.Equal(verr[i]), "error %d: expected %q got %q", i, expected.Error(), verr[i].Error())
					}
				}
			})
		}
	})

	t.Run("MultipleErrors", func(t *testing.T) {
		// A zero-valued license reports every required field in declaration order
		// (description is optional and a zero expires is defaulted, so neither appears).
		license := &api.License{}
		err := license.Validate("POST")
		require.Error(t, err)

		var verr validation.Errors
		require.True(t, errors.As(err, &verr), "expected validation.Errors")

		expected := validation.Errors{
			validation.Required("name"),
			validation.Required("email"),
			validation.Required("permissions"),
		}
		require.Len(t, verr, len(expected), "unexpected number of validation errors: %s", err)
		for i := range expected {
			require.True(t, expected[i].Equal(verr[i]), "error %d: expected %q got %q", i, expected[i].Error(), verr[i].Error())
		}

		// Every check is independent: all errors are collected rather than short-circuiting.
		now := time.Now()
		license = &api.License{
			ID:          ulid.Make(),
			Email:       "not-an-email",
			Permissions: []string{"foo"},
			Syncs:       1,
			Downloads:   1,
			LastAccess:  &now,
			Created:     now,
			Modified:    now,
		}
		err = license.Validate("PUT")
		require.Error(t, err)
		require.True(t, errors.As(err, &verr), "expected validation.Errors")

		expected = validation.Errors{
			validation.ReadOnly("id"),
			validation.ReadOnly("created"),
			validation.ReadOnly("modified"),
			validation.ReadOnly("syncs"),
			validation.ReadOnly("downloads"),
			validation.ReadOnly("last_access"),
			validation.Required("name"),
			validation.Invalid("email", "mail: missing '@' or angle-addr"),
			validation.Invalid("permissions", `"foo" is not a valid permission`).Index(0),
		}
		require.Len(t, verr, len(expected), "unexpected number of validation errors: %s", err)
		for i := range expected {
			require.True(t, expected[i].Equal(verr[i]), "error %d: expected %q got %q", i, expected[i].Error(), verr[i].Error())
		}
	})
}
