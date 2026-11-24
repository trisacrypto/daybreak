package auth_test

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/stretchr/testify/require"
	"github.com/trisacrypto/daybreak/pkg/db"
	"github.com/trisacrypto/daybreak/pkg/db/fields"
	"github.com/trisacrypto/daybreak/pkg/errors"
	"go.rtnl.ai/ulid"
	"go.rtnl.ai/x/api"
	"go.rtnl.ai/x/hawk"

	. "github.com/trisacrypto/daybreak/pkg/auth"
)

// The name of the session cookie used by the test server.
const sessionName = "daybreak"

// cSpell: disable
// The client IDs of the test licenses, named for what the license is used to test.
var (
	adminID         = ulid.MustParse("01M7WM5CZY2J9HQDR8G0E8TR3S")
	envoyID         = ulid.MustParse("01M7WNAFV81YGFDNHD26Z8GTP7")
	noPermissionsID = ulid.MustParse("01M7WNAP55D9ZHX7027EG5KXRA")
	expiredID       = ulid.MustParse("01M7WNAY3WBDM45DC5JT45XP0M")
	wildcardID      = ulid.MustParse("01M7WNB5EW957PRW7BSFS0DBYY")
)

// cSpell: enable

var testLicenses = map[ulid.ULID]*db.License{
	adminID: {
		ID:          adminID,
		Secret:      makeSecret(),
		Name:        "Admin",
		Permissions: []string{CounterpartiesAdmin, CounterpartiesCreate, DaybreakSync, DaybreakDownload, LicensesAdmin},
		Expires:     fields.Time(time.Now().AddDate(1, 0, 0)),
	},
	envoyID: {
		ID:          envoyID,
		Secret:      makeSecret(),
		Name:        "Envoy",
		Permissions: []string{DaybreakSync, DaybreakDownload},
		Expires:     fields.Time(time.Now().AddDate(1, 0, 0)),
	},
	noPermissionsID: {
		ID:          noPermissionsID,
		Secret:      makeSecret(),
		Name:        "No Permissions",
		Permissions: []string{},
		Expires:     fields.Time(time.Now().AddDate(1, 0, 0)),
	},
	expiredID: {
		ID:          expiredID,
		Secret:      makeSecret(),
		Name:        "Expired",
		Permissions: []string{DaybreakSync, DaybreakDownload},
		Expires:     fields.Time(time.Now().AddDate(0, 0, -9)),
	},
	wildcardID: {
		ID:          wildcardID,
		Secret:      makeSecret(),
		Name:        "Wildcard",
		Permissions: []string{AllPermissions},
		Expires:     fields.Time(time.Now().AddDate(1, 0, 0)),
	},
}

