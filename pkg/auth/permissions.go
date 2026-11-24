package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	CounterpartiesAdmin  = "counterparties:admin"  // Can edit and delete counterparty records.
	CounterpartiesCreate = "counterparties:create" // Can create new counterparty records.
	DaybreakSync         = "daybreak:sync"         // Can sync counterparty records with Daybreak.
	DaybreakDownload     = "daybreak:download"     // Can download counterparty records from Daybreak.
	LicensesAdmin        = "licenses:admin"        // Can create, edit, and delete license records.
	AllPermissions       = "*/*"                   // Used for logged in users who have no specific permissions.
)

// This are the valid permissions that can be assigned to a license.
var validPermissions = map[string]struct{}{
	CounterpartiesAdmin:  {},
	CounterpartiesCreate: {},
	DaybreakSync:         {},
	DaybreakDownload:     {},
	LicensesAdmin:        {},
}

// IsValidPermission checks if a permission is valid.
func IsValidPermission(permission string) bool {
	_, ok := validPermissions[permission]
	return ok
}

// Returns a middleware that checks if the context has the all of the given permissions.
// If not, the request is aborted with a 403 status code. No redirects occur.
func Authorize(permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// These are the permissions to the user or the license key.
		granted := GetPermissions(c)
		if !HasAdminPermissions(granted) && !HasAllPermissions(granted, permissions...) {
			c.AbortWithStatusJSON(http.StatusForbidden, Unauthorized)
			return
		}
		c.Next()
	}
}

// Some predefined permissions for convenience.
var (
	EnvoyUser           = Authorize(DaybreakSync)
	EnvoyDownload       = Authorize(DaybreakDownload)
	DaybreakContributor = Authorize(CounterpartiesCreate)
	DaybreakEditor      = Authorize(CounterpartiesAdmin)
	DaybreakLicensor    = Authorize(LicensesAdmin)
)

// Returns true if the context has the given permission.
func HasPermission(permissions []string, permission string) bool {
	for _, p := range permissions {
		if p == permission {
			return true
		}
	}
	return false
}

// Returns true if the context has all of the given permissions.
func HasAllPermissions(permissions []string, required ...string) bool {
	for _, permission := range required {
		if !HasPermission(permissions, permission) {
			return false
		}
	}
	return true
}

// Returns true if the granted permissions are wildcard permissions.
func HasAdminPermissions(permissions []string) bool {
	return len(permissions) == 1 && permissions[0] == AllPermissions
}
