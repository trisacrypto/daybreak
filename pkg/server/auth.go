package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/trisacrypto/daybreak/pkg"
	"github.com/trisacrypto/daybreak/pkg/api/v1"
	"github.com/trisacrypto/daybreak/pkg/errors"
	"golang.org/x/oauth2"
)

const (
	oauthStateTTL = 20 * time.Minute
	oauthStateKey = "oauth_state"
)

func (s *Server) AuthLogin(ctx context.Context, c *gin.Context) *errors.HTTP {
	redirect := s.auth.AuthCodeURL(oauthStateCookie(c))
	c.Redirect(http.StatusTemporaryRedirect, redirect)
	return nil
}

func (s *Server) AuthLogout(ctx context.Context, c *gin.Context) *errors.HTTP {
	session := sessions.Default(c)
	session.Clear()
	if err := session.Save(); err != nil {
		return api.Error(http.StatusInternalServerError, err, "failed to save session")
	}

	// TODO: Redirect to the login page
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
	return nil
}

func (s *Server) AuthCallback(ctx context.Context, c *gin.Context) *errors.HTTP {
	var (
		err      error
		token    *oauth2.Token
		userinfo map[string]any
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

	// Save the user information to the session
	session := sessions.Default(c)
	session.Set("user", userinfo)
	if err := session.Save(); err != nil {
		return api.Error(http.StatusInternalServerError, err, "failed to save session")
	}

	// Redirect the user back to the dashboard
	c.JSON(http.StatusOK, userinfo)
	return nil
}

func oauthStateCookie(c *gin.Context) string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Errorf("failed to generate random bytes: %w", err))
	}

	state := base64.URLEncoding.EncodeToString(buf)
	c.SetCookie(oauthStateKey, state, int(oauthStateTTL.Seconds()), "/", "", false, true)
	return state
}

func userInfo(ctx context.Context, token *oauth2.Token) (map[string]any, error) {
	var (
		err error
		req *http.Request
		rep *http.Response
	)

	if req, err = http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v3/userinfo", nil); err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("daybreak/%s", pkg.Version(false)))
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))

	if rep, err = http.DefaultClient.Do(req); err != nil {
		return nil, err
	}
	defer rep.Body.Close()

	userinfo := make(map[string]any)
	if err = json.NewDecoder(rep.Body).Decode(&userinfo); err != nil {
		return nil, err
	}

	return userinfo, nil
}
