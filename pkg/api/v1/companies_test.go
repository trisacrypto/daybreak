package api_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trisacrypto/daybreak/pkg/api/v1"
	"go.rtnl.ai/x/validation"
)

// A syntactically valid LEI with a correct Mod97 checksum (from the ivms101 testdata).
const testLEI = "283100RWUNF913WOJT02"

// A minimal IVMS101 legal person that passes ivms101.Validate.
const testIVMS101 = `{"name": {"nameIdentifier": [{"legalPersonName": "Bob's Discount VASP, PLC", "legalPersonNameIdentifierType": "LEGL"}]}, "countryOfRegistration": "GB"}`

// mockCompany returns a fully valid company request payload.
func mockCompany() *api.Company {
	return &api.Company{
		LEI:              testLEI,
		Domain:           "bobsvasp.co.uk",
		Name:             "Bob's Discount VASP",
		Website:          "https://bobsvasp.co.uk",
		Country:          "GB",
		BusinessCategory: api.BusinessCategoryBusiness,
		VASPCategories:   []string{api.VASPCategoryExchange, api.VASPCategoryCustodian},
		IVMS101:          json.RawMessage(testIVMS101),
	}
}

func TestCompanyValidate(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		tests := []struct {
			name   string
			modify func(c *api.Company)
		}{
			{"Complete", func(c *api.Company) {}},
			{"NoLEI", func(c *api.Company) { c.LEI = "" }},
			{"NilVASPCategories", func(c *api.Company) { c.VASPCategories = nil }},
			{"EmptyVASPCategories", func(c *api.Company) { c.VASPCategories = []string{} }},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				for _, method := range []string{"POST", "PUT", ""} {
					company := mockCompany()
					tc.modify(company)
					require.NoError(t, company.Validate(method), "expected no error for method %q", method)
					require.NotNil(t, company.VASPCategories, "expected non-nil vasp categories after validation")
				}
			})
		}
	})

	t.Run("OptionalIVMS101", func(t *testing.T) {
		// IVMS101 is optional: nil, empty, JSON null, and an empty object are all
		// accepted and normalized to nil so the model stores a NULL record.
		tests := []struct {
			name    string
			ivms101 json.RawMessage
		}{
			{"Nil", nil},
			{"Empty", json.RawMessage(``)},
			{"Null", json.RawMessage(`null`)},
			{"EmptyObject", json.RawMessage(`{}`)},
			{"EmptyObjectWithWhitespace", json.RawMessage("{ }\n")},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				for _, method := range []string{"POST", "PUT", ""} {
					company := mockCompany()
					company.IVMS101 = tc.ivms101
					require.NoError(t, company.Validate(method), "expected no error for method %q", method)
					require.Nil(t, company.IVMS101, "expected empty ivms101 to be normalized to nil")
				}
			})
		}
	})

	t.Run("Normalization", func(t *testing.T) {
		company := &api.Company{
			LEI:              "  " + testLEI + "\n",
			Domain:           "\tbobsvasp.co.uk  ",
			Name:             "  Bob's Discount VASP ",
			Website:          " https://bobsvasp.co.uk\t",
			Country:          " gb ",
			BusinessCategory: "  business_entity ",
			VASPCategories:   []string{" exchange", "", "CUSTODIAN ", "   "},
			IVMS101:          json.RawMessage(testIVMS101),
		}

		require.NoError(t, company.Validate("POST"))
		require.Equal(t, testLEI, company.LEI)
		require.Equal(t, "bobsvasp.co.uk", company.Domain)
		require.Equal(t, "Bob's Discount VASP", company.Name)
		require.Equal(t, "https://bobsvasp.co.uk", company.Website)
		require.Equal(t, "GB", company.Country)
		require.Equal(t, api.BusinessCategoryBusiness, company.BusinessCategory)
		require.Equal(t, []string{api.VASPCategoryExchange, api.VASPCategoryCustodian}, company.VASPCategories)
		require.JSONEq(t, testIVMS101, string(company.IVMS101), "ivms101 payload should not be modified")
	})

	t.Run("Invalid", func(t *testing.T) {
		tests := []struct {
			name   string
			modify func(c *api.Company)
			errs   []*validation.FieldError
		}{
			// Read-only fields
			{
				"ReadOnlyID",
				func(c *api.Company) { c.ID = uuid.New() },
				[]*validation.FieldError{validation.ReadOnly("id")},
			},
			{
				"ReadOnlyCreated",
				func(c *api.Company) { c.Created = time.Now() },
				[]*validation.FieldError{validation.ReadOnly("created")},
			},
			{
				"ReadOnlyModified",
				func(c *api.Company) { c.Modified = time.Now() },
				[]*validation.FieldError{validation.ReadOnly("modified")},
			},
			{
				"ReadOnlyVerifiedOn",
				func(c *api.Company) { c.VerifiedOn = time.Now() },
				[]*validation.FieldError{validation.ReadOnly("verified_on")},
			},
			// LEI is optional but must be valid when present
			{
				"LEIWrongLength",
				func(c *api.Company) { c.LEI = "ABC" },
				[]*validation.FieldError{validation.Invalid("lei", "invalid length: 3, expected 20")},
			},
			{
				"LEIBadChecksum",
				func(c *api.Company) { c.LEI = "283100RWUNF913WOJT03" },
				[]*validation.FieldError{validation.Invalid("lei", "invalid checksum")},
			},
			{
				"LEILowercase",
				func(c *api.Company) { c.LEI = "283100rwunf913wojt02" },
				[]*validation.FieldError{validation.Invalid("lei", "invalid character at position 6: r")},
			},
			// Required string fields
			{
				"MissingDomain",
				func(c *api.Company) { c.Domain = "" },
				[]*validation.FieldError{validation.Required("domain")},
			},
			{
				"WhitespaceDomain",
				func(c *api.Company) { c.Domain = "   " },
				[]*validation.FieldError{validation.Required("domain")},
			},
			{
				"MissingName",
				func(c *api.Company) { c.Name = "" },
				[]*validation.FieldError{validation.Required("name")},
			},
			{
				"WhitespaceName",
				func(c *api.Company) { c.Name = "\t\n" },
				[]*validation.FieldError{validation.Required("name")},
			},
			{
				"MissingWebsite",
				func(c *api.Company) { c.Website = "" },
				[]*validation.FieldError{validation.Required("website")},
			},
			{
				"WhitespaceWebsite",
				func(c *api.Company) { c.Website = " " },
				[]*validation.FieldError{validation.Required("website")},
			},
			// Country
			{
				"MissingCountry",
				func(c *api.Company) { c.Country = "" },
				[]*validation.FieldError{validation.Required("country")},
			},
			{
				"WhitespaceCountry",
				func(c *api.Company) { c.Country = "  " },
				[]*validation.FieldError{validation.Required("country")},
			},
			{
				"CountryAlpha3",
				func(c *api.Company) { c.Country = "GBR" },
				[]*validation.FieldError{validation.Invalid("country", "invalid ISO 3166-1 alpha-2 code")},
			},
			{
				"CountryNonAlpha",
				func(c *api.Company) { c.Country = "G1" },
				[]*validation.FieldError{validation.Invalid("country", "invalid ISO 3166-1 alpha-2 code")},
			},
			{
				"CountryUnassigned",
				func(c *api.Company) { c.Country = "XX" },
				[]*validation.FieldError{validation.Invalid("country", "invalid ISO 3166-1 alpha-2 code")},
			},
			// Business category is required and must be a known value
			{
				"MissingBusinessCategory",
				func(c *api.Company) { c.BusinessCategory = "" },
				[]*validation.FieldError{validation.Invalid("business_category", `"" is not a valid business category`)},
			},
			{
				"UnknownBusinessCategory",
				func(c *api.Company) { c.BusinessCategory = "foo" },
				[]*validation.FieldError{validation.Invalid("business_category", `"FOO" is not a valid business category`)},
			},
			// VASP categories
			{
				"UnknownVASPCategory",
				func(c *api.Company) { c.VASPCategories = []string{"foo"} },
				[]*validation.FieldError{validation.Invalid("vasp_categories", `"FOO" is not a valid VASP category`).Index(0)},
			},
			{
				"MultipleUnknownVASPCategories",
				func(c *api.Company) { c.VASPCategories = []string{"Exchange", "foo", "", "bar"} },
				[]*validation.FieldError{
					validation.Invalid("vasp_categories", `"FOO" is not a valid VASP category`).Index(1),
					validation.Invalid("vasp_categories", `"BAR" is not a valid VASP category`).Index(3),
				},
			},
			// IVMS101 is optional but must be a valid legal person when present
			{
				"MalformedIVMS101",
				func(c *api.Company) { c.IVMS101 = json.RawMessage(`not json`) },
				[]*validation.FieldError{validation.Invalid("ivms101", "invalid character 'o' in literal null (expecting 'u')")},
			},
			{
				"IVMS101MissingName",
				func(c *api.Company) { c.IVMS101 = json.RawMessage(`{"countryOfRegistration": "GB"}`) },
				[]*validation.FieldError{validation.Invalid("ivms101", "ivms101: missing name: this field is required")},
			},
			{
				"IVMS101NoLegalName",
				func(c *api.Company) {
					c.IVMS101 = json.RawMessage(`{"name": {"nameIdentifier": [{"legalPersonName": "Bob VASP", "legalPersonNameIdentifierType": "SHRT"}]}}`)
				},
				[]*validation.FieldError{validation.Invalid("ivms101", "ivms101: invalid field name.name: at least one name identifier must have a LEGL name identifier type")},
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				company := mockCompany()
				tc.modify(company)

				err := company.Validate("POST")
				require.Error(t, err, "expected a validation error")

				var verr validation.Errors
				require.True(t, errors.As(err, &verr), "expected validation.Errors")
				require.Len(t, verr, len(tc.errs), "unexpected number of validation errors: %s", err)
				for i, expected := range tc.errs {
					require.True(t, expected.Equal(verr[i]), "error %d: expected %q got %q", i, expected.Error(), verr[i].Error())
				}
			})
		}
	})

	t.Run("MultipleErrors", func(t *testing.T) {
		// A zero-valued company reports every required field in declaration order
		// (ivms101 is optional and does not appear).
		company := &api.Company{}
		err := company.Validate("POST")
		require.Error(t, err)

		var verr validation.Errors
		require.True(t, errors.As(err, &verr), "expected validation.Errors")

		expected := validation.Errors{
			validation.Required("domain"),
			validation.Required("name"),
			validation.Required("website"),
			validation.Required("country"),
			validation.Invalid("business_category", `"" is not a valid business category`),
		}
		require.Len(t, verr, len(expected), "unexpected number of validation errors: %s", err)
		for i := range expected {
			require.True(t, expected[i].Equal(verr[i]), "error %d: expected %q got %q", i, expected[i].Error(), verr[i].Error())
		}

		// Every check is independent: all errors are collected rather than short-circuiting.
		company = &api.Company{
			ID:               uuid.New(),
			LEI:              "ABC",
			Country:          "GBR",
			BusinessCategory: "foo",
			VASPCategories:   []string{"bar"},
			IVMS101:          json.RawMessage(`{"countryOfRegistration": "GB"}`),
			VerifiedOn:       time.Now(),
			Created:          time.Now(),
			Modified:         time.Now(),
		}
		err = company.Validate("PUT")
		require.Error(t, err)
		require.True(t, errors.As(err, &verr), "expected validation.Errors")

		expected = validation.Errors{
			validation.ReadOnly("id"),
			validation.ReadOnly("created"),
			validation.ReadOnly("modified"),
			validation.ReadOnly("verified_on"),
			validation.Invalid("lei", "invalid length: 3, expected 20"),
			validation.Required("domain"),
			validation.Required("name"),
			validation.Required("website"),
			validation.Invalid("country", "invalid ISO 3166-1 alpha-2 code"),
			validation.Invalid("business_category", `"FOO" is not a valid business category`),
			validation.Invalid("vasp_categories", `"BAR" is not a valid VASP category`).Index(0),
			validation.Invalid("ivms101", "ivms101: missing name: this field is required"),
		}
		require.Len(t, verr, len(expected), "unexpected number of validation errors: %s", err)
		for i := range expected {
			require.True(t, expected[i].Equal(verr[i]), "error %d: expected %q got %q", i, expected[i].Error(), verr[i].Error())
		}
	})
}
