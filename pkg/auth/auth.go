package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/trisacrypto/daybreak/pkg/db"
	"github.com/trisacrypto/daybreak/pkg/db/fields"
	"github.com/trisacrypto/daybreak/pkg/errors"
	"go.rtnl.ai/gimlet/logger"
	"go.rtnl.ai/ulid"
	"go.rtnl.ai/x/api"
	"go.rtnl.ai/x/hawk"
)

const (
	UserInfoKey    = "userinfo"
	PermissionsKey = "permissions"
	UserSessionKey = "user"
	LicenseIDKey   = "license"

	OAuthStateKey = "oauth_state"
	OAuthStateTTL = 20 * time.Minute

	HawkWindow = 90 * time.Second

	Authorization       = "Authorization"
	XRequestID          = "X-Request-ID"
	XResponseID         = "X-Response-ID"
	XTimestamp          = "X-Timestamp"
	ServerAuthorization = "Server-Authorization"

	// The content type of the archive that is requested from the download endpoint.
	MIMEZIP = "application/zip"
)

var (
	Unauthorized  = api.Reply{Success: false, Error: "you do not have permission to access this resource"}
	InternalError = api.Reply{Success: false, Error: "an internal error occurred"}
)

var (
	requiredHeaders = []string{
		Authorization,
		XRequestID,
		XTimestamp,
	}

	// The headers that the Hawk token must sign to prevent replay attacks.
	signedHeaders = []string{
		XRequestID,
		XTimestamp,
	}
)

// This middleware is used to authenticate either a user or a license key and is
// primarily used for the API endpoints that also power the HTMX partials. Requests
// for HTML are authenticated by the session and requests for JSON or a zip archive
// (e.g. the download endpoint) are authenticated by a license key.
func Authenticate(db TokenIdentifier) gin.HandlerFunc {
	licenseRequired := LicenseRequired(db)
	return func(c *gin.Context) {
		switch format := c.NegotiateFormat(binding.MIMEHTML, binding.MIMEJSON, MIMEZIP); format {
		case binding.MIMEHTML:
			LoginRequired(c)
		case binding.MIMEJSON, MIMEZIP:
			licenseRequired(c)
		default:
			c.AbortWithStatus(http.StatusNotAcceptable)
		}
	}
}

// This middleware is used to authenticate a license key and checks the Authorization
// header for a HAWK signed request. If the request is authenticated or the license
// key is expired the request is aborted with unauthorized or forbidden status codes.
func LicenseRequired(store TokenIdentifier) gin.HandlerFunc {
	requestCache := &RequestCache{}
	return func(c *gin.Context) {
		var (
			ok           bool
			err          error
			token        *hawk.Token
			clientID     ulid.ULID
			license      *db.License
			requestID    string
			responseID   string
			responseAuth string
		)

		// Ensure the request has the required headers.
		for _, header := range requiredHeaders {
			if c.Request.Header.Get(header) == "" {
				c.AbortWithStatusJSON(http.StatusBadRequest, api.Reply{Success: false, Error: fmt.Sprintf("missing required header: %s", header)})
				return
			}
		}

		// Ensure the timestamp of the request is the allowed duration.
		if ok, err := ValidateTimestamp(c.Request.Header.Get(XTimestamp)); err != nil || !ok {
			if err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest, api.Reply{Success: false, Error: "invalid request timestamp"})
			} else {
				c.AbortWithStatusJSON(http.StatusUnauthorized, Unauthorized)
			}
			return
		}

		// Parse the Hawk token from the Authorization header.
		if token, err = hawk.Parse(c.Request.Header.Get(Authorization)); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, Unauthorized)
			return
		}

		// Ensure the token signs the headers that are used to prevent replay attacks.
		// The client chooses which headers are signed, so a header that is not signed
		// could be changed to replay the request without invalidating the signature.
		for _, header := range signedHeaders {
			if !slices.ContainsFunc(token.Headers(), func(signed string) bool { return strings.EqualFold(signed, header) }) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, Unauthorized)
				return
			}
		}

		// Ensure the client ID is a valid ULID.
		if clientID, err = ulid.Parse(token.KeyID()); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, Unauthorized)
			return
		}

		// Retrieve the license from the database.
		ctx, cancel := context.WithTimeout(c.Request.Context(), 4*time.Second)
		defer cancel()

		if license, err = store.RetrieveLicense(ctx, clientID); err != nil {
			if errors.Is(err, errors.ErrNotFound) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, Unauthorized)
				return
			}
			c.Error(fmt.Errorf("failed to retrieve license: %w", err))
			c.AbortWithStatusJSON(http.StatusInternalServerError, InternalError)
			return
		}

		// Collect the headers and verify the signature with the license secret.
		token.Collect(c.Request.Header)
		if ok, err := token.Verify(license.Secret); err != nil || !ok {
			if err != nil {
				c.Error(fmt.Errorf("failed to verify hawk token: %w", err))
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, Unauthorized)
			return
		}

		// Check license expired after verifying the signature so that we don't leak
		// information about the license to someone who doesn't have the secret.
		if license.IsExpired() {
			c.AbortWithStatusJSON(http.StatusForbidden, Unauthorized)
			return
		}

		// Ensure the request ID is not in the cache to prevent reply attacks
		// This check happens after the signature is verified to prevent a timing
		// attack where the attacker floods the cache with request IDs to cause the
		// cache to reset and then replies the request with a valid request ID.
		requestID = c.Request.Header.Get(XRequestID)
		if ok, err := requestCache.Check(requestID); err != nil || !ok {
			if err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest, api.Reply{Success: false, Error: "invalid request ID"})
			} else {
				c.AbortWithStatusJSON(http.StatusUnauthorized, Unauthorized)
			}
			return
		}

		// Add server-side authentication information to the response.
		// cSpell:ignore xtimestamp
		xtimestamp := time.Now().Format(time.RFC3339)
		if responseID, ok = logger.RequestID(c); !ok {
			responseID = ulid.Make().String()
		}

		c.Header(XResponseID, responseID)
		c.Header(XTimestamp, xtimestamp)
		c.Header(XRequestID, requestID)

		mac := hawk.New(license.ID.String(), license.Secret)
		mac.Append(XResponseID, responseID)
		mac.Append(XTimestamp, xtimestamp)
		mac.Append(XRequestID, requestID)

		if responseAuth, err = mac.Authorization(); err != nil {
			c.Error(fmt.Errorf("failed to generate hawk authorization header: %w", err))
			c.AbortWithStatusJSON(http.StatusInternalServerError, InternalError)
			return
		}

		c.Header(ServerAuthorization, responseAuth)

		// Record that the license was successfully accessed.
		if err = store.LicenseAccessed(ctx, license.ID); err != nil {
			c.Error(fmt.Errorf("failed to record license access: %w", err))
			c.AbortWithStatusJSON(http.StatusInternalServerError, InternalError)
			return
		}

		// The request is authenticated and the license is valid; so set the permissions
		// NOTE: permissions must be a []string or GetPermissions will panic.
		c.Set(LicenseIDKey, license.ID)
		c.Set(PermissionsKey, []string(license.Permissions))
		c.Next()
	}
}

