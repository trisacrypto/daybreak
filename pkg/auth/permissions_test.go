package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/trisacrypto/daybreak/pkg/auth"
	"go.rtnl.ai/x/api"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

// Every permission that can be assigned to a license.
var licensePermissions = []string{
	auth.CounterpartiesAdmin,
	auth.CounterpartiesCreate,
	auth.DaybreakSync,
	auth.DaybreakDownload,
	auth.LicensesAdmin,
}

func TestIsValidPermission(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		for _, permission := range licensePermissions {
			require.True(t, auth.IsValidPermission(permission), "expected %q to be valid", permission)
		}

		// The constants are part of the API so their values must not drift.
		for _, permission := range []string{
			"counterparties:admin", "counterparties:create",
			"daybreak:sync", "daybreak:download", "licenses:admin",
		} {
			require.True(t, auth.IsValidPermission(permission), "expected %q to be valid", permission)
		}
	})

	t.Run("Invalid", func(t *testing.T) {
		tests := []struct {
			name       string
			permission string
		}{
			// The wildcard is reserved for logged in users, it cannot be assigned to a license.
			{"Wildcard", auth.AllPermissions},
			{"Empty", ""},
			{"Whitespace", "  "},
			{"Unknown", "foo:bar"},
			{"UnknownAction", "licenses:create"},
			{"UnknownResource", "users:admin"},
			{"ResourceOnly", "licenses"},
			{"ActionOnly", "admin"},
			{"ResourceWildcard", "licenses:*"},
			{"UpperCase", "LICENSES:ADMIN"},
			{"MixedCase", "Licenses:Admin"},
			{"LeadingSpace", " licenses:admin"},
			{"TrailingSpace", "licenses:admin "},
			{"Combined", "daybreak:sync,daybreak:download"},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				require.False(t, auth.IsValidPermission(tc.permission), "expected %q to be invalid", tc.permission)
			})
		}
	})
}

func TestHasPermission(t *testing.T) {
	tests := []struct {
		name        string
		permissions []string
		permission  string
		expected    bool
	}{
		{"Only", []string{auth.LicensesAdmin}, auth.LicensesAdmin, true},
		{"First", []string{auth.DaybreakSync, auth.DaybreakDownload, auth.LicensesAdmin}, auth.DaybreakSync, true},
		{"Middle", []string{auth.DaybreakSync, auth.DaybreakDownload, auth.LicensesAdmin}, auth.DaybreakDownload, true},
		{"Last", []string{auth.DaybreakSync, auth.DaybreakDownload, auth.LicensesAdmin}, auth.LicensesAdmin, true},
		{"Duplicates", []string{auth.DaybreakSync, auth.DaybreakSync}, auth.DaybreakSync, true},
		{"WildcardLiteral", []string{auth.AllPermissions}, auth.AllPermissions, true},
		{"Nil", nil, auth.LicensesAdmin, false},
		{"Empty", []string{}, auth.LicensesAdmin, false},
		{"Missing", []string{auth.DaybreakSync, auth.DaybreakDownload}, auth.LicensesAdmin, false},
		{"SameResource", []string{auth.CounterpartiesCreate}, auth.CounterpartiesAdmin, false},
		{"SameAction", []string{auth.CounterpartiesAdmin}, auth.LicensesAdmin, false},
		{"CaseSensitive", []string{"LICENSES:ADMIN"}, auth.LicensesAdmin, false},
		{"NoTrim", []string{" licenses:admin "}, auth.LicensesAdmin, false},
		{"EmptyPermission", []string{auth.LicensesAdmin}, "", false},
		// The wildcard is not expanded by HasPermission, see HasAdminPermissions.
		{"WildcardNotExpanded", []string{auth.AllPermissions}, auth.LicensesAdmin, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, auth.HasPermission(tc.permissions, tc.permission))
		})
	}
}

