package config

import (
	"crypto/rand"
	"encoding/base64"
	"net/url"

	"go.rtnl.ai/confire"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type AuthConfig struct {
	ClientID       string   `env:"GOOGLE_CLIENT_ID" required:"true" split_words:"true" desc:"the client id for the google oauth2 application"`
	ClientSecret   string   `env:"GOOGLE_CLIENT_SECRET" required:"true" split_words:"true" desc:"the client secret for the google oauth2 application"`
	AllowedDomains []string `split_words:"true" required:"true" desc:"a list of allowed domains to allow access to the application"`
	SessionKey     string   `split_words:"true" required:"false" desc:"specify a 32 or 64 byte key for session authentication in base64 std encoding, if not specified one will be generated."`
}

func (c *AuthConfig) Validate() (err error) {
	if c.SessionKey != "" {
		if key, perr := base64.StdEncoding.DecodeString(c.SessionKey); perr != nil {
			err = confire.Join(err, confire.Invalid("auth", "session_key", "must be a valid base64 encoded string"))
		} else if len(key) != 32 && len(key) != 64 {
			err = confire.Join(err, confire.Invalid("auth", "session_key", "must be a 32 or 64 byte key"))
		}
	}

	return err
}

func (c *Config) OAuth2() *oauth2.Config {
	redirect, _ := url.Parse(c.Origin)
	redirect.Path = "/auth/callback"

	return &oauth2.Config{
		RedirectURL:  redirect.String(),
		ClientID:     c.Auth.ClientID,
		ClientSecret: c.Auth.ClientSecret,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}
}

func (c *AuthConfig) SessionKeyPairs() (out [][]byte) {
	out = make([][]byte, 0, 1)

	if c.SessionKey != "" {
		key, err := base64.StdEncoding.DecodeString(c.SessionKey)
		if err != nil {
			panic(err)
		}
		out = append(out, key)
	} else {
		key := make([]byte, 32)
		rand.Read(key)
		out = append(out, key)
	}

	return out
}

func (c *AuthConfig) DomainAllowed(hd string) bool {
	for _, domain := range c.AllowedDomains {
		if hd == domain {
			return true
		}
	}
	return false
}
