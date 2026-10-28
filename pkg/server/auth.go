package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/trisacrypto/daybreak/pkg"
	"github.com/trisacrypto/daybreak/pkg/api/v1"
	"github.com/trisacrypto/daybreak/pkg/errors"
	"github.com/trisacrypto/daybreak/pkg/web/scene"
	"go.rtnl.ai/gimlet/htmx"
	"golang.org/x/oauth2"
)

func init() {
	gob.Register(UserInfo{})
}

//============================================================================
// Handlers
//============================================================================

const (
	oauthStateTTL  = 20 * time.Minute
	oauthStateKey  = "oauth_state"
	userSessionKey = "user"
)

func (s *Server) LoginPage(ctx context.Context, c *gin.Context) *errors.HTTP {
	data := scene.New(c).ForPage("login")
	c.HTML(http.StatusOK, "auth/login/login.html", data)
	return nil
}

func (s *Server) AuthLogin(ctx context.Context, c *gin.Context) *errors.HTTP {
	redirect := s.auth.AuthCodeURL(oauthStateCookie(c))
	c.Redirect(http.StatusTemporaryRedirect, redirect)
	return nil
}

func (s *Server) AuthLogout(ctx context.Context, c *gin.Context) *errors.HTTP {
	session := sessions.Default(c)
	session.Delete(userSessionKey)
	session.Clear()

	if err := session.Save(); err != nil {
		return api.Error(http.StatusInternalServerError, err, "failed to save session")
	}

	// Redirect to the login page
	c.Redirect(http.StatusTemporaryRedirect, "/login")
	return nil
}

func (s *Server) AuthCallback(ctx context.Context, c *gin.Context) *errors.HTTP {
	var (
		err      error
		token    *oauth2.Token
		userinfo *UserInfo
	)

	// Validate state parameter against CSRF attacks
	if oauthState, cerr := c.Cookie(oauthStateKey); cerr != nil || oauthState != c.Query("state") {
		return api.Error(http.StatusBadRequest, errors.ErrInvalidOAuthState, "invalid oauth state")
	}

	// Clean up state cookie
	c.SetCookie(oauthStateKey, "", -1, "/", "", false, true)

	// Use the authorization code to get an access token
	if token, err = s.auth.Exchange(ctx, c.Query("code")); err != nil {
		return api.Error(http.StatusInternalServerError, err, "failed to exchange authorization code for token")
	}

	// Get the user information
	if userinfo, err = userInfo(ctx, token); err != nil {
		return api.Error(http.StatusInternalServerError, err, "failed to get user information")
	}

	// Validate the domains are correct
	if userinfo.HD == "" {
		return api.Error(http.StatusInternalServerError, errors.ErrInvalidUserInfo, "invalid userinfo returned from OAuth provider")
	}

	if !s.conf.Auth.DomainAllowed(userinfo.HD) {
		// TODO: return to login page with a flash message
		return api.Error(http.StatusForbidden, errors.ErrDomainNotAllowed, "only email addresses from allowed organizations are allowed")
	}

	// Save the user information to the session
	session := sessions.Default(c)
	session.Set(userSessionKey, userinfo)
	if err := session.Save(); err != nil {
		return api.Error(http.StatusInternalServerError, err, "failed to save session")
	}

	// Redirect the user back to the dashboard
	c.Redirect(http.StatusTemporaryRedirect, "/")
	return nil
}

//============================================================================
// Middleware
//============================================================================

// LoginRequired is a session based middleware that is for the UI only.
func LoginRequired(c *gin.Context) {
	session := sessions.Default(c)
	user := session.Get(userSessionKey)

	if user == nil {
		if htmx.IsHTMXRequest(c) {
			c.Redirect(http.StatusTemporaryRedirect, "/login")
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, api.Unauthorized)
		return
	}

	c.Next()
}

//============================================================================
// UserInfo
//============================================================================

type UserInfo struct {
	Sub           string `json:"sub"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	HD            string `json:"hd"`
	FamilyName    string `json:"family_name"`
	GivenName     string `json:"given_name"`
	Picture       string `json:"picture"`
}

//============================================================================
// Helpers
//============================================================================

func oauthStateCookie(c *gin.Context) string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Errorf("failed to generate random bytes: %w", err))
	}

	state := base64.URLEncoding.EncodeToString(buf)
	c.SetCookie(oauthStateKey, state, int(oauthStateTTL.Seconds()), "/", "", false, true)
	return state
}

const userinfoURL = "https://www.googleapis.com/oauth2/v3/userinfo"

func userInfo(ctx context.Context, token *oauth2.Token) (*UserInfo, error) {
	var (
		err error
		req *http.Request
		rep *http.Response
	)

	if req, err = http.NewRequestWithContext(ctx, http.MethodGet, userinfoURL, nil); err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("daybreak/%s", pkg.Version(false)))
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))

	if rep, err = http.DefaultClient.Do(req); err != nil {
		return nil, err
	}
	defer rep.Body.Close()

	userinfo := &UserInfo{}
	if err = json.NewDecoder(rep.Body).Decode(&userinfo); err != nil {
		return nil, err
	}

	return userinfo, nil
}
