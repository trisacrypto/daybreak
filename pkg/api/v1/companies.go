package api

import (
	"database/sql"
	"encoding/json"
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
		IVMS101:          fields.NullJSONB{JSONB: fields.JSONB(c.IVMS101), Valid: c.IVMS101 != nil},
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

	var person *ivms101.LegalPerson
	if perr := json.Unmarshal(c.IVMS101, &person); perr != nil {
		err = validation.Error(err, validation.Invalid("ivms101", err.Error()))
	} else {
		if perr := ivms101.Validate(person); perr != nil {
			err = validation.Error(err, validation.Invalid("ivms101", perr.Error()))
		}
	}

	return err
}
