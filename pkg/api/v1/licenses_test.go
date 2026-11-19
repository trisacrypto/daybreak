package api_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/trisacrypto/daybreak/pkg/api/v1"
	"github.com/trisacrypto/daybreak/pkg/db/fields"
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
		Expires:     api.Expiration(fields.Time(time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC))),
	}
}

func TestLicenseUnmarshalJSON(t *testing.T) {
	t.Run("Expires", func(t *testing.T) {
		tests := []struct {
			name     string
			expires  string
			expected time.Time
		}{
			{"DateOnly", `"2030-01-02"`, time.Date(2030, time.January, 2, 0, 0, 0, 0, time.UTC)},
			{"DateTimeMinutes", `"2030-01-02 15:16"`, time.Date(2030, time.January, 2, 15, 16, 0, 0, time.UTC)},
			{"DateTimeSeconds", `"2030-01-02 15:16:17"`, time.Date(2030, time.January, 2, 15, 16, 17, 0, time.UTC)},
			{"RFC3339", `"2030-01-02T15:16:17Z"`, time.Date(2030, time.January, 2, 15, 16, 17, 0, time.UTC)},
			{"RFC3339Nano", `"2030-01-02T15:16:17.123456789Z"`, time.Date(2030, time.January, 2, 15, 16, 17, 123456789, time.UTC)},
			{"RFC3339Offset", `"2030-01-02T15:16:17-06:00"`, time.Date(2030, time.January, 2, 21, 16, 17, 0, time.UTC)},
			{"Null", `null`, time.Time{}},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				license := &api.License{}
				data := []byte(`{"name": "Bob's Discount VASP", "expires": ` + tc.expires + `}`)
				require.NoError(t, json.Unmarshal(data, license))
				require.True(t, tc.expected.Equal(license.Expires.Time), "expected %s got %s", tc.expected, license.Expires)
				require.Equal(t, "Bob's Discount VASP", license.Name)
			})
		}
	})

	t.Run("InvalidExpires", func(t *testing.T) {
		tests := []struct {
			name    string
			expires string
		}{
			{"Empty", `""`},
			{"NotADate", `"tomorrow"`},
			{"InvalidMonth", `"2030-13-02"`},
			{"InvalidHour", `"2030-01-02 25:16"`},
			{"USDate", `"01/02/2030"`},
			{"PartialTime", `"2030-01-02 15"`},
			{"TrailingData", `"2030-01-02 15:16:17 CST"`},
			{"Number", `1893597377`},
			{"Bool", `true`},
			{"Object", `{}`},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				license := &api.License{}
				data := []byte(`{"name": "Bob's Discount VASP", "expires": ` + tc.expires + `}`)
				require.Error(t, json.Unmarshal(data, license))
			})
		}
	})

	t.Run("OmittedExpires", func(t *testing.T) {
		// An omitted expires is zero-valued so that it is defaulted by validation.
		license := &api.License{}
		require.NoError(t, json.Unmarshal([]byte(`{"name": "Bob's Discount VASP"}`), license))
		require.True(t, license.Expires.IsZero())

		// As with the standard library, fields not in the JSON are not modified.
		license = mockLicense()
		expires := license.Expires
		require.NoError(t, json.Unmarshal([]byte(`{"name": "Alice's VASP"}`), license))
		require.Equal(t, "Alice's VASP", license.Name)
		require.Equal(t, "bob@bobsvasp.co.uk", license.Email)
		require.Equal(t, expires, license.Expires)
	})

	t.Run("RoundTrip", func(t *testing.T) {
		// Marshaling is unchanged, so all fields must survive a round trip.
		lastAccess := time.Date(2029, time.June, 15, 8, 30, 0, 0, time.UTC)
		license := mockLicense()
		license.ID = ulid.Make()
		license.Syncs = 42
		license.Downloads = 7
		license.LastAccess = &lastAccess
		license.Created = time.Date(2029, time.January, 1, 0, 0, 0, 0, time.UTC)
		license.Modified = time.Date(2029, time.February, 1, 0, 0, 0, 0, time.UTC)

		data, err := json.Marshal(license)
		require.NoError(t, err)

		actual := &api.License{}
		require.NoError(t, json.Unmarshal(data, actual))
		require.Equal(t, license, actual)
	})
}

func TestLicenseValidate(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		tests := []struct {
			name   string
			modify func(l *api.License)
		}{
			{"Complete", func(l *api.License) {}},
			{"NoDescription", func(l *api.License) { l.Description = "" }},
			{"NoExpires", func(l *api.License) { l.Expires = api.ExpirationDate{} }},
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
			Expires:     api.Expiration(fields.Time(time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC))),
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
			license.Expires = api.ExpirationDate{}

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
		expires := time.Date(2030, time.January, 1, 12, 34, 56, 0, time.FixedZone("CST", -6*60*60))
		license = mockLicense()
		license.Expires = api.Expiration(fields.Time(expires))
		require.NoError(t, license.Validate("POST"))
		require.True(t, expires.Equal(license.Expires.Time), "expected expires to match")
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