// LoginRequired is a session based middleware that checks for a valid cookie and is
// only usable by the UI since cookies generally require a browser. A request is
// determined to be a UI request if the request accepts HTML or is an htmx request.
func LoginRequired(c *gin.Context) {
	// Load the user from the session (via the cookie)
	session := sessions.Default(c)
	user := session.Get(UserSessionKey)

	// If there is no user then no one is logged in; so redirect to the login page.
	// NOTE: abort MUST be called here to prevent any writes to the response body.
	if user == nil {
		c.Redirect(http.StatusTemporaryRedirect, "/login")
		c.Abort()
		return
	}

	// Add the user and the default user permissions to the context
	c.Set(UserInfoKey, user)
	c.Set(PermissionsKey, []string{AllPermissions})
	c.Next()
}

type TokenIdentifier interface {
	RetrieveLicense(ctx context.Context, id ulid.ULID) (license *db.License, err error)
	LicenseAccessed(ctx context.Context, id ulid.ULID) (err error)
}

// Returns the user that LoginRequired set on the context or nil if there is no user,
// e.g. because the request was authenticated with a license key.
func GetUserInfo(c *gin.Context) *UserInfo {
	val, ok := c.Get(UserInfoKey)
	if !ok {
		return nil
	}

	// The user is registered with gob as a value, so a value is what is decoded from
	// the session even when a pointer was saved to it. The pointer is only returned
	// by the session during the request that it was saved in.
	switch user := val.(type) {
	case UserInfo:
		return &user
	case *UserInfo:
		return user
	default:
		panic(fmt.Errorf("unexpected user info type on context: %T", val))
	}
}

func GetPermissions(c *gin.Context) []string {
	val, ok := c.Get(PermissionsKey)
	if !ok {
		return nil
	}

	switch permissions := val.(type) {
	case []string:
		return permissions
	case fields.StringArray:
		return []string(permissions)
	default:
		panic(fmt.Errorf("unexpected permissions type on context: %T", val))
	}
}

func GetLicenseID(c *gin.Context) ulid.ULID {
	val, ok := c.Get(LicenseIDKey)
	if !ok {
		return ulid.ULID{}
	}
	return val.(ulid.ULID)
}

func StateCookie(c *gin.Context) string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Errorf("failed to generate random bytes: %w", err))
	}

	state := base64.URLEncoding.EncodeToString(buf)
	c.SetCookie(OAuthStateKey, state, int(OAuthStateTTL.Seconds()), "/", "", true, true)
	return state
}

func ValidateTimestamp(timestamp string) (ok bool, err error) {
	var ts time.Time
	if ts, err = time.Parse(time.RFC3339, timestamp); err != nil {
		return false, err
	}

	// Get the absolute value of the elapsed time to ensure it's positive.
	// The Hawk protocol requires the timestamp to be within the allowed window and
	// takes into account possible clock skew or drift that might cause the timestamp
	// to be slightly in the future.
	elapsed := time.Since(ts).Abs()
	if elapsed > HawkWindow {
		return false, nil
	}
	return true, nil

}
