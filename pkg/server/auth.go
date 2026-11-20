package server

import (
	"context"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/trisacrypto/daybreak/pkg/api/v1"
	"github.com/trisacrypto/daybreak/pkg/auth"
	"github.com/trisacrypto/daybreak/pkg/errors"
	"github.com/trisacrypto/daybreak/pkg/web/scene"
	"golang.org/x/oauth2"
)

//============================================================================
// Handlers
//============================================================================

func (s *Server) LoginPage(ctx context.Context, c *gin.Context) *errors.HTTP {
	data := scene.New(c).ForPage("login")
	c.HTML(http.StatusOK, "auth/login/login.html", data)
	return nil
}

func (s *Server) AuthLogin(ctx context.Context, c *gin.Context) *errors.HTTP {
	redirect := s.auth.AuthCodeURL(auth.StateCookie(c))
	c.Redirect(http.StatusTemporaryRedirect, redirect)
	return nil
}

func (s *Server) AuthLogout(ctx context.Context, c *gin.Context) *errors.HTTP {
	session := sessions.Default(c)
	session.Delete(auth.UserSessionKey)
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
		userinfo *auth.UserInfo
	)

	// Validate state parameter against CSRF attacks
	if oauthState, cerr := c.Cookie(auth.OAuthStateKey); cerr != nil || oauthState != c.Query("state") {
		return api.Error(http.StatusBadRequest, errors.ErrInvalidOAuthState, "invalid oauth state")
	}

	// Clean up state cookie
	c.SetCookie(auth.OAuthStateKey, "", -1, "/", "", false, true)

	// Use the authorization code to get an access token
	if token, err = s.auth.Exchange(ctx, c.Query("code")); err != nil {
		return api.Error(http.StatusInternalServerError, err, "failed to exchange authorization code for token")
	}

	// Get the user information
	if userinfo, err = auth.Profile(ctx, token); err != nil {
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
	session.Set(auth.UserSessionKey, userinfo)
	if err := session.Save(); err != nil {
		return api.Error(http.StatusInternalServerError, err, "failed to save session")
	}

	// Redirect the user back to the dashboard
	c.Redirect(http.StatusTemporaryRedirect, "/")
	return nil
}
