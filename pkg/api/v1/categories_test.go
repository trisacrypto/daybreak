package api_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trisacrypto/daybreak/pkg/api/v1"
	"go.rtnl.ai/x/validation"
)

func TestValidateBusinessCategory(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			// Canonical values
			{api.BusinessCategoryUnknown, api.BusinessCategoryUnknown},
			{api.BusinessCategoryPrivate, api.BusinessCategoryPrivate},
			{api.BusinessCategoryGovernment, api.BusinessCategoryGovernment},
			{api.BusinessCategoryBusiness, api.BusinessCategoryBusiness},
			{api.BusinessCategoryNonCommercial, api.BusinessCategoryNonCommercial},
			// Case insensitive
			{"unknown_entity", api.BusinessCategoryUnknown},
			{"private_organization", api.BusinessCategoryPrivate},
			{"Government_Entity", api.BusinessCategoryGovernment},
			{"business_ENTITY", api.BusinessCategoryBusiness},
			{"non_commercial_entity", api.BusinessCategoryNonCommercial},
			// Whitespace is trimmed
			{"  BUSINESS_ENTITY  ", api.BusinessCategoryBusiness},
			{"\tprivate_organization\n", api.BusinessCategoryPrivate},
		}

		for i, tc := range tests {
			actual, err := api.ValidateBusinessCategory(tc.input)
			require.NoError(t, err, "test case %d: expected no error for %q", i, tc.input)
			require.Equal(t, tc.expected, actual, "test case %d: unexpected normalized category for %q", i, tc.input)
		}
	})

	t.Run("Invalid", func(t *testing.T) {
		tests := []struct {
			input    string
			expected *validation.FieldError
		}{
			{"", validation.Invalid("business_category", `"" is not a valid business category`)},
			{"   ", validation.Invalid("business_category", `"" is not a valid business category`)},
			{"foo", validation.Invalid("business_category", `"FOO" is not a valid business category`)},
			{"BUSINESS", validation.Invalid("business_category", `"BUSINESS" is not a valid business category`)},
			{"BUSINESS ENTITY", validation.Invalid("business_category", `"BUSINESS ENTITY" is not a valid business category`)},
			{"BUSINESS_ENTITY_", validation.Invalid("business_category", `"BUSINESS_ENTITY_" is not a valid business category`)},
			// VASP categories are not business categories
			{api.VASPCategoryExchange, validation.Invalid("business_category", `"EXCHANGE" is not a valid business category`)},
		}

		for i, tc := range tests {
			actual, err := api.ValidateBusinessCategory(tc.input)
			require.Empty(t, actual, "test case %d: expected empty category for %q", i, tc.input)
			require.Error(t, err, "test case %d: expected error for %q", i, tc.input)

			var ferr *validation.FieldError
			require.True(t, errors.As(err, &ferr), "test case %d: expected a validation.FieldError for %q", i, tc.input)
			require.Equal(t, "business_category", ferr.Field(), "test case %d: unexpected field for %q", i, tc.input)
			require.True(t, tc.expected.Equal(ferr), "test case %d: expected %q got %q", i, tc.expected.Error(), ferr.Error())
		}
	})
}

