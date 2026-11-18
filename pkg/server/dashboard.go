package server

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/trisacrypto/daybreak/pkg/api/v1"
	"github.com/trisacrypto/daybreak/pkg/db"
	"github.com/trisacrypto/daybreak/pkg/errors"
	"github.com/trisacrypto/daybreak/pkg/web/scene"
)

// HTMX only endpoint that renders a partial of the info card on the dashboard.
func (s *Server) DashboardInfo(ctx context.Context, c *gin.Context) *errors.HTTP {
	var (
		err  error
		info *db.DashboardInfo
	)

	if info, err = s.db.DashboardInfo(ctx); err != nil {
		return api.Error(http.StatusInternalServerError, err, "could not retrieve dashboard info")
	}

	c.HTML(http.StatusOK, "partials/dashboard/info.html", info)
	return nil
}

// JSON endpoint that returns the country counts for the dashboard choropleth map.
func (s *Server) CountryCounts(ctx context.Context, c *gin.Context) *errors.HTTP {
	var (
		err    error
		counts []*db.CountryCount
		out    []*api.CountryCount
	)

	if counts, err = s.db.CountryCounts(ctx); err != nil {
		return api.Error(http.StatusInternalServerError, err, "could not retrieve country counts")
	}

	out = api.NewCountryCounts(counts)
	c.JSON(http.StatusOK, out)
	return nil
}

// HTMX only endpoint that renders a partial of the recent counterparties card on the dashboard.
func (s *Server) DashboardCounterparties(ctx context.Context, c *gin.Context) *errors.HTTP {
	var (
		err       error
		companies []*db.Company
		out       []*api.Company
	)

	if companies, err = s.db.RecentCompanies(ctx, 10); err != nil {
		return api.Error(http.StatusInternalServerError, err, "could not retrieve recent companies")
	}

	out = make([]*api.Company, 0, len(companies))
	for _, company := range companies {
		out = append(out, api.NewCompany(company))
	}

	c.HTML(http.StatusOK, "partials/companies/recent.html", scene.New(c).WithAPIData(out))
	return nil
}

// HTMX only endpoint that renders a partial of the recent licenses card on the dashboard.
func (s *Server) DashboardLicenses(ctx context.Context, c *gin.Context) *errors.HTTP {
	var (
		err      error
		licenses []*db.License
		out      []*api.License
	)

	if licenses, err = s.db.RecentLicenses(ctx, 10); err != nil {
		return api.Error(http.StatusInternalServerError, err, "could not retrieve recent licenses")
	}

	out = make([]*api.License, 0, len(licenses))
	for _, license := range licenses {
		out = append(out, api.NewLicense(license))
	}

	c.HTML(http.StatusOK, "partials/licenses/recent.html", scene.New(c).WithAPIData(out))
	return nil
}
