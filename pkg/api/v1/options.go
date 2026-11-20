package api

import (
	"encoding/base64"
	"net/http"
	"time"

	"go.rtnl.ai/ulid"
	"go.rtnl.ai/x/hawk"
)

type Option func(*APIv1) error

func WithClient(client *http.Client) Option {
	return func(c *APIv1) error {
		c.client = client
		return nil
	}
}

func WithTransport(transport http.RoundTripper) Option {
	return func(c *APIv1) error {
		c.client.Transport = transport
		return nil
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(c *APIv1) error {
		c.client.Timeout = timeout
		return nil
	}
}

func WithCredentials(clientID ulid.ULID, clientSecret []byte) Option {
	transport := &hawk.Transport{
		Credentials: &hawk.Credential{
			ClientID: clientID.String(),
			Secret:   base64.RawStdEncoding.EncodeToString(clientSecret),
		},
	}
	return WithTransport(transport)
}
