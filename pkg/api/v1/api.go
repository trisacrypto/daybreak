package api

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.rtnl.ai/ulid"
	"go.rtnl.ai/x/validation"
)

type Client interface {
	Status(context.Context) (*StatusReply, error)
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
