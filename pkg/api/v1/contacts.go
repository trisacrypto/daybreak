package api

import (
	"database/sql"
	"encoding/json"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trisacrypto/daybreak/pkg/db"
	"github.com/trisacrypto/daybreak/pkg/db/fields"
	"github.com/trisacrypto/trisa/pkg/ivms101"
	"go.rtnl.ai/x/validation"
)

type Contact struct {
	ID        uuid.UUID       `json:"id"`
	Name      string          `json:"name"`
	Email     string          `json:"email"`
	Role      string          `json:"role"`
	IVMS101   json.RawMessage `json:"ivms101"`
	CompanyID uuid.UUID       `json:"company_id,omitzero"`
	Created   time.Time       `json:"created"`
	Modified  time.Time       `json:"modified"`
}

func NewContact(model *db.Contact) *Contact {
	return &Contact{
		ID:       model.ID,
		Name:     model.Name,
		Email:    model.Email,
		Role:     model.Role.String,
		IVMS101:  json.RawMessage(model.IVMS101.JSONB),
		Created:  model.Created.Time(),
		Modified: model.Modified.Time(),
	}
}

func (c *Contact) Model() *db.Contact {
	contact := &db.Contact{
		Name:    c.Name,
		Email:   c.Email,
		Role:    sql.NullString{String: c.Role, Valid: c.Role != ""},
		IVMS101: fields.NullJSONB{JSONB: fields.JSONB(c.IVMS101), Valid: len(c.IVMS101) > 0},
	}

	if c.CompanyID != uuid.Nil {
		contact.CompanyID = c.CompanyID
	}
	return contact
}

func (c *Contact) Validate(method string) (err error) {
	if c.ID != uuid.Nil {
		err = validation.Error(err, validation.ReadOnly("id"))
	}

	if !c.Created.IsZero() {
		err = validation.Error(err, validation.ReadOnly("created"))
	}

	if !c.Modified.IsZero() {
		err = validation.Error(err, validation.ReadOnly("modified"))
	}

	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		err = validation.Error(err, validation.Required("name"))
	}

	c.Email = strings.TrimSpace(c.Email)
	if c.Email == "" {
		err = validation.Error(err, validation.Required("email"))
	} else if _, perr := mail.ParseAddress(c.Email); perr != nil {
		err = validation.Error(err, validation.Invalid("email", perr.Error()))

	}

	c.Role = strings.TrimSpace(c.Role)

	c.IVMS101 = normalizeJSON(c.IVMS101)
	if len(c.IVMS101) > 0 {
		var person *ivms101.NaturalPerson
		if perr := json.Unmarshal(c.IVMS101, &person); perr != nil {
			err = validation.Error(err, validation.Invalid("ivms101", perr.Error()))
		} else {
			if perr := ivms101.Validate(person); perr != nil {
				err = validation.Error(err, validation.Invalid("ivms101", perr.Error()))
			}
		}
	}
	return err
}
