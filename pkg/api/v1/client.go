package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/trisacrypto/daybreak/pkg"
)

func New(endpoint string, opts ...Option) (_ Client, err error) {
	client := &APIv1{
		client: &http.Client{
			Transport:     nil,
			CheckRedirect: nil,
			Jar:           nil,
			Timeout:       30 * time.Second,
		},
	}

	if client.endpoint, err = url.Parse(endpoint); err != nil {
		return nil, fmt.Errorf("invalid endpoint: %w", err)
	}

	for _, opt := range opts {
		if err = opt(client); err != nil {
			return nil, err
		}
	}
	return client, nil
}

type APIv1 struct {
	endpoint *url.URL
	client   *http.Client
}

const statusEP = "/v1/status"

func (c *APIv1) Status(ctx context.Context) (out *StatusReply, err error) {
	var req *http.Request
	if req, err = c.NewRequest(ctx, http.MethodGet, statusEP, nil); err != nil {
		return nil, err
	}

	var rep *http.Response
	if rep, err = c.client.Do(req); err != nil {
		return nil, fmt.Errorf("could not send request: %w", err)
	}
	defer rep.Body.Close()

	if (rep.StatusCode < 200 || rep.StatusCode >= 300) && rep.StatusCode != http.StatusServiceUnavailable {
		reply := &Reply{}

		if mediatype, _, err := mime.ParseMediaType(rep.Header.Get("Content-Type")); err == nil {
			switch mediatype {
			case "application/json":
				json.NewDecoder(rep.Body).Decode(reply)
			case "text/plain":
				data, _ := io.ReadAll(rep.Body)
				reply.Error = string(data)
			}
		}

		if reply.Error == "" {
			reply.Error = http.StatusText(rep.StatusCode)
		}

		return nil, &StatusError{
			Code:  rep.StatusCode,
			Reply: reply,
		}
	}

	out = &StatusReply{}
	if err = json.NewDecoder(rep.Body).Decode(out); err != nil {
		return nil, fmt.Errorf("could not deserialize response: %w", err)
	}
	return out, nil
}

//============================================================================
// Companies Resource
//============================================================================

const (
	companiesEP = "/v1/companies"
)

