package api

import (
	"github.com/trisacrypto/daybreak/pkg/db"
	"go.rtnl.ai/x/country"
)

type CountryCount struct {
	Code    string `json:"code"`
	Country string `json:"country"`
	Count   int64  `json:"count"`
}

func NewCountryCounts(counts []*db.CountryCount) []*CountryCount {
	out := make([]*CountryCount, 0, len(counts))
	for _, count := range counts {
		c := &CountryCount{
			Country: count.Country,
			Count:   count.Count,
		}

		if info, err := country.Alpha2(count.Country); err == nil {
			c.Code = info.Numeric
			out = append(out, c)
		}
	}
	return out
}