func TestAuthMiddleware(t *testing.T) {
	// Create a test server
	store := &TestTokenIdentifier{licenses: testLicenses, accesses: make(map[ulid.ULID]int)}
	router := gin.New()
	router.Use(sessions.Sessions(sessionName, cookie.NewStore(makeSecret())))

	authenticate := Authenticate(store)

	router.POST("/login", Login)
	router.GET("/counterparties", authenticate, Ok)
	router.POST("/counterparties", authenticate, Authorize(CounterpartiesCreate), Ok)
	router.GET("/daybreak", authenticate, Authorize(DaybreakSync, DaybreakDownload), Ok)
	router.GET("/whoami", authenticate, WhoAmI)

	srv := httptest.NewTLSServer(router)
	t.Cleanup(srv.Close)

	// Redirects are not followed so that the redirect to the login page can be tested.
	client := srv.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	// Creates a JSON API request that is signed by the license with the client ID.
	signed := func(t *testing.T, method, path string, clientID ulid.ULID) *http.Request {
		t.Helper()
		req, err := newTestRequest(method, srv.URL+path, clientID, nil)
		require.NoError(t, err)
		req.Header.Set("Accept", binding.MIMEJSON)
		return req
	}

	// Signs the request with the license again after its headers have been modified.
	resign := func(t *testing.T, req *http.Request, clientID ulid.ULID) {
		t.Helper()
		require.NoError(t, signTestRequest(req, clientID.String(), testLicenses[clientID].Secret))
	}

	do := func(t *testing.T, req *http.Request) *http.Response {
		t.Helper()
		return execute(t, client, req)
	}

	t.Run("LicenseRequired", func(t *testing.T) {
		// JSON requests are authenticated by the license that signed the request, and
		// the server signs its reply with the same license.
		for _, clientID := range []ulid.ULID{adminID, envoyID, noPermissionsID, wildcardID} {
			license := testLicenses[clientID]
			t.Run(license.Name, func(t *testing.T) {
				req := signed(t, http.MethodGet, "/counterparties", clientID)
				rep := do(t, req)
				require.Equal(t, http.StatusOK, rep.StatusCode)
				requireServerAuth(t, req, rep, license)
			})
		}

		// The request ID can be a ULID instead of a UUID.
		t.Run("ULIDRequestID", func(t *testing.T) {
			req := signed(t, http.MethodGet, "/counterparties", envoyID)
			req.Header.Set(XRequestID, ulid.Make().String())
			resign(t, req, envoyID)

			rep := do(t, req)
			require.Equal(t, http.StatusOK, rep.StatusCode)
			requireServerAuth(t, req, rep, testLicenses[envoyID])
		})

		// Handlers are given the permissions of the license; there is no user.
		t.Run("Identity", func(t *testing.T) {
			for _, clientID := range []ulid.ULID{adminID, envoyID, noPermissionsID, wildcardID} {
				license := testLicenses[clientID]
				rep := do(t, signed(t, http.MethodGet, "/whoami", clientID))
				require.Equal(t, http.StatusOK, rep.StatusCode)

				identity := &Identity{}
				require.NoError(t, json.NewDecoder(rep.Body).Decode(identity))
				require.Nil(t, identity.User, "expected no user for the %s license", license.Name)
				require.Equal(t, []string(license.Permissions), identity.Permissions, "unexpected permissions for the %s license", license.Name)
			}
		})
	})

	t.Run("LicenseAccessed", func(t *testing.T) {
		// Asserts that exactly one access was recorded for each of the licenses since
		// the accesses were counted, and that none were recorded for any other license.
		requireAccessed := func(t *testing.T, before map[string]int, accessed ...ulid.ULID) {
			t.Helper()
			expected := maps.Clone(before)
			for _, clientID := range accessed {
				expected[clientID.String()]++
			}
			require.Equal(t, expected, store.Accesses(), "unexpected license accesses recorded")
		}

		t.Run("Authenticated", func(t *testing.T) {
			// The access is recorded for the license that signed the request, and only
			// for that license, every time that a request is authenticated.
			for _, clientID := range []ulid.ULID{adminID, envoyID, noPermissionsID, wildcardID} {
				t.Run(testLicenses[clientID].Name, func(t *testing.T) {
					for range 2 {
						before := store.Accesses()
						rep := do(t, signed(t, http.MethodGet, "/counterparties", clientID))
						require.Equal(t, http.StatusOK, rep.StatusCode)
						requireAccessed(t, before, clientID)
					}
				})
			}
		})

		t.Run("NotPermitted", func(t *testing.T) {
			// The license is authenticated before its permissions are checked, so the
			// access is recorded even if the license is not permitted to use the route.
			before := store.Accesses()
			rep := do(t, signed(t, http.MethodPost, "/counterparties", envoyID))
			require.Equal(t, http.StatusForbidden, rep.StatusCode)
			requireAccessed(t, before, envoyID)
		})

		t.Run("NotAuthenticated", func(t *testing.T) {
			// No access is recorded for a request that the middleware rejects, whether
			// or not the license that it claims to be from exists.
			tests := []struct {
				name   string
				status int
				modify func(t *testing.T, req *http.Request)
			}{
				{"NotAcceptable", http.StatusNotAcceptable, func(t *testing.T, req *http.Request) {
					req.Header.Set("Accept", "text/plain")
				}},
				{"MissingHeader", http.StatusBadRequest, func(t *testing.T, req *http.Request) {
					req.Header.Del(XTimestamp)
				}},
				{"OutsideWindow", http.StatusUnauthorized, func(t *testing.T, req *http.Request) {
					req.Header.Set(XTimestamp, time.Now().Add(-HawkWindow-time.Minute).Format(time.RFC3339))
					resign(t, req, envoyID)
				}},
				{"MalformedToken", http.StatusUnauthorized, func(t *testing.T, req *http.Request) {
					req.Header.Set(Authorization, "Bearer testing")
				}},
				{"UnsignedHeaders", http.StatusUnauthorized, func(t *testing.T, req *http.Request) {
					require.NoError(t, signTestRequest(req, envoyID.String(), testLicenses[envoyID].Secret, XTimestamp))
				}},
				{"UnknownLicense", http.StatusUnauthorized, func(t *testing.T, req *http.Request) {
					require.NoError(t, signTestRequest(req, ulid.Make().String(), testLicenses[envoyID].Secret))
				}},
				{"WrongSecret", http.StatusUnauthorized, func(t *testing.T, req *http.Request) {
					require.NoError(t, signTestRequest(req, envoyID.String(), makeSecret()))
				}},
				{"ExpiredLicense", http.StatusForbidden, func(t *testing.T, req *http.Request) {
					require.NoError(t, signTestRequest(req, expiredID.String(), testLicenses[expiredID].Secret))
				}},
				{"InvalidRequestID", http.StatusBadRequest, func(t *testing.T, req *http.Request) {
					req.Header.Set(XRequestID, "not-a-request-id")
					resign(t, req, envoyID)
				}},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					before := store.Accesses()
					req := signed(t, http.MethodGet, "/counterparties", envoyID)
					tc.modify(t, req)

					require.Equal(t, tc.status, do(t, req).StatusCode)
					requireAccessed(t, before)
				})
			}
		})

		t.Run("Replayed", func(t *testing.T) {
			// Only the original request is recorded as an access, not its replays.
			before := store.Accesses()
			req := signed(t, http.MethodGet, "/counterparties", envoyID)
			require.Equal(t, http.StatusOK, do(t, req).StatusCode)

			for range 2 {
				require.Equal(t, http.StatusUnauthorized, do(t, req.Clone(req.Context())).StatusCode)
			}
			requireAccessed(t, before, envoyID)
		})

		t.Run("NotRecorded", func(t *testing.T) {
			// The request does not reach the handler if the access cannot be recorded.
			store.FailAccesses(fmt.Errorf("could not record license access"))
			t.Cleanup(func() { store.FailAccesses(nil) })

			before := store.Accesses()
			rep := do(t, signed(t, http.MethodGet, "/counterparties", envoyID))
			requireReply(t, rep, http.StatusInternalServerError, InternalError)
			requireAccessed(t, before)
		})
	})

	t.Run("LoginRequired", func(t *testing.T) {
		// HTML requests are authenticated by the session cookie, so logging in
		// requires a client that stores cookies like a browser does.
		jar, err := cookiejar.New(nil)
		require.NoError(t, err)
		browser := &http.Client{Transport: client.Transport, Jar: jar, CheckRedirect: client.CheckRedirect}

		html := func(t *testing.T, method, path string) *http.Request {
			t.Helper()
			req, err := http.NewRequest(method, srv.URL+path, nil)
			require.NoError(t, err)
			req.Header.Set("Accept", binding.MIMEHTML)
			return req
		}

		requireLoginRedirect := func(t *testing.T, rep *http.Response) {
			t.Helper()
			require.Equal(t, http.StatusTemporaryRedirect, rep.StatusCode)
			require.Equal(t, "/login", rep.Header.Get("Location"))
		}

		t.Run("Redirect", func(t *testing.T) {
			// Without a session, any request that can be answered with HTML is
			// redirected to login, including a request with no Accept header.
			tests := []struct {
				name   string
				accept string
			}{
				{"HTML", "text/html"},
				{"Browser", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"},
				{"HTMLPreferred", "text/html, application/json"},
				{"Wildcard", "*/*"},
				{"NoAccept", ""},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					req := html(t, http.MethodGet, "/counterparties")
					req.Header.Del("Accept")
					if tc.accept != "" {
						req.Header.Set("Accept", tc.accept)
					}

					requireLoginRedirect(t, do(t, req))
				})
			}
		})

		t.Run("LicenseNotAccepted", func(t *testing.T) {
			// A license does not authenticate a request for HTML.
			req := signed(t, http.MethodGet, "/counterparties", adminID)
			req.Header.Set("Accept", binding.MIMEHTML)
			requireLoginRedirect(t, do(t, req))
		})

		t.Run("ForgedSession", func(t *testing.T) {
			// A session cookie that was not issued by the server is not a login.
			req := html(t, http.MethodGet, "/counterparties")
			req.AddCookie(&http.Cookie{Name: sessionName, Value: "forged"})
			requireLoginRedirect(t, do(t, req))
		})

		// Login to create a session for the remainder of the tests.
		requireLoginRedirect(t, execute(t, browser, html(t, http.MethodGet, "/counterparties")))
		rep := execute(t, browser, html(t, http.MethodPost, "/login"))
		require.Equal(t, http.StatusOK, rep.StatusCode)

		t.Run("LoggedIn", func(t *testing.T) {
			// A logged in user has all permissions and can access every route.
			tests := []struct {
				method string
				path   string
			}{
				{http.MethodGet, "/counterparties"},
				{http.MethodPost, "/counterparties"},
				{http.MethodGet, "/daybreak"},
			}

			accesses := store.Accesses()
			for _, tc := range tests {
				rep := execute(t, browser, html(t, tc.method, tc.path))
				require.Equal(t, http.StatusOK, rep.StatusCode, "expected %s %s to be allowed", tc.method, tc.path)
			}

			// A session is not a license, so no license access is recorded for it.
			require.Equal(t, accesses, store.Accesses(), "expected no license access to be recorded for a session")
		})

		t.Run("Identity", func(t *testing.T) {
			// Handlers are given the user that was saved to the session at login, once
			// it has made the round trip through the session cookie, along with the
			// wildcard permission that is granted to every logged in user.
			rep := execute(t, browser, html(t, http.MethodGet, "/whoami"))
			require.Equal(t, http.StatusOK, rep.StatusCode)

			identity := &Identity{}
			require.NoError(t, json.NewDecoder(rep.Body).Decode(identity))
			require.Equal(t, testUser, identity.User)
			require.Equal(t, []string{AllPermissions}, identity.Permissions)
		})

		t.Run("SessionNotAccepted", func(t *testing.T) {
			// A session does not authenticate a request for JSON, a license is required.
			req := html(t, http.MethodGet, "/counterparties")
			req.Header.Set("Accept", binding.MIMEJSON)

			rep := execute(t, browser, req)
			requireRejected(t, rep, http.StatusBadRequest, api.Reply{Error: "missing required header: Authorization"})
		})
	})

	t.Run("NotAcceptable", func(t *testing.T) {
		// Only HTML and JSON can be negotiated. The requests are otherwise valid and
		// would be authenticated by the license if they accepted JSON.
		tests := []struct {
			name   string
			accept string
		}{
			{"Text", "text/plain"},
			{"XML", "application/xml"},
			{"Image", "image/png"},
			{"Multiple", "text/csv, application/pdf;q=0.9"},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				lookups := store.calls.Load()
				req := signed(t, http.MethodGet, "/counterparties", adminID)
				req.Header.Set("Accept", tc.accept)

				rep := do(t, req)
				require.Equal(t, http.StatusNotAcceptable, rep.StatusCode)
				require.Empty(t, rep.Header.Get(ServerAuthorization), "expected the response to not be signed by the server")
				require.Equal(t, lookups, store.calls.Load(), "expected the request to be rejected before the license is retrieved")

				body, err := io.ReadAll(rep.Body)
				require.NoError(t, err)
				require.Empty(t, body)
			})
		}
	})

	t.Run("MissingRequiredHeader", func(t *testing.T) {
		for _, header := range []string{Authorization, XRequestID, XTimestamp} {
			t.Run(header, func(t *testing.T) {
				req := signed(t, http.MethodGet, "/counterparties", envoyID)
				req.Header.Del(header)

				rep := do(t, req)
				requireRejected(t, rep, http.StatusBadRequest, api.Reply{Error: "missing required header: " + header})
			})
		}

		t.Run("Unsigned", func(t *testing.T) {
			// A request with no authentication at all reports the first missing header.
			req, err := http.NewRequest(http.MethodGet, srv.URL+"/counterparties", nil)
			require.NoError(t, err)
			req.Header.Set("Accept", binding.MIMEJSON)

			rep := do(t, req)
			requireRejected(t, rep, http.StatusBadRequest, api.Reply{Error: "missing required header: Authorization"})
		})
	})

	t.Run("InvalidTimestamp", func(t *testing.T) {
		// The timestamp is signed in every case so that it is the only thing that is
		// wrong with the request.
		timestamped := func(t *testing.T, timestamp string) *http.Response {
			t.Helper()
			req := signed(t, http.MethodGet, "/counterparties", envoyID)
			req.Header.Set(XTimestamp, timestamp)
			resign(t, req, envoyID)
			return do(t, req)
		}

		//cSpell: ignore Unparseable
		t.Run("Unparseable", func(t *testing.T) {
			now := time.Now()
			tests := []struct {
				name      string
				timestamp string
			}{
				{"Text", "now"},
				{"Unix", fmt.Sprintf("%d", now.Unix())},
				{"DateOnly", now.Format(time.DateOnly)},
				{"DateTime", now.Format(time.DateTime)},
				{"RFC1123", now.Format(time.RFC1123)},
				{"NoTimezone", now.Format("2006-01-02T15:04:05")},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					rep := timestamped(t, tc.timestamp)
					requireRejected(t, rep, http.StatusBadRequest, api.Reply{Error: "invalid request timestamp"})
				})
			}
		})

		t.Run("OutsideWindow", func(t *testing.T) {
			tests := []struct {
				name    string
				elapsed time.Duration
			}{
				{"Past", -HawkWindow - time.Minute},
				{"Future", HawkWindow + time.Minute},
				{"LongPast", -24 * time.Hour},
				{"LongFuture", 24 * time.Hour},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					rep := timestamped(t, time.Now().Add(tc.elapsed).Format(time.RFC3339))
					requireRejected(t, rep, http.StatusUnauthorized, Unauthorized)
				})
			}
		})

		t.Run("InsideWindow", func(t *testing.T) {
			// The window applies in both directions to allow for clock skew, and the
			// timestamp does not have to be in UTC or truncated to the second.
			tests := []struct {
				name      string
				timestamp string
			}{
				{"Past", time.Now().Add(-HawkWindow + 30*time.Second).Format(time.RFC3339)},
				{"Future", time.Now().Add(HawkWindow - 30*time.Second).Format(time.RFC3339)},
				{"UTC", time.Now().UTC().Format(time.RFC3339)},
				{"Timezone", time.Now().In(time.FixedZone("JST", 9*60*60)).Format(time.RFC3339)},
				{"Nanoseconds", time.Now().Format(time.RFC3339Nano)},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					rep := timestamped(t, tc.timestamp)
					require.Equal(t, http.StatusOK, rep.StatusCode)
				})
			}
		})
	})

	t.Run("InvalidHawkToken", func(t *testing.T) {
		t.Run("Malformed", func(t *testing.T) {
			// Sign a request by hand to get the parts of a token that is valid for it.
			envoy := testLicenses[envoyID]
			req := signed(t, http.MethodGet, "/counterparties", envoyID)

			mac := hawk.New(envoy.ID.String(), envoy.Secret)
			mac.Append(XRequestID, req.Header.Get(XRequestID))
			mac.Append(XTimestamp, req.Header.Get(XTimestamp))

			sig, err := mac.Signature()
			require.NoError(t, err)
			nonce, headers, kid := mac.Nonce(), mac.Headers(), mac.Key()

			send := func(t *testing.T, token string) *http.Response {
				t.Helper()
				req := req.Clone(req.Context())
				req.Header.Set(Authorization, token)
				return do(t, req)
			}

			tests := []struct {
				name  string
				token string
			}{
				// This was a backdoor for testing in a previous version of the middleware.
				{"Bearer", "Bearer testing"},
				{"SchemeOnly", "HMAC"},
				{"NoScheme", fmt.Sprintf("sig=%s, nonce=%s, headers=%s, kid=%s", sig, nonce, headers, kid)},
				{"WrongScheme", fmt.Sprintf("Hawk sig=%s, nonce=%s, headers=%s, kid=%s", sig, nonce, headers, kid)},
				{"LowerCaseScheme", fmt.Sprintf("hmac sig=%s, nonce=%s, headers=%s, kid=%s", sig, nonce, headers, kid)},
				{"NotParameters", fmt.Sprintf("HMAC %s", sig)},
				{"MissingSignature", fmt.Sprintf("HMAC nonce=%s, headers=%s, kid=%s", nonce, headers, kid)},
				{"MissingNonce", fmt.Sprintf("HMAC sig=%s, headers=%s, kid=%s", sig, headers, kid)},
				{"MissingHeaders", fmt.Sprintf("HMAC sig=%s, nonce=%s, kid=%s", sig, nonce, kid)},
				{"MissingKeyID", fmt.Sprintf("HMAC sig=%s, nonce=%s, headers=%s", sig, nonce, headers)},
				{"EmptyHeaders", fmt.Sprintf("HMAC sig=%s, nonce=%s, headers=, kid=%s", sig, nonce, kid)},
				{"EmptyKeyID", fmt.Sprintf("HMAC sig=%s, nonce=%s, headers=%s, kid=", sig, nonce, headers)},
				{"SignatureNotBase64", fmt.Sprintf("HMAC sig=!!!, nonce=%s, headers=%s, kid=%s", nonce, headers, kid)},
				{"NonceNotBase64", fmt.Sprintf("HMAC sig=%s, nonce=!!!, headers=%s, kid=%s", sig, headers, kid)},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					lookups := store.calls.Load()
					rep := send(t, tc.token)
					requireRejected(t, rep, http.StatusUnauthorized, Unauthorized)
					require.Equal(t, lookups, store.calls.Load(), "expected the token to be rejected before the license is retrieved")
				})
			}

			// The same request is accepted with the well formed token, so the tokens
			// above could only have been rejected because they were malformed.
			rep := send(t, fmt.Sprintf("HMAC sig=%s, nonce=%s, headers=%s, kid=%s", sig, nonce, headers, kid))
			require.Equal(t, http.StatusOK, rep.StatusCode)
		})

		t.Run("Unverified", func(t *testing.T) {
			// These tokens are well formed and identify a license, but the signature
			// does not match the request that was received.
			tests := []struct {
				name   string
				tamper func(t *testing.T, req *http.Request)
			}{
				{"WrongSecret", func(t *testing.T, req *http.Request) {
					require.NoError(t, signTestRequest(req, envoyID.String(), makeSecret()))
				}},
				{"OtherLicenseSecret", func(t *testing.T, req *http.Request) {
					require.NoError(t, signTestRequest(req, envoyID.String(), testLicenses[adminID].Secret))
				}},
				{"TamperedRequestID", func(t *testing.T, req *http.Request) {
					req.Header.Set(XRequestID, uuid.New().String())
				}},
				{"TamperedTimestamp", func(t *testing.T, req *http.Request) {
					req.Header.Set(XTimestamp, time.Now().Add(-time.Minute).Format(time.RFC3339))
				}},
				{"ReorderedHeaders", func(t *testing.T, req *http.Request) {
					token := req.Header.Get(Authorization)
					reordered := strings.Replace(token, "headers=x-request-id;x-timestamp", "headers=x-timestamp;x-request-id", 1)
					require.NotEqual(t, token, reordered)
					req.Header.Set(Authorization, reordered)
				}},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					lookups := store.calls.Load()
					req := signed(t, http.MethodGet, "/counterparties", envoyID)
					tc.tamper(t, req)

					rep := do(t, req)
					requireRejected(t, rep, http.StatusUnauthorized, Unauthorized)
					require.Equal(t, lookups+1, store.calls.Load(), "expected the license to be retrieved to verify the signature")
				})
			}
		})

		t.Run("UnsignedHeaders", func(t *testing.T) {
			// The client chooses which headers its token signs, so the token must sign
			// both the request ID and the timestamp; any that are not signed could be
			// changed to replay the request. These tokens are correctly signed with
			// the secret of the license, but do not sign both of the headers.
			envoy := testLicenses[envoyID]
			tests := []struct {
				name    string
				headers []string
			}{
				{"TimestampOnly", []string{XTimestamp}},
				{"RequestIDOnly", []string{XRequestID}},
				{"Unrelated", []string{"Accept"}},
				{"UnrelatedAndTimestamp", []string{"Accept", XTimestamp}},
				{"UnrelatedAndRequestID", []string{XRequestID, "Accept"}},
				{"SimilarName", []string{"X-Request", XTimestamp}},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					lookups := store.calls.Load()
					req := signed(t, http.MethodGet, "/counterparties", envoyID)
					require.NoError(t, signTestRequest(req, envoy.ID.String(), envoy.Secret, tc.headers...))

					rep := do(t, req)
					requireRejected(t, rep, http.StatusUnauthorized, Unauthorized)
					require.Equal(t, lookups, store.calls.Load(), "expected the token to be rejected before the license is retrieved")
				})
			}

			// A token that does not sign the request ID could otherwise be replayed
			// by sending it again with a request ID that has not been seen before.
			t.Run("Replay", func(t *testing.T) {
				req := signed(t, http.MethodGet, "/counterparties", envoyID)
				require.NoError(t, signTestRequest(req, envoy.ID.String(), envoy.Secret, XTimestamp))

				for range 2 {
					req = req.Clone(req.Context())
					req.Header.Set(XRequestID, uuid.New().String())
					requireRejected(t, do(t, req), http.StatusUnauthorized, Unauthorized)
				}
			})

			// The headers may be signed in any order and with additional headers, and
			// the names of the headers in the token are not case sensitive.
			t.Run("Signed", func(t *testing.T) {
				tests := []struct {
					name string
					sign func(t *testing.T, req *http.Request)
				}{
					{"Reordered", func(t *testing.T, req *http.Request) {
						require.NoError(t, signTestRequest(req, envoy.ID.String(), envoy.Secret, XTimestamp, XRequestID))
					}},
					{"Additional", func(t *testing.T, req *http.Request) {
						require.NoError(t, signTestRequest(req, envoy.ID.String(), envoy.Secret, "Accept", XRequestID, XTimestamp))
					}},
					{"MixedCase", func(t *testing.T, req *http.Request) {
						token := req.Header.Get(Authorization)
						mixed := strings.Replace(token, "headers=x-request-id;x-timestamp", "headers=X-Request-ID;X-Timestamp", 1)
						require.NotEqual(t, token, mixed)
						req.Header.Set(Authorization, mixed)
					}},
				}

				for _, tc := range tests {
					t.Run(tc.name, func(t *testing.T) {
						req := signed(t, http.MethodGet, "/counterparties", envoyID)
						tc.sign(t, req)

						rep := do(t, req)
						require.Equal(t, http.StatusOK, rep.StatusCode)
						requireServerAuth(t, req, rep, envoy)
					})
				}
			})
		})
	})

	t.Run("InvalidClientID", func(t *testing.T) {
		// The requests are signed with the secret of a license, so the key ID is the
		// only thing that is wrong with the token.
		tests := []struct {
			name  string
			keyID string
		}{
			{"Text", "envoy"},
			{"UUID", uuid.New().String()},
			{"Truncated", envoyID.String()[:25]},
			{"TooLong", envoyID.String() + "0"},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				lookups := store.calls.Load()
				req := signed(t, http.MethodGet, "/counterparties", envoyID)
				require.NoError(t, signTestRequest(req, tc.keyID, testLicenses[envoyID].Secret))

				rep := do(t, req)
				requireRejected(t, rep, http.StatusUnauthorized, Unauthorized)
				require.Equal(t, lookups, store.calls.Load(), "expected the client ID to be rejected before a license is retrieved")
			})
		}
	})

	t.Run("ClientIDNotFound", func(t *testing.T) {
		tests := []struct {
			name     string
			clientID ulid.ULID
		}{
			{"Unknown", ulid.Make()},
			{"Zero", ulid.Zero},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				lookups := store.calls.Load()
				req := signed(t, http.MethodGet, "/counterparties", envoyID)
				require.NoError(t, signTestRequest(req, tc.clientID.String(), testLicenses[envoyID].Secret))

				rep := do(t, req)
				requireRejected(t, rep, http.StatusUnauthorized, Unauthorized)
				require.Equal(t, lookups+1, store.calls.Load(), "expected the license to be looked up by client ID")
			})
		}
	})

	t.Run("ExpiredLicense", func(t *testing.T) {
		// The license has the permissions required for the route but is expired.
		for _, path := range []string{"/counterparties", "/daybreak"} {
			rep := do(t, signed(t, http.MethodGet, path, expiredID))
			requireRejected(t, rep, http.StatusForbidden, Unauthorized)
		}

		// Without the secret a request cannot be used to discover the license is expired.
		req := signed(t, http.MethodGet, "/counterparties", expiredID)
		require.NoError(t, signTestRequest(req, expiredID.String(), makeSecret()))
		requireRejected(t, do(t, req), http.StatusUnauthorized, Unauthorized)
	})

	t.Run("InvalidRequestID", func(t *testing.T) {
		// The request ID is signed in every case so that it is the only thing that
		// is wrong with the request.
		tests := []struct {
			name      string
			requestID string
		}{
			{"Text", "not-a-request-id"},
			{"Number", "12345"},
			{"UUIDTooLong", uuid.New().String() + "0"},
			{"ULIDTooLong", ulid.Make().String() + "0"},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				req := signed(t, http.MethodGet, "/counterparties", envoyID)
				req.Header.Set(XRequestID, tc.requestID)
				resign(t, req, envoyID)

				rep := do(t, req)
				requireRejected(t, rep, http.StatusBadRequest, api.Reply{Error: "invalid request ID"})
			})
		}

		t.Run("Unverified", func(t *testing.T) {
			// The request ID is not validated until the request has been verified.
			req := signed(t, http.MethodGet, "/counterparties", envoyID)
			req.Header.Set(XRequestID, "not-a-request-id")
			require.NoError(t, signTestRequest(req, envoyID.String(), makeSecret()))
			requireRejected(t, do(t, req), http.StatusUnauthorized, Unauthorized)
		})
	})

	t.Run("ReplayedRequestID", func(t *testing.T) {
		t.Run("Resent", func(t *testing.T) {
			req := signed(t, http.MethodGet, "/counterparties", envoyID)
			require.Equal(t, http.StatusOK, do(t, req).StatusCode)

			// The exact same request cannot be sent again.
			for range 2 {
				rep := do(t, req.Clone(req.Context()))
				requireRejected(t, rep, http.StatusUnauthorized, Unauthorized)
			}

			// A replay does not prevent new requests from being accepted.
			require.Equal(t, http.StatusOK, do(t, signed(t, http.MethodGet, "/counterparties", envoyID)).StatusCode)
		})

		t.Run("Resigned", func(t *testing.T) {
			// The request ID cannot be reused in a new request, even if it is signed
			// again, sent to a different route, or encoded differently.
			requestID := uuid.New().String()
			tests := []struct {
				name      string
				method    string
				path      string
				requestID string
			}{
				{"SameRoute", http.MethodGet, "/counterparties", requestID},
				{"OtherRoute", http.MethodGet, "/daybreak", requestID},
				{"OtherMethod", http.MethodPost, "/counterparties", requestID},
				{"UpperCase", http.MethodGet, "/counterparties", strings.ToUpper(requestID)},
			}

			req := signed(t, http.MethodGet, "/counterparties", adminID)
			req.Header.Set(XRequestID, requestID)
			resign(t, req, adminID)
			require.Equal(t, http.StatusOK, do(t, req).StatusCode)

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					req := signed(t, tc.method, tc.path, adminID)
					req.Header.Set(XRequestID, tc.requestID)
					resign(t, req, adminID)
					requireRejected(t, do(t, req), http.StatusUnauthorized, Unauthorized)
				})
			}
		})

		t.Run("Unverified", func(t *testing.T) {
			// A request ID is only remembered once the request has been verified, so
			// a request that is rejected cannot be used to block a legitimate request.
			requestID := uuid.New().String()

			req := signed(t, http.MethodGet, "/counterparties", envoyID)
			req.Header.Set(XRequestID, requestID)
			require.NoError(t, signTestRequest(req, envoyID.String(), makeSecret()))
			requireRejected(t, do(t, req), http.StatusUnauthorized, Unauthorized)

			req = signed(t, http.MethodGet, "/counterparties", envoyID)
			req.Header.Set(XRequestID, requestID)
			resign(t, req, envoyID)
			require.Equal(t, http.StatusOK, do(t, req).StatusCode)
		})
	})

	t.Run("NotPermitted", func(t *testing.T) {
		tests := []struct {
			name     string
			clientID ulid.ULID
			method   string
			path     string
			status   int
		}{
			// No permissions are required to list counterparties, only authentication.
			{"NoPermissions/List", noPermissionsID, http.MethodGet, "/counterparties", http.StatusOK},
			{"NoPermissions/Create", noPermissionsID, http.MethodPost, "/counterparties", http.StatusForbidden},
			{"NoPermissions/Daybreak", noPermissionsID, http.MethodGet, "/daybreak", http.StatusForbidden},
			{"Envoy/List", envoyID, http.MethodGet, "/counterparties", http.StatusOK},
			{"Envoy/Create", envoyID, http.MethodPost, "/counterparties", http.StatusForbidden},
			{"Envoy/Daybreak", envoyID, http.MethodGet, "/daybreak", http.StatusOK},
			{"Admin/Create", adminID, http.MethodPost, "/counterparties", http.StatusOK},
			{"Admin/Daybreak", adminID, http.MethodGet, "/daybreak", http.StatusOK},
			{"Wildcard/Create", wildcardID, http.MethodPost, "/counterparties", http.StatusOK},
			{"Wildcard/Daybreak", wildcardID, http.MethodGet, "/daybreak", http.StatusOK},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				req := signed(t, tc.method, tc.path, tc.clientID)
				rep := do(t, req)

				if tc.status == http.StatusForbidden {
					requireReply(t, rep, http.StatusForbidden, Unauthorized)
				} else {
					require.Equal(t, tc.status, rep.StatusCode)
				}

				// The license is authenticated even when it is not permitted, so the
				// server signs the reply in either case.
				requireServerAuth(t, req, rep, testLicenses[tc.clientID])
			})
		}
	})
}

