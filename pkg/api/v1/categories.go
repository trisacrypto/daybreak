package api

import (
	"fmt"
	"strings"

	"go.rtnl.ai/x/validation"
)

// Business Categories
const (
	BusinessCategoryUnknown       = "UNKNOWN_ENTITY"
	BusinessCategoryPrivate       = "PRIVATE_ORGANIZATION"
	BusinessCategoryGovernment    = "GOVERNMENT_ENTITY"
	BusinessCategoryBusiness      = "BUSINESS_ENTITY"
	BusinessCategoryNonCommercial = "NON_COMMERCIAL_ENTITY"
)

// VASP Categories
const (
	VASPCategoryUnknown    = "Unknown"
	VASPCategoryExchange   = "Exchange"
	VASPCategoryDEX        = "DEX"
	VASPCategoryP2P        = "P2P"
	VASPCategoryKiosk      = "Kiosk"
	VASPCategoryCustodian  = "Custodian"
	VASPCategoryOTC        = "OTC"
	VASPCategoryFund       = "Fund"
	VASPCategoryProject    = "Project"
	VASPCategoryGambling   = "Gambling"
	VASPCategoryMiner      = "Miner"
	VASPCategoryMixer      = "Mixer"
	VASPCategoryIndividual = "Individual"
	VASPCategoryOther      = "Other"
)

var (
	businessCategories map[string]struct{}
	vaspCategories     map[string]string
)

func init() {
	businessCategories = map[string]struct{}{
		BusinessCategoryUnknown:       {},
		BusinessCategoryPrivate:       {},
		BusinessCategoryGovernment:    {},
		BusinessCategoryBusiness:      {},
		BusinessCategoryNonCommercial: {},
	}
	vaspCategories = map[string]string{
		strings.ToUpper(VASPCategoryUnknown):    VASPCategoryUnknown,
		strings.ToUpper(VASPCategoryExchange):   VASPCategoryExchange,
		strings.ToUpper(VASPCategoryDEX):        VASPCategoryDEX,
		strings.ToUpper(VASPCategoryP2P):        VASPCategoryP2P,
		strings.ToUpper(VASPCategoryKiosk):      VASPCategoryKiosk,
		strings.ToUpper(VASPCategoryCustodian):  VASPCategoryCustodian,
		strings.ToUpper(VASPCategoryOTC):        VASPCategoryOTC,
		strings.ToUpper(VASPCategoryFund):       VASPCategoryFund,
		strings.ToUpper(VASPCategoryProject):    VASPCategoryProject,
		strings.ToUpper(VASPCategoryGambling):   VASPCategoryGambling,
		strings.ToUpper(VASPCategoryMiner):      VASPCategoryMiner,
		strings.ToUpper(VASPCategoryMixer):      VASPCategoryMixer,
		strings.ToUpper(VASPCategoryIndividual): VASPCategoryIndividual,
		strings.ToUpper(VASPCategoryOther):      VASPCategoryOther,
	}
}

func ValidateBusinessCategory(category string) (_ string, _ error) {
	category = strings.ToUpper(strings.TrimSpace(category))
	if _, ok := businessCategories[category]; !ok {
		return "", validation.Invalid("business_category", fmt.Sprintf("%q is not a valid business category", category))
	}
	return category, nil
}

func ValidateVASPCategories(categories []string) (_ []string, err error) {
	validated := make([]string, 0, len(categories))
	for i, category := range categories {
		category = strings.ToUpper(strings.TrimSpace(category))
		if category == "" {
			continue
		}

		if valid, ok := vaspCategories[category]; !ok {
			err = validation.Error(err,
				validation.Invalid(
					"vasp_categories", fmt.Sprintf("%q is not a valid VASP category", category),
				).Index(i),
			)
		} else {
			validated = append(validated, valid)
		}
	}

	return validated, err
}