func (c *APIv1) ListCompanies(ctx context.Context) (out []*Company, err error) {
	out = make([]*Company, 0)
	if err = c.List(ctx, companiesEP, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *APIv1) CreateCompany(ctx context.Context, in *Company) (out *Company, err error) {
	if err = c.Create(ctx, companiesEP, in, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *APIv1) RetrieveCompany(ctx context.Context, id uuid.UUID) (out *Company, err error) {
	ep := fmt.Sprintf("%s/%s", companiesEP, id)
	if err = c.Retrieve(ctx, ep, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *APIv1) UpdateCompany(ctx context.Context, in *Company) (out *Company, err error) {
	ep := fmt.Sprintf("%s/%s", companiesEP, in.ID)

	// Ensure read-only fields are cleared
	if err = c.Update(ctx, ep, in.Clone(), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *APIv1) DeleteCompany(ctx context.Context, id uuid.UUID) (err error) {
	ep := fmt.Sprintf("%s/%s", companiesEP, id)
	if err = c.Delete(ctx, ep); err != nil {
		return err
	}
	return nil
}

//============================================================================
// Company Contacts Helpers
//============================================================================

const (
	companyContactsEP = "/v1/companies/%s/contacts"
)

func (c *APIv1) ListCompanyContacts(ctx context.Context, id uuid.UUID) (out []*Contact, err error) {
	ep := fmt.Sprintf(companyContactsEP, id)
	out = make([]*Contact, 0)
	if err = c.List(ctx, ep, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *APIv1) CreateCompanyContact(ctx context.Context, companyID uuid.UUID, in *Contact) (out *Contact, err error) {
	ep := fmt.Sprintf(companyContactsEP, companyID)
	if err = c.Create(ctx, ep, in, &out); err != nil {
		return nil, err
	}
	return out, nil
}

//============================================================================
// Contacts Resource
//============================================================================

var (
	contactsEP = "/v1/contacts"
)

func (c *APIv1) ListContacts(ctx context.Context) (out []*Contact, err error) {
	out = make([]*Contact, 0)
	if err = c.List(ctx, contactsEP, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *APIv1) CreateContact(ctx context.Context, in *Contact) (out *Contact, err error) {
	if err = c.Create(ctx, contactsEP, in, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *APIv1) RetrieveContact(ctx context.Context, id uuid.UUID) (out *Contact, err error) {
	ep := fmt.Sprintf("%s/%s", contactsEP, id)
	if err = c.Retrieve(ctx, ep, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *APIv1) UpdateContact(ctx context.Context, in *Contact) (out *Contact, err error) {
	ep := fmt.Sprintf("%s/%s", contactsEP, in.ID)

	// Ensure read-only fields are cleared
	if err = c.Update(ctx, ep, in.Clone(), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *APIv1) DeleteContact(ctx context.Context, id uuid.UUID) (err error) {
	ep := fmt.Sprintf("%s/%s", contactsEP, id)
	if err = c.Delete(ctx, ep); err != nil {
		return err
	}
	return nil
}

//============================================================================
// REST Helpers
//============================================================================

func (c *APIv1) List(ctx context.Context, path string, out any) (err error) {
	var req *http.Request
	if req, err = c.NewRequest(ctx, http.MethodGet, path, nil); err != nil {
		return err
	}

	if _, err = c.Do(req, out, true); err != nil {
		return err
	}
	return nil
}

func (c *APIv1) Create(ctx context.Context, path string, in, out any) (err error) {
	var req *http.Request
	if req, err = c.NewRequest(ctx, http.MethodPost, path, in); err != nil {
		return err
	}

	if _, err = c.Do(req, out, true); err != nil {
		return err
	}
	return nil
}

func (c *APIv1) Retrieve(ctx context.Context, path string, out any) (err error) {
	var req *http.Request
	if req, err = c.NewRequest(ctx, http.MethodGet, path, nil); err != nil {
		return err
	}

	if _, err = c.Do(req, out, true); err != nil {
		return err
	}
	return nil
}

func (c *APIv1) Update(ctx context.Context, path string, in, out any) (err error) {
	var req *http.Request
	if req, err = c.NewRequest(ctx, http.MethodPut, path, in); err != nil {
		return err
	}

	if _, err = c.Do(req, out, true); err != nil {
		return err
	}
	return nil
}

func (c *APIv1) Delete(ctx context.Context, path string) (err error) {
	var req *http.Request
	if req, err = c.NewRequest(ctx, http.MethodDelete, path, nil); err != nil {
		return err
	}

	if _, err = c.Do(req, nil, true); err != nil {
		return err
	}
	return nil
}

//============================================================================
// HTTP Client
//============================================================================

var (
	userAgent string = fmt.Sprintf("Daybreak/%s", pkg.Version(false))
)

func (c *APIv1) NewRequest(ctx context.Context, method, path string, data interface{}) (req *http.Request, err error) {
	// Resolve the URL reference from the path
	endpoint := c.endpoint.ResolveReference(&url.URL{Path: path})

	var body io.ReadWriter
	if data != nil {
		body = &bytes.Buffer{}
		if err = json.NewEncoder(body).Encode(data); err != nil {
			return nil, fmt.Errorf("could not serialize request data: %s", err)
		}
	} else {
		body = nil
	}

	// Create the http request
	if req, err = http.NewRequestWithContext(ctx, method, endpoint.String(), body); err != nil {
		return nil, fmt.Errorf("could not create request: %s", err)
	}

	// Set the headers on the request
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en")
	req.Header.Set("Accept-Charset", "utf-8")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Content-Type", "application/json")

	// Return the request
	return req, nil
}

func (c *APIv1) Do(req *http.Request, out any, checkStatus bool) (rep *http.Response, err error) {
	if rep, err = c.client.Do(req); err != nil {
		return nil, fmt.Errorf("could not send request: %s", err)
	}
	defer rep.Body.Close()

	if checkStatus {
		if rep.StatusCode < 200 || rep.StatusCode >= 300 {
			serr := &StatusError{
				Code:  rep.StatusCode,
				Reply: &Reply{},
			}

			if mediatype, _, err := mime.ParseMediaType(rep.Header.Get("Content-Type")); err == nil {
				switch mediatype {
				case "application/json":
					json.NewDecoder(rep.Body).Decode(serr.Reply)
				case "text/plain":
					data, _ := io.ReadAll(rep.Body)
					serr.Reply.Error = string(data)
				}
			}

			if serr.Reply.Error == "" {
				serr.Reply.Error = http.StatusText(rep.StatusCode)
			}
			return rep, serr
		}
	}

	if out != nil && rep.StatusCode >= 200 && rep.StatusCode < 300 {
		if err = json.NewDecoder(rep.Body).Decode(out); err != nil {
			return nil, fmt.Errorf("could not deserialize response: %s", err)
		}
	}
	return rep, nil
}