func TestGetUserInfo(t *testing.T) {
	// Creates a context with the value set as the user info, if there is one.
	contextWith := func(val any) *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		if val != nil {
			c.Set(UserInfoKey, val)
		}
		return c
	}

	t.Run("NotSet", func(t *testing.T) {
		require.Nil(t, GetUserInfo(contextWith(nil)))
	})

	t.Run("Value", func(t *testing.T) {
		// This is what is decoded from the session, even if a pointer was saved to it.
		user := GetUserInfo(contextWith(*testUser))
		require.Equal(t, testUser, user)
		require.NotSame(t, testUser, user)
	})

	t.Run("Pointer", func(t *testing.T) {
		require.Same(t, testUser, GetUserInfo(contextWith(testUser)))
	})

	t.Run("UnexpectedType", func(t *testing.T) {
		// Anything else on the context is a programming error, not a missing user.
		c := contextWith(map[string]string{"email": testUser.Email})
		require.PanicsWithError(t, "unexpected user info type on context: map[string]string", func() { GetUserInfo(c) })
	})
}

//============================================================================
// Test Token Identifier
//============================================================================

type TestTokenIdentifier struct {
	licenses map[ulid.ULID]*db.License
	calls    atomic.Int64

	// Accesses are recorded by the server while it handles a request and are read by
	// the tests, so the fields below must only be used while holding the lock.
	mu        sync.Mutex
	accesses  map[ulid.ULID]int
	accessErr error
}

