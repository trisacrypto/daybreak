package api

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/trisacrypto/daybreak/pkg/db"
	"go.rtnl.ai/ulid"
	"go.rtnl.ai/x/validation"
)

type Client interface {
	Status(context.Context) (*StatusReply, error)
	Sync(context.Context, time.Time) (*SyncReply, time.Time, error)
	Download(context.Context, time.Time, io.Writer) (time.Time, error)
	DownloadTo(context.Context, time.Time, string) error
	ListCompanies(context.Context) ([]*Company, error)
	CreateCompany(context.Context, *Company) (*Company, error)
	RetrieveCompany(context.Context, uuid.UUID) (*Company, error)
	UpdateCompany(context.Context, *Company) (*Company, error)
	DeleteCompany(context.Context, uuid.UUID) error
	ListCompanyContacts(context.Context, uuid.UUID) ([]*Contact, error)
	CreateCompanyContact(context.Context, uuid.UUID, *Contact) (*Contact, error)
	ListContacts(context.Context) ([]*Contact, error)
	CreateContact(context.Context, *Contact) (*Contact, error)
	RetrieveContact(context.Context, uuid.UUID) (*Contact, error)
	UpdateContact(context.Context, *Contact) (*Contact, error)
	DeleteContact(context.Context, uuid.UUID) error
	ListLicenses(context.Context) ([]*License, error)
	CreateLicense(context.Context, *License) (*License, error)
	RetrieveLicense(context.Context, ulid.ULID) (*License, error)
	LicenseCredentials(context.Context, ulid.ULID) (*LicenseCredentials, error)
	UpdateLicense(context.Context, *License) (*License, error)
	DeleteLicense(context.Context, ulid.ULID) error
}

// Reply contains standard fields for generic API replies.
type Reply struct {
	Success     bool        `json:"success"`
	Error       string      `json:"error,omitempty"`
	ErrorDetail ErrorDetail `json:"error_detail,omitempty"`
}

// StatusReply is returned on status requests.
type StatusReply struct {
	Status  string `json:"status"`
	Uptime  string `json:"uptime,omitempty"`
	Version string `json:"version,omitempty"`
}

type Models[D any, T any, M db.Model[T]] interface {
	*D
	Model() M
}

type DTO struct {
	ID       ulid.ULID `json:"id,omitzero,omitempty"`       // readonly field
	Created  time.Time `json:"created,omitzero,omitempty"`  // readonly field
	Modified time.Time `json:"modified,omitzero,omitempty"` // readonly field
}

func (d *DTO) Validate(method string) (err error) {
	if !d.ID.IsZero() {
		err = validation.Error(err, validation.ReadOnlyField("id"))
	}
	if !d.Created.IsZero() {
		err = validation.Error(err, validation.ReadOnlyField("created"))
	}
	if !d.Modified.IsZero() {
		err = validation.Error(err, validation.ReadOnlyField("modified"))
	}
	return err
}

func (d *DTO) IsZero() bool {
	return d == nil || (d.ID.IsZero() && d.Created.IsZero() && d.Modified.IsZero())
}
