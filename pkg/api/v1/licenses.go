package api

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
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
	ID          ulid.ULID      `json:"id"`
	Name        string         `json:"name"`
	Email       string         `json:"email"`
	Description string         `json:"description"`
	Permissions []string       `json:"permissions"`
	Syncs       int64          `json:"syncs"`
	Downloads   int64          `json:"downloads"`
	LastAccess  *time.Time     `json:"last_access"`
	Expires     ExpirationDate `json:"expires,omitzero"`
	Created     time.Time      `json:"created,omitzero"`
	Modified    time.Time      `json:"modified,omitzero"`
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
		Expires:     Expiration(model.Expires),
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
		Expires:     l.Expires.Timestamp(),
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
		l.Expires = ExpiresNextYear()
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

func (l *License) ExpirationStatusClass() string {
	if l.Expires.IsZero() {
		return "text-muted"
	}

	if l.Expires.Before(time.Now().UTC()) {
		return "text-danger"
	}

	if l.Expires.Before(time.Now().UTC().AddDate(0, 0, 60)) {
		return "text-warning"
	}

	return ""
}

//============================================================================
// Expires Timestamp
//============================================================================

type ExpirationDate struct {
	time.Time
}

func Expiration(t fields.Timestamp) ExpirationDate {
	return ExpirationDate{Time: t.Time()}
}

func ExpiresNextYear() ExpirationDate {
	return ExpirationDate{Time: time.Now().UTC().AddDate(1, 0, 0).Truncate(time.Hour)}
}

// The layouts that the expires timestamp of a license can be parsed from in addition
// to RFC3339. These layouts have no timezone, so they are interpreted as UTC.
var expiresLayouts = []string{
	time.DateTime,      // 2006-01-02 15:04:05
	"2006-01-02 15:04", // no seconds
	time.DateOnly,      // 2006-01-02
}

func (t *ExpirationDate) UnmarshalJSON(data []byte) (err error) {
	var ts time.Time
	if err = ts.UnmarshalJSON(data); err == nil {
		*t = ExpirationDate{Time: ts}
		return nil
	}

	var s string
	if json.Unmarshal(data, &s) != nil {
		return err
	}

	for _, layout := range expiresLayouts {
		if ts, err = time.Parse(layout, s); err == nil {
			*t = ExpirationDate{Time: ts}
			return nil
		}
	}

	return fmt.Errorf("cannot parse %q as an expiration date", s)
}

func (t ExpirationDate) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Format(time.DateOnly))
}

func (t ExpirationDate) Timestamp() fields.Timestamp {
	return fields.Time(t.Time)
}
