package api

import (
	"net/http"
	"time"
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