func TestHasAllPermissions(t *testing.T) {
	tests := []struct {
		name        string
		permissions []string
		required    []string
		expected    bool
	}{
		{"Single", []string{auth.LicensesAdmin}, []string{auth.LicensesAdmin}, true},
		{"Exact", []string{auth.DaybreakSync, auth.DaybreakDownload}, []string{auth.DaybreakSync, auth.DaybreakDownload}, true},
		{"OrderIndependent", []string{auth.DaybreakSync, auth.DaybreakDownload}, []string{auth.DaybreakDownload, auth.DaybreakSync}, true},
		{"Superset", licensePermissions, []string{auth.CounterpartiesAdmin, auth.LicensesAdmin}, true},
		{"Everything", licensePermissions, licensePermissions, true},
		{"DuplicateRequired", []string{auth.DaybreakSync}, []string{auth.DaybreakSync, auth.DaybreakSync}, true},
		// Nothing required is vacuously true, even when nothing is granted.
		{"NoneRequired", []string{auth.LicensesAdmin}, nil, true},
		{"NoneRequiredNoneGranted", nil, nil, true},
		{"NoneRequiredEmptyGranted", []string{}, []string{}, true},
		{"NilGranted", nil, []string{auth.LicensesAdmin}, false},
		{"EmptyGranted", []string{}, []string{auth.LicensesAdmin}, false},
		{"MissingOnly", []string{auth.DaybreakSync}, []string{auth.LicensesAdmin}, false},
		{"MissingFirst", []string{auth.DaybreakDownload}, []string{auth.DaybreakSync, auth.DaybreakDownload}, false},
		{"MissingLast", []string{auth.DaybreakSync}, []string{auth.DaybreakSync, auth.DaybreakDownload}, false},
		{"MissingAll", []string{auth.LicensesAdmin}, []string{auth.DaybreakSync, auth.DaybreakDownload}, false},
		// The wildcard is not expanded by HasAllPermissions, see HasAdminPermissions.
		{"WildcardNotExpanded", []string{auth.AllPermissions}, []string{auth.LicensesAdmin}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, auth.HasAllPermissions(tc.permissions, tc.required...))
		})
	}
}

func TestHasAdminPermissions(t *testing.T) {
	tests := []struct {
		name        string
		permissions []string
		expected    bool
	}{
		{"Wildcard", []string{auth.AllPermissions}, true},
		{"WildcardLiteral", []string{"*/*"}, true},
		{"Nil", nil, false},
		{"Empty", []string{}, false},
		{"SinglePermission", []string{auth.LicensesAdmin}, false},
		{"EveryLicensePermission", licensePermissions, false},
		// The wildcard must be the only permission granted to be considered admin.
		{"WildcardFirst", []string{auth.AllPermissions, auth.LicensesAdmin}, false},
		{"WildcardLast", []string{auth.LicensesAdmin, auth.AllPermissions}, false},
		{"WildcardRepeated", []string{auth.AllPermissions, auth.AllPermissions}, false},
		{"Star", []string{"*"}, false},
		{"ColonWildcard", []string{"*:*"}, false},
		{"ResourceWildcard", []string{"licenses:*"}, false},
		{"PaddedWildcard", []string{" */* "}, false},
		{"EmptyString", []string{""}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, auth.HasAdminPermissions(tc.permissions))
		})
	}
}

// Sets the granted permissions on the context the same way authentication middleware does.
func grant(permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(auth.PermissionsKey, permissions)
		c.Next()
	}
}

// Executes a request against a router with the Authorize middleware guarding the
// handler. If authenticate is nil then no permissions are set on the context. Returns
// the response and if the handler guarded by the middleware was called.
func authorize(t *testing.T, authenticate gin.HandlerFunc, required ...string) (*httptest.ResponseRecorder, bool) {
	t.Helper()

	called := false
	router := gin.New()
	if authenticate != nil {
		router.Use(authenticate)
	}

	router.GET("/", auth.Authorize(required...), func(c *gin.Context) {
		called = true
		c.JSON(http.StatusOK, api.Reply{Success: true})
	})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	return w, called
}

