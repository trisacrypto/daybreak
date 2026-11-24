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
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/trisacrypto/daybreak/pkg"
	"github.com/trisacrypto/daybreak/pkg/errors"
	"go.rtnl.ai/ulid"
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
// Sync and Download
//============================================================================

const syncEP = "/v1/sync"

func (c *APIv1) Sync(ctx context.Context, ifModifiedSince time.Time) (out *SyncReply, lastModified time.Time, err error) {
	var req *http.Request
	if req, err = c.NewRequest(ctx, http.MethodGet, syncEP, nil); err != nil {
		return nil, time.Time{}, err
	}

	req.Header.Set("Cache-Control", "no-cache")
	if !ifModifiedSince.IsZero() {
		req.Header.Set("If-Modified-Since", ifModifiedSince.UTC().Format(http.TimeFormat))
	}

	out = &SyncReply{}

	var rep *http.Response
	rep, err = c.Do(req, out, true)

	if rep != nil {
		lastModified, _ = time.Parse(http.TimeFormat, rep.Header.Get("Last-Modified"))
	}

	if err != nil {
		return nil, lastModified, err
	}
	return out, lastModified, nil
}

const downloadEP = "/v1/download"

func (c *APIv1) Download(ctx context.Context, ifModifiedSince time.Time, w io.Writer) (lastModified time.Time, err error) {
	var req *http.Request
	if req, err = c.NewRequest(ctx, http.MethodGet, downloadEP, nil); err != nil {
		return time.Time{}, err
	}

	req.Header.Set("Accept", "application/zip")
	req.Header.Set("Cache-Control", "no-cache")
	if !ifModifiedSince.IsZero() {
		req.Header.Set("If-Modified-Since", ifModifiedSince.UTC().Format(http.TimeFormat))
	}

	// NOTE: c.Do closes the response body, so the request is sent directly in order
	// to stream the body of the response to the writer.
	var rep *http.Response
	if rep, err = c.client.Do(req); err != nil {
		return time.Time{}, fmt.Errorf("could not send request: %s", err)
	}
	defer rep.Body.Close()

	lastModified, _ = time.Parse(http.TimeFormat, rep.Header.Get("Last-Modified"))
	if err = c.checkStatus(rep); err != nil {
		return lastModified, err
	}

	_, err = io.Copy(w, rep.Body)
	return lastModified, err
}

func (c *APIv1) DownloadTo(ctx context.Context, lastModified time.Time, dir string) (err error) {
	var req *http.Request
	if req, err = c.NewRequest(ctx, http.MethodGet, downloadEP, nil); err != nil {
		return err
	}

	req.Header.Set("Accept", "application/zip")
	req.Header.Set("Cache-Control", "no-cache")
	if !lastModified.IsZero() {
		req.Header.Set("If-Modified-Since", lastModified.UTC().Format(http.TimeFormat))
	}

	// NOTE: c.Do closes the response body, so the request is sent directly in order
	// to stream the body of the response to the file.
	var rep *http.Response
	if rep, err = c.client.Do(req); err != nil {
		return fmt.Errorf("could not send request: %s", err)
	}
	defer rep.Body.Close()

	if err = c.checkStatus(rep); err != nil {
		return err
	}

	var params map[string]string
	if _, params, err = mime.ParseMediaType(rep.Header.Get("Content-Disposition")); err != nil {
		return fmt.Errorf("could not parse content-disposition header: %w", err)
	}

	// cSpell:ignore fname
	fname, ok := params["filename"]
	if !ok || fname == "" {
		return errors.New("no filename in content-disposition header")
	}

	// Never trust a server-supplied path: strip any directory components
	fname = filepath.Base(fname)
	path := filepath.Join(dir, fname)

	var f *os.File
	if f, err = os.Create(path); err != nil {
		return fmt.Errorf("could not create file: %w", err)
	}
	defer f.Close()

	if _, err = io.Copy(f, rep.Body); err != nil {
		return fmt.Errorf("could not copy file: %w", err)
	}
	return nil
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
// Licenses Resource
//============================================================================

const (
	licensesEP = "/v1/licenses"
)

func (c *APIv1) ListLicenses(ctx context.Context) (out []*License, err error) {
	out = make([]*License, 0)
	if err = c.List(ctx, licensesEP, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *APIv1) CreateLicense(ctx context.Context, in *License) (out *License, err error) {
	if err = c.Create(ctx, licensesEP, in, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *APIv1) RetrieveLicense(ctx context.Context, id ulid.ULID) (out *License, err error) {
	ep := fmt.Sprintf("%s/%s", licensesEP, id)
	if err = c.Retrieve(ctx, ep, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *APIv1) LicenseCredentials(ctx context.Context, id ulid.ULID) (out *LicenseCredentials, err error) {
	ep := fmt.Sprintf("%s/%s/credentials", licensesEP, id)
	if err = c.Retrieve(ctx, ep, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *APIv1) UpdateLicense(ctx context.Context, in *License) (out *License, err error) {
	ep := fmt.Sprintf("%s/%s", licensesEP, in.ID)

	// Ensure read-only fields are cleared
	if err = c.Update(ctx, ep, in.Clone(), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *APIv1) DeleteLicense(ctx context.Context, id ulid.ULID) (err error) {
	ep := fmt.Sprintf("%s/%s", licensesEP, id)
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
		if err = c.checkStatus(rep); err != nil {
			return rep, err
		}
	}

	if out != nil && rep.StatusCode >= 200 && rep.StatusCode < 300 {
		if err = json.NewDecoder(rep.Body).Decode(out); err != nil {
			return rep, fmt.Errorf("could not deserialize response: %s", err)
		}
	}
	return rep, nil
}

// Returns ErrNotModified for a 304 response and a StatusError for any other non-2xx
// response; the error reply is read from the response body but the body is not closed.
func (c *APIv1) checkStatus(rep *http.Response) error {
	if rep.StatusCode == http.StatusNotModified {
		return errors.ErrNotModified
	}

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
		return serr
	}
	return nil
}
