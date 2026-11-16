package server

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/trisacrypto/daybreak/pkg/api/v1"
	"github.com/trisacrypto/daybreak/pkg/db"
	"github.com/trisacrypto/daybreak/pkg/errors"
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