func (t *TestTokenIdentifier) RetrieveLicense(ctx context.Context, id ulid.ULID) (license *db.License, _ error) {
	t.calls.Add(1)

	var ok bool
	if license, ok = t.licenses[id]; !ok {
		return nil, errors.ErrNotFound
	}
	return license, nil
}

func (t *TestTokenIdentifier) LicenseAccessed(ctx context.Context, id ulid.ULID) (err error) {
	t.calls.Add(1)

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.accessErr != nil {
		return t.accessErr
	}

	t.accesses[id]++
	return nil
}

// Returns a copy of the number of accesses that have been recorded for each license,
// keyed by the string of the license ID so that test failures are readable.
func (t *TestTokenIdentifier) Accesses() map[string]int {
	t.mu.Lock()
	defer t.mu.Unlock()

	accesses := make(map[string]int, len(t.accesses))
	for id, count := range t.accesses {
		accesses[id.String()] = count
	}
	return accesses
}

// Causes LicenseAccessed to return the error instead of recording the access, until
// it is called again with nil.
func (t *TestTokenIdentifier) FailAccesses(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.accessErr = err
}

//============================================================================
// Test Handlers
//============================================================================

// Ok is a test handler that returns a 200 OK response after the middleware has been applied.
func Ok(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// The user that is logged in by the Login test handler.
// cSpell: ignore WNNMCJSCEC6J3RC50Z4MV7
var testUser = &UserInfo{
	Sub:           "01M7WNNMCJSCEC6J3RC50Z4MV7",
	Name:          "Test User",
	Email:         "test@example.com",
	EmailVerified: true,
	HD:            "example.com",
	FamilyName:    "User",
	GivenName:     "Test",
	Picture:       "https://example.com/avatar.png",
}

// The user and permissions that the middleware set on the context of a request.
type Identity struct {
	User        *UserInfo `json:"user"`
	Permissions []string  `json:"permissions"`
}

// WhoAmI is a test handler that returns the identity the middleware authenticated.
func WhoAmI(c *gin.Context) {
	c.JSON(http.StatusOK, Identity{User: GetUserInfo(c), Permissions: GetPermissions(c)})
}

// Sets a user session cookie on the request and returns a 200 OK response.
func Login(c *gin.Context) {
	session := sessions.Default(c)
	session.Set(UserSessionKey, testUser)

	if err := session.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

//============================================================================
// Test Helper Functions
//============================================================================

func newTestRequest(method, path string, clientID ulid.ULID, body io.Reader) (req *http.Request, err error) {
	if req, err = http.NewRequest(method, path, body); err != nil {
		return nil, err
	}

	// Do not authorize the request if the client ID is zero.
	if clientID.IsZero() {
		return req, nil
	}

	// Do not authorize the request if the client ID is not found.
	license, ok := testLicenses[clientID]
	if !ok {
		return req, nil
	}

	// Add Hawk authentication headers.
	req.Header.Set(XRequestID, uuid.New().String())
	req.Header.Set(XTimestamp, time.Now().Add(-3*time.Millisecond).Format(time.RFC3339))

	if err = signTestRequest(req, clientID.String(), license.Secret); err != nil {
		return nil, err
	}
	return req, nil
}

// Signs the headers that are currently on the request, replacing its Authorization
// header; the X-Request-ID and X-Timestamp headers are signed if none are specified.
// Use this to sign a request again after its headers are modified or to sign a request
// with credentials that are not a license.
func signTestRequest(req *http.Request, keyID string, secret []byte, headers ...string) (err error) {
	if len(headers) == 0 {
		headers = []string{XRequestID, XTimestamp}
	}

	mac := hawk.New(keyID, secret)
	for _, header := range headers {
		mac.Append(header, req.Header.Get(header))
	}

	var auth string
	if auth, err = mac.Authorization(); err != nil {
		return err
	}

	req.Header.Set(Authorization, auth)
	return nil
}

// Executes the request with the client; the response body is closed when the test ends.
func execute(t *testing.T, client *http.Client, req *http.Request) *http.Response {
	t.Helper()

	rep, err := client.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { rep.Body.Close() })
	return rep
}

// Asserts the response has the expected status code and JSON error reply.
func requireReply(t *testing.T, rep *http.Response, status int, expected api.Reply) {
	t.Helper()
	require.Equal(t, status, rep.StatusCode)

	reply := api.Reply{}
	require.NoError(t, json.NewDecoder(rep.Body).Decode(&reply))
	require.Equal(t, expected, reply)
}

// Asserts the request was not authenticated by the license middleware: the response
// must have the expected status code and error reply and must not be signed.
func requireRejected(t *testing.T, rep *http.Response, status int, expected api.Reply) {
	t.Helper()
	requireReply(t, rep, status, expected)
	require.Empty(t, rep.Header.Get(ServerAuthorization), "expected the response to not be signed by the server")
}

// Asserts the response to the request was signed by the server with the secret of the
// license, which proves to the client that the reply came from the license holder.
func requireServerAuth(t *testing.T, req *http.Request, rep *http.Response, license *db.License) {
	t.Helper()

	// The request ID is echoed so that the reply cannot be replayed for another request.
	require.Equal(t, req.Header.Get(XRequestID), rep.Header.Get(XRequestID))
	require.NotEmpty(t, rep.Header.Get(XResponseID))

	ok, err := ValidateTimestamp(rep.Header.Get(XTimestamp))
	require.NoError(t, err)
	require.True(t, ok, "expected the response timestamp to be in the allowed window")

	token, err := hawk.Parse(rep.Header.Get(ServerAuthorization))
	require.NoError(t, err)
	require.Equal(t, license.ID.String(), token.KeyID())
	require.Equal(t, []string{"x-response-id", "x-timestamp", "x-request-id"}, token.Headers())

	token.Collect(rep.Header)
	ok, err = token.Verify(license.Secret)
	require.NoError(t, err)
	require.True(t, ok, "expected the response to be signed with the license secret")
}

func makeSecret() []byte {
	secret := make([]byte, 32)
	rand.Read(secret)
	return secret
}