func TestValidateVASPCategories(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		tests := []struct {
			input    []string
			expected []string
		}{
			// Nil and empty inputs produce an empty (non-nil) slice
			{nil, []string{}},
			{[]string{}, []string{}},
			// All canonical values
			{
				[]string{
					api.VASPCategoryUnknown, api.VASPCategoryExchange, api.VASPCategoryDEX,
					api.VASPCategoryP2P, api.VASPCategoryKiosk, api.VASPCategoryCustodian,
					api.VASPCategoryOTC, api.VASPCategoryFund, api.VASPCategoryProject,
					api.VASPCategoryGambling, api.VASPCategoryMiner, api.VASPCategoryMixer,
					api.VASPCategoryIndividual, api.VASPCategoryOther,
				},
				[]string{
					api.VASPCategoryUnknown, api.VASPCategoryExchange, api.VASPCategoryDEX,
					api.VASPCategoryP2P, api.VASPCategoryKiosk, api.VASPCategoryCustodian,
					api.VASPCategoryOTC, api.VASPCategoryFund, api.VASPCategoryProject,
					api.VASPCategoryGambling, api.VASPCategoryMiner, api.VASPCategoryMixer,
					api.VASPCategoryIndividual, api.VASPCategoryOther,
				},
			},
			// Case insensitive, normalized to canonical casing
			{[]string{"exchange", "DEX", "dex", "p2p", "CUSTODIAN", "oTc"}, []string{"Exchange", "DEX", "DEX", "P2P", "Custodian", "OTC"}},
			// Whitespace is trimmed
			{[]string{"  Exchange  ", "\tMiner\n"}, []string{"Exchange", "Miner"}},
			// Empty and whitespace-only entries are skipped
			{[]string{"", "Exchange", "   ", "Fund", ""}, []string{"Exchange", "Fund"}},
			{[]string{"", "   "}, []string{}},
			// Duplicates are preserved and order is maintained
			{[]string{"Other", "Exchange", "Other"}, []string{"Other", "Exchange", "Other"}},
		}

		for i, tc := range tests {
			actual, err := api.ValidateVASPCategories(tc.input)
			require.NoError(t, err, "test case %d: expected no error for %v", i, tc.input)
			require.NotNil(t, actual, "test case %d: expected non-nil slice for %v", i, tc.input)
			require.Equal(t, tc.expected, actual, "test case %d: unexpected validated categories for %v", i, tc.input)
		}
	})

	t.Run("Invalid", func(t *testing.T) {
		tests := []struct {
			input    []string
			expected []string
			errs     []*validation.FieldError
		}{
			{
				[]string{"foo"},
				[]string{},
				[]*validation.FieldError{
					validation.Invalid("vasp_categories", `"FOO" is not a valid VASP category`).Index(0),
				},
			},
			// Index reflects the position in the original input, including skipped empties
			{
				[]string{"", "Exchange", "bar"},
				[]string{"Exchange"},
				[]*validation.FieldError{
					validation.Invalid("vasp_categories", `"BAR" is not a valid VASP category`).Index(2),
				},
			},
			// Multiple invalid entries are all reported; valid ones are still returned
			{
				[]string{"foo", "Miner", "Exchange Service", "   ", "DEX", "mixer!"},
				[]string{"Miner", "DEX"},
				[]*validation.FieldError{
					validation.Invalid("vasp_categories", `"FOO" is not a valid VASP category`).Index(0),
					validation.Invalid("vasp_categories", `"EXCHANGE SERVICE" is not a valid VASP category`).Index(2),
					validation.Invalid("vasp_categories", `"MIXER!" is not a valid VASP category`).Index(5),
				},
			},
			// Business categories are not VASP categories
			{
				[]string{api.BusinessCategoryBusiness},
				[]string{},
				[]*validation.FieldError{
					validation.Invalid("vasp_categories", `"BUSINESS_ENTITY" is not a valid VASP category`).Index(0),
				},
			},
		}

		for i, tc := range tests {
			actual, err := api.ValidateVASPCategories(tc.input)
			require.Equal(t, tc.expected, actual, "test case %d: unexpected validated categories for %v", i, tc.input)
			require.Error(t, err, "test case %d: expected error for %v", i, tc.input)

			var verr validation.Errors
			require.True(t, errors.As(err, &verr), "test case %d: expected validation.Errors for %v", i, tc.input)
			require.Len(t, verr, len(tc.errs), "test case %d: unexpected number of validation errors for %v", i, tc.input)
			for j, expected := range tc.errs {
				require.True(t, expected.Equal(verr[j]), "test case %d: error %d: expected %q got %q", i, j, expected.Error(), verr[j].Error())
			}
		}
	})
}