func TestAuthorize(t *testing.T) {
	t.Run("Allowed", func(t *testing.T) {
		tests := []struct {
			name         string
			authenticate gin.HandlerFunc
			required     []string
		}{
			{"Exact", grant(auth.LicensesAdmin), []string{auth.LicensesAdmin}},
			{"ExactMultiple", grant(auth.DaybreakSync, auth.DaybreakDownload), []string{auth.DaybreakSync, auth.DaybreakDownload}},
			{"OrderIndependent", grant(auth.DaybreakSync, auth.DaybreakDownload), []string{auth.DaybreakDownload, auth.DaybreakSync}},
			{"Superset", grant(licensePermissions...), []string{auth.CounterpartiesCreate}},
			{"Everything", grant(licensePermissions...), licensePermissions},
			// Logged in users have the wildcard permission and can access everything.
			{"WildcardOne", grant(auth.AllPermissions), []string{auth.LicensesAdmin}},
			{"WildcardMultiple", grant(auth.AllPermissions), []string{auth.CounterpartiesAdmin, auth.CounterpartiesCreate}},
			{"WildcardEverything", grant(auth.AllPermissions), licensePermissions},
			{"WildcardNoneRequired", grant(auth.AllPermissions), nil},
			// If no permissions are required then the middleware does not restrict access.
			{"NoneRequired", grant(auth.DaybreakSync), nil},
			{"NoneRequiredNoneGranted", grant(), nil},
			{"NoneRequiredNotAuthenticated", nil, nil},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				w, called := authorize(t, tc.authenticate, tc.required...)
				require.Equal(t, http.StatusOK, w.Code)
				require.True(t, called, "expected the handler to be called")

				reply := api.Reply{}
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &reply))
				require.True(t, reply.Success)
			})
		}
	})

	t.Run("Forbidden", func(t *testing.T) {
		tests := []struct {
			name         string
			authenticate gin.HandlerFunc
			required     []string
		}{
			{"NotAuthenticated", nil, []string{auth.LicensesAdmin}},
			{"NoneGranted", grant(), []string{auth.LicensesAdmin}},
			{"EmptyGranted", grant([]string{}...), []string{auth.LicensesAdmin}},
			{"Missing", grant(auth.DaybreakSync, auth.DaybreakDownload), []string{auth.LicensesAdmin}},
			{"MissingFirst", grant(auth.DaybreakDownload), []string{auth.DaybreakSync, auth.DaybreakDownload}},
			{"MissingLast", grant(auth.DaybreakSync), []string{auth.DaybreakSync, auth.DaybreakDownload}},
			{"MissingOneOfEverything", grant(licensePermissions[1:]...), licensePermissions},
			// Permissions are not hierarchical: admin does not imply create and vice versa.
			{"AdminDoesNotImplyCreate", grant(auth.CounterpartiesAdmin), []string{auth.CounterpartiesCreate}},
			{"CreateDoesNotImplyAdmin", grant(auth.CounterpartiesCreate), []string{auth.CounterpartiesAdmin}},
			{"OtherResourceAdmin", grant(auth.CounterpartiesAdmin), []string{auth.LicensesAdmin}},
			// Permissions are matched exactly, they must be normalized before being granted.
			{"CaseSensitive", grant("LICENSES:ADMIN"), []string{auth.LicensesAdmin}},
			{"NoTrim", grant(" licenses:admin "), []string{auth.LicensesAdmin}},
			// The wildcard only grants access when it is the only permission granted.
			{"WildcardWithOthers", grant(auth.AllPermissions, auth.DaybreakSync), []string{auth.LicensesAdmin}},
			{"WildcardRepeated", grant(auth.AllPermissions, auth.AllPermissions), []string{auth.LicensesAdmin}},
			{"ResourceWildcard", grant("licenses:*"), []string{auth.LicensesAdmin}},
			{"ColonWildcard", grant("*:*"), []string{auth.LicensesAdmin}},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				w, called := authorize(t, tc.authenticate, tc.required...)
				require.Equal(t, http.StatusForbidden, w.Code)
				require.False(t, called, "expected the request to be aborted before the handler")

				// The middleware must respond with JSON and never redirect.
				require.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
				require.Empty(t, w.Header().Get("Location"))

				reply := api.Reply{}
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &reply))
				require.Equal(t, auth.Unauthorized, reply)
				require.False(t, reply.Success)
				require.Equal(t, "you do not have permission to access this resource", reply.Error)
			})
		}
	})

	t.Run("Chain", func(t *testing.T) {
		// When multiple Authorize middleware are chained, all of them must pass.
		chain := func(authenticate gin.HandlerFunc) (*httptest.ResponseRecorder, bool) {
			called := false
			router := gin.New()
			router.Use(authenticate, auth.Authorize(auth.DaybreakSync))
			router.GET("/", auth.Authorize(auth.DaybreakDownload), func(c *gin.Context) {
				called = true
				c.JSON(http.StatusOK, api.Reply{Success: true})
			})

			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
			return w, called
		}

		w, called := chain(grant(auth.DaybreakSync, auth.DaybreakDownload))
		require.Equal(t, http.StatusOK, w.Code)
		require.True(t, called, "expected the handler to be called")

		w, called = chain(grant(auth.AllPermissions))
		require.Equal(t, http.StatusOK, w.Code)
		require.True(t, called, "expected the handler to be called")

		w, called = chain(grant(auth.DaybreakSync))
		require.Equal(t, http.StatusForbidden, w.Code)
		require.False(t, called, "expected the request to be aborted by the second middleware")

		w, called = chain(grant(auth.DaybreakDownload))
		require.Equal(t, http.StatusForbidden, w.Code)
		require.False(t, called, "expected the request to be aborted by the first middleware")
	})
}
