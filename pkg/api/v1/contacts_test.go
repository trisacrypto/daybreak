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

// A minimal IVMS101 natural person that passes ivms101.Validate.
const testNaturalPerson = `{"name": {"nameIdentifier": [{"primaryIdentifier": "Smith", "secondaryIdentifier": "Bob", "nameIdentifierType": "LEGL"}]}, "countryOfResidence": "GB"}`

// mockContact returns a fully valid contact request payload.
func mockContact() *api.Contact {
	return &api.Contact{
		Name:    "Bob Smith",
		Email:   "bob@bobsvasp.co.uk",
		Role:    "Compliance Officer",
		IVMS101: json.RawMessage(testNaturalPerson),
	}
}

func TestContactValidate(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		tests := []struct {
			name   string
			modify func(c *api.Contact)
		}{
			{"Complete", func(c *api.Contact) {}},
			{"NoRole", func(c *api.Contact) { c.Role = "" }},
			{"CompanyIDIsWritable", func(c *api.Contact) { c.CompanyID = uuid.New() }},
			{"EmailWithDisplayName", func(c *api.Contact) { c.Email = "Bob Smith <bob@bobsvasp.co.uk>" }},
			{"EmailSubaddress", func(c *api.Contact) { c.Email = "bob+compliance@bobsvasp.co.uk" }},
			{
				"IVMS101WithOptionalFields",
				func(c *api.Contact) {
					c.IVMS101 = json.RawMessage(`{
						"name": {"nameIdentifier": [{"primaryIdentifier": "Smith", "secondaryIdentifier": "Bob", "nameIdentifierType": "LEGL"}]},
						"geographicAddress": [{"addressType": "HOME", "addressLine": ["1 High Street", "London"], "country": "GB"}],
						"nationalIdentification": {"nationalIdentifier": "AB123456C", "nationalIdentifierType": "SOCS", "countryOfIssue": "GB"},
						"customerIdentification": "cust-0001",
						"dateAndPlaceOfBirth": {"dateOfBirth": "1980-01-02", "placeOfBirth": "London"},
						"countryOfResidence": "GB"
					}`)
				},
			},
			{"IVMS101UnknownCountryOfResidence", func(c *api.Contact) {
				c.IVMS101 = json.RawMessage(`{"name": {"nameIdentifier": [{"primaryIdentifier": "Smith", "nameIdentifierType": "LEGL"}]}, "countryOfResidence": "XX"}`)
			}},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				for _, method := range []string{"POST", "PUT", ""} {
					contact := mockContact()
					tc.modify(contact)
					require.NoError(t, contact.Validate(method), "expected no error for method %q", method)
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
					contact := mockContact()
					contact.IVMS101 = tc.ivms101
					require.NoError(t, contact.Validate(method), "expected no error for method %q", method)
					require.Nil(t, contact.IVMS101, "expected empty ivms101 to be normalized to nil")
				}
			})
		}
	})

	t.Run("Normalization", func(t *testing.T) {
		contact := &api.Contact{
			Name:    "  Bob Smith\n",
			Email:   "\tbob@bobsvasp.co.uk  ",
			Role:    " Compliance Officer\t",
			IVMS101: json.RawMessage(testNaturalPerson),
		}

		require.NoError(t, contact.Validate("POST"))
		require.Equal(t, "Bob Smith", contact.Name)
		require.Equal(t, "bob@bobsvasp.co.uk", contact.Email)
		require.Equal(t, "Compliance Officer", contact.Role)
		require.JSONEq(t, testNaturalPerson, string(contact.IVMS101), "ivms101 payload should not be modified")

		// Role is optional: whitespace-only role is normalized to the empty string.
		contact = mockContact()
		contact.Role = "   "
		require.NoError(t, contact.Validate("POST"))
		require.Equal(t, "", contact.Role)
	})

	t.Run("Invalid", func(t *testing.T) {
		tests := []struct {
			name   string
			modify func(c *api.Contact)
			errs   []*validation.FieldError
		}{
			// Read-only fields
			{
				"ReadOnlyID",
				func(c *api.Contact) { c.ID = uuid.New() },
				[]*validation.FieldError{validation.ReadOnly("id")},
			},
			{
				"ReadOnlyCreated",
				func(c *api.Contact) { c.Created = time.Now() },
				[]*validation.FieldError{validation.ReadOnly("created")},
			},
			{
				"ReadOnlyModified",
				func(c *api.Contact) { c.Modified = time.Now() },
				[]*validation.FieldError{validation.ReadOnly("modified")},
			},
			// Name is required
			{
				"MissingName",
				func(c *api.Contact) { c.Name = "" },
				[]*validation.FieldError{validation.Required("name")},
			},
			{
				"WhitespaceName",
				func(c *api.Contact) { c.Name = "\t\n " },
				[]*validation.FieldError{validation.Required("name")},
			},
			// Email is required and must parse as an RFC 5322 address
			{
				"MissingEmail",
				func(c *api.Contact) { c.Email = "" },
				[]*validation.FieldError{validation.Required("email")},
			},
			{
				"WhitespaceEmail",
				func(c *api.Contact) { c.Email = "   " },
				[]*validation.FieldError{validation.Required("email")},
			},
			{
				"EmailNoAtSign",
				func(c *api.Contact) { c.Email = "bobsvasp.co.uk" },
				[]*validation.FieldError{validation.Invalid("email", "mail: missing '@' or angle-addr")},
			},
			{
				"EmailNoDomain",
				func(c *api.Contact) { c.Email = "bob@" },
				[]*validation.FieldError{validation.Invalid("email", "mail: missing '@' or angle-addr")},
			},
			{
				"EmailNoLocalPart",
				func(c *api.Contact) { c.Email = "@bobsvasp.co.uk" },
				[]*validation.FieldError{validation.Invalid("email", "mail: missing word in phrase: mail: invalid string")},
			},
			{
				"EmailMultipleAddresses",
				func(c *api.Contact) { c.Email = "bob@bobsvasp.co.uk, alice@bobsvasp.co.uk" },
				[]*validation.FieldError{validation.Invalid("email", "mail: expected single address, got \", alice@bobsvasp.co.uk\"")},
			},
			{
				"EmailInternalWhitespace",
				func(c *api.Contact) { c.Email = "bob smith@bobsvasp.co.uk" },
				[]*validation.FieldError{validation.Invalid("email", "mail: no angle-addr")},
			},
			// IVMS101 is optional but must be a valid natural person when present
			{
				"MalformedIVMS101",
				func(c *api.Contact) { c.IVMS101 = json.RawMessage(`not json`) },
				[]*validation.FieldError{validation.Invalid("ivms101", "invalid character 'o' in literal null (expecting 'u')")},
			},
			{
				"IVMS101WrongJSONType",
				func(c *api.Contact) { c.IVMS101 = json.RawMessage(`[]`) },
				[]*validation.FieldError{validation.Invalid("ivms101", "json: cannot unmarshal array into Go value of type ivms101.serialNaturalPerson")},
			},
			{
				"IVMS101MissingName",
				func(c *api.Contact) { c.IVMS101 = json.RawMessage(`{"countryOfResidence": "GB"}`) },
				[]*validation.FieldError{validation.Invalid("ivms101", "ivms101: missing name: this field is required")},
			},
			{
				"IVMS101EmptyNameIdentifiers",
				func(c *api.Contact) { c.IVMS101 = json.RawMessage(`{"name": {"nameIdentifier": []}}`) },
				[]*validation.FieldError{validation.Invalid("ivms101", "ivms101: missing name.name: this field is required")},
			},
			{
				"IVMS101NoLegalName",
				func(c *api.Contact) {
					c.IVMS101 = json.RawMessage(`{"name": {"nameIdentifier": [{"primaryIdentifier": "Smith", "nameIdentifierType": "ALIA"}]}}`)
				},
				[]*validation.FieldError{validation.Invalid("ivms101", "ivms101: invalid field name.name: at least one name identifier must have a LEGL name identifier type")},
			},
			{
				"IVMS101MissingPrimaryIdentifier",
				func(c *api.Contact) {
					c.IVMS101 = json.RawMessage(`{"name": {"nameIdentifier": [{"secondaryIdentifier": "Bob", "nameIdentifierType": "LEGL"}]}}`)
				},
				[]*validation.FieldError{validation.Invalid("ivms101", "ivms101: missing name.nameIdentifier[0].primaryIdentifier: this field is required")},
			},
			{
				"IVMS101InvalidCountryOfResidence",
				func(c *api.Contact) {
					c.IVMS101 = json.RawMessage(`{"name": {"nameIdentifier": [{"primaryIdentifier": "Smith", "nameIdentifierType": "LEGL"}]}, "countryOfResidence": "GBR"}`)
				},
				[]*validation.FieldError{validation.Invalid("ivms101", "ivms101: invalid field countryOfResidence: iso3166: country alpha-2 codes must be 2 uppercase characters")},
			},
			{
				"IVMS101InvalidDateOfBirth",
				func(c *api.Contact) {
					c.IVMS101 = json.RawMessage(`{"name": {"nameIdentifier": [{"primaryIdentifier": "Smith", "nameIdentifierType": "LEGL"}]}, "dateAndPlaceOfBirth": {"dateOfBirth": "01/02/1980", "placeOfBirth": "London"}}`)
				},
				[]*validation.FieldError{validation.Invalid("ivms101", "ivms101: invalid field dateAndPlaceOfBirth.dateOfBirth: invalid date format")},
			},
			{
				"IVMS101RegistrationAuthorityNotAllowed",
				func(c *api.Contact) {
					c.IVMS101 = json.RawMessage(`{"name": {"nameIdentifier": [{"primaryIdentifier": "Smith", "nameIdentifierType": "LEGL"}]}, "nationalIdentification": {"nationalIdentifier": "AB123456C", "nationalIdentifierType": "SOCS", "countryOfIssue": "GB", "registrationAuthority": "RA000001"}}`)
				},
				[]*validation.FieldError{validation.Invalid("ivms101", "ivms101: invalid field nationalIdentification.registrationAuthority: no registration authority allowed for natural persons")},
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				contact := mockContact()
				tc.modify(contact)

				err := contact.Validate("POST")
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
		// A zero-valued contact reports every required field in declaration order
		// (ivms101 is optional and does not appear).
		contact := &api.Contact{}
		err := contact.Validate("POST")
		require.Error(t, err)

		var verr validation.Errors
		require.True(t, errors.As(err, &verr), "expected validation.Errors")

		expected := validation.Errors{
			validation.Required("name"),
			validation.Required("email"),
		}
		require.Len(t, verr, len(expected), "unexpected number of validation errors: %s", err)
		for i := range expected {
			require.True(t, expected[i].Equal(verr[i]), "error %d: expected %q got %q", i, expected[i].Error(), verr[i].Error())
		}

		// Every check is independent: all errors are collected rather than short-circuiting.
		contact = &api.Contact{
			ID:       uuid.New(),
			Email:    "not-an-email",
			IVMS101:  json.RawMessage(`{"countryOfResidence": "GB"}`),
			Created:  time.Now(),
			Modified: time.Now(),
		}
		err = contact.Validate("PUT")
		require.Error(t, err)
		require.True(t, errors.As(err, &verr), "expected validation.Errors")

		expected = validation.Errors{
			validation.ReadOnly("id"),
			validation.ReadOnly("created"),
			validation.ReadOnly("modified"),
			validation.Required("name"),
			validation.Invalid("email", "mail: missing '@' or angle-addr"),
			validation.Invalid("ivms101", "ivms101: missing name: this field is required"),
		}
		require.Len(t, verr, len(expected), "unexpected number of validation errors: %s", err)
		for i := range expected {
			require.True(t, expected[i].Equal(verr[i]), "error %d: expected %q got %q", i, expected[i].Error(), verr[i].Error())
		}
	})
}
