package api

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"net/mail"
	"slices"
	"strings"
	"time"

	"github.com/trisacrypto/daybreak/pkg/db"
	"github.com/trisacrypto/daybreak/pkg/db/fields"
	"go.rtnl.ai/ulid"
	"go.rtnl.ai/x/validation"
)

// This are the valid permissions that can be assigned to a license.
var validPermissions = []string{
	"counterparties:admin",
	"counterparties:create",
	"daybreak:sync",
	"daybreak:download",
	"licenses:admin",
}

type License struct {
	ID          ulid.ULID  `json:"id"`
	Name        string     `json:"name"`
	Email       string     `json:"email"`
	Description string     `json:"description"`
	Permissions []string   `json:"permissions"`
	Syncs       int64      `json:"syncs"`
	Downloads   int64      `json:"downloads"`
	LastAccess  *time.Time `json:"last_access"`
	Expires     time.Time  `json:"expires,omitzero"`
	Created     time.Time  `json:"created,omitzero"`
	Modified    time.Time  `json:"modified,omitzero"`
}

type LicenseCredentials struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

func NewLicense(model *db.License) *License {
	license := &License{
		ID:          model.ID,
		Name:        model.Name,
		Email:       model.Email,
		Description: model.Description.String,
		Permissions: model.Permissions,
		Syncs:       model.Syncs,
		Downloads:   model.Downloads,
		Expires:     model.Expires.Time(),
		Created:     model.Created.Time(),
		Modified:    model.Modified.Time(),
	}

	if !model.LastAccess.IsZero() {
		lastAccess := model.LastAccess.Time()
		license.LastAccess = &lastAccess
	}

	return license
}

func NewLicenseCredentials(model *db.License) *LicenseCredentials {
	return &LicenseCredentials{
		ClientID:     model.ID.String(),
		ClientSecret: base64.StdEncoding.EncodeToString(model.Secret),
	}
}

func (l *License) Model() *db.License {
	return &db.License{
		Name:        l.Name,
		Email:       l.Email,
		Description: sql.NullString{String: l.Description, Valid: l.Description != ""},
		Permissions: l.Permissions,
		Expires:     fields.Time(l.Expires),
	}
}

func (l *License) Validate(method string) (err error) {
	if !l.ID.IsZero() {
		err = validation.Error(err, validation.ReadOnly("id"))
	}

	if !l.Created.IsZero() {
		err = validation.Error(err, validation.ReadOnly("created"))
	}

	if !l.Modified.IsZero() {
		err = validation.Error(err, validation.ReadOnly("modified"))
	}

	if l.Syncs != 0 {
		err = validation.Error(err, validation.ReadOnly("syncs"))
	}

	if l.Downloads != 0 {
		err = validation.Error(err, validation.ReadOnly("downloads"))
	}

	if l.LastAccess != nil && !l.LastAccess.IsZero() {
		err = validation.Error(err, validation.ReadOnly("last_access"))
	}

	l.Name = strings.TrimSpace(l.Name)
	if l.Name == "" {
		err = validation.Error(err, validation.Required("name"))
	}

	l.Email = strings.TrimSpace(l.Email)
	if l.Email == "" {
		err = validation.Error(err, validation.Required("email"))
	} else if _, perr := mail.ParseAddress(l.Email); perr != nil {
		err = validation.Error(err, validation.Invalid("email", perr.Error()))
	}

	l.Description = strings.TrimSpace(l.Description)

	if len(l.Permissions) == 0 {
		err = validation.Error(err, validation.Required("permissions"))
	}

	for i := range l.Permissions {
		l.Permissions[i] = strings.ToLower(strings.TrimSpace(l.Permissions[i]))
		if !slices.Contains(validPermissions, l.Permissions[i]) {
			msg := fmt.Sprintf("%q is not a valid permission", l.Permissions[i])
			err = validation.Error(
				err,
				validation.Invalid("permissions", msg).Index(i),
			)
		}
	}

	// Default the expiration to one year from now at hour granularity.
	if l.Expires.IsZero() {
		l.Expires = time.Now().UTC().AddDate(1, 0, 0).Truncate(time.Hour)
	}

	return err
}

func (l *License) Clone() *License {
	return &License{
		Name:        l.Name,
		Email:       l.Email,
		Description: l.Description,
		Permissions: l.Permissions,
		Expires:     l.Expires,
	}
}
