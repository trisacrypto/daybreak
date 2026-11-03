package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trisacrypto/daybreak/pkg/db"
	"github.com/trisacrypto/daybreak/pkg/db/fields"
	"github.com/trisacrypto/lei"
	"github.com/trisacrypto/trisa/pkg/ivms101"
	"go.rtnl.ai/x/country"
	"go.rtnl.ai/x/validation"
)

type Company struct {
	ID               uuid.UUID       `json:"id"`
	LEI              string          `json:"lei"`
	Domain           string          `json:"domain"`
	Name             string          `json:"name"`
	Website          string          `json:"website"`
	Country          string          `json:"country"`
	BusinessCategory string          `json:"business_category"`
	VASPCategories   []string        `json:"vasp_categories"`
	IVMS101          json.RawMessage `json:"ivms101"`
	VerifiedOn       time.Time       `json:"verified_on"`
	Created          time.Time       `json:"created"`
	Modified         time.Time       `json:"modified"`

	// Contacts may be supplied on create to atomically create the company and its
	// initial contacts; the first contact becomes the company's primary contact.
	Contacts []*Contact `json:"contacts,omitempty"`
}

func NewCompany(model *db.Company) *Company {
	return &Company{
		ID:               model.ID,
		LEI:              model.LEI.String,
		Domain:           model.Domain,
		Name:             model.Name,
		Website:          model.Website,
		Country:          model.Country,
		BusinessCategory: model.BusinessCategory.String,
		VASPCategories:   model.VASPCategories,
		IVMS101:          json.RawMessage(model.IVMS101.JSONB),
		VerifiedOn:       model.VerifiedOn.Time(),
		Created:          model.Created.Time(),
		Modified:         model.Modified.Time(),
	}
}

func (c *Company) Model() *db.Company {
	return &db.Company{
		LEI:              sql.NullString{String: c.LEI, Valid: c.LEI != ""},
		Domain:           c.Domain,
		Name:             c.Name,
		Website:          c.Website,
		Country:          c.Country,
		BusinessCategory: sql.NullString{String: c.BusinessCategory, Valid: c.BusinessCategory != ""},
		VASPCategories:   c.VASPCategories,
		IVMS101:          fields.NullJSONB{JSONB: fields.JSONB(c.IVMS101), Valid: len(c.IVMS101) > 0},
		VerifiedOn:       fields.Time(c.VerifiedOn),
	}
}

func (c *Company) Validate(method string) (err error) {
	if c.ID != uuid.Nil {
		err = validation.Error(err, validation.ReadOnly("id"))
	}

	if !c.Created.IsZero() {
		err = validation.Error(err, validation.ReadOnly("created"))
	}

	if !c.Modified.IsZero() {
		err = validation.Error(err, validation.ReadOnly("modified"))
	}

	if !c.VerifiedOn.IsZero() {
		err = validation.Error(err, validation.ReadOnly("verified_on"))
	}

	c.LEI = strings.TrimSpace(c.LEI)
	if c.LEI != "" {
		if _, perr := lei.Parse(c.LEI); perr != nil {
			err = validation.Error(err, validation.Invalid("lei", perr.Error()))
		}
	}

	c.Domain = strings.TrimSpace(c.Domain)
	if c.Domain == "" {
		err = validation.Error(err, validation.Required("domain"))
	}

	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		err = validation.Error(err, validation.Required("name"))
	}

	c.Website = strings.TrimSpace(c.Website)
	if c.Website == "" {
		err = validation.Error(err, validation.Required("website"))
	}

	c.Country = strings.TrimSpace(strings.ToUpper(c.Country))
	if c.Country == "" {
		err = validation.Error(err, validation.Required("country"))
	} else if _, perr := country.Alpha2(c.Country); perr != nil {
		err = validation.Error(err, validation.Invalid("country", "invalid ISO 3166-1 alpha-2 code"))
	}

	var verr error
	if c.BusinessCategory, verr = ValidateBusinessCategory(c.BusinessCategory); verr != nil {
		err = validation.Error(err, verr)
	}

	if c.VASPCategories, verr = ValidateVASPCategories(c.VASPCategories); verr != nil {
		err = validation.Error(err, verr)
	}

	// Change null or empty object to an empty IVMS101 record
	c.IVMS101 = normalizeJSON(c.IVMS101)
	if len(c.IVMS101) > 0 {
		var person *ivms101.LegalPerson
		if perr := json.Unmarshal(c.IVMS101, &person); perr != nil {
			err = validation.Error(err, validation.Invalid("ivms101", perr.Error()))
		} else {
			if perr := ivms101.Validate(person); perr != nil {
				err = validation.Error(err, validation.Invalid("ivms101", perr.Error()))
			}
		}
	}

	// Nested contacts are only accepted on create; updates must use the contacts resource.
	if method != http.MethodPost && len(c.Contacts) > 0 {
		err = validation.Error(err, validation.ReadOnly("contacts"))
	}

	// Validate nested contacts; company_id is assigned by the server on create.
	for i, contact := range c.Contacts {
		if contact == nil {
			continue
		}

		cerr := contact.Validate(method)
		if contact.CompanyID != uuid.Nil {
			cerr = validation.Error(cerr, validation.ReadOnly("company_id"))
		}

		if cerr != nil {
			switch child := cerr.(type) {
			case validation.Errors:
				for _, ferr := range child {
					err = validation.Error(err, ferr.SubfieldArray("contacts", i))
				}
			case *validation.FieldError:
				err = validation.Error(err, child.SubfieldArray("contacts", i))
			default:
				err = validation.Error(err, validation.Invalid("contacts", child.Error()).Index(i))
			}
		}
	}
	return err
}

// Clone returns a deep copy of the company with read-only fields cleared.
func (c *Company) Clone() *Company {
	return &Company{
		LEI:              c.LEI,
		Domain:           c.Domain,
		Name:             c.Name,
		Website:          c.Website,
		Country:          c.Country,
		BusinessCategory: c.BusinessCategory,
		VASPCategories:   c.VASPCategories,
		IVMS101:          c.IVMS101,
	}
}
