package server

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trisacrypto/daybreak/pkg/api/v1"
	"github.com/trisacrypto/daybreak/pkg/db"
	"github.com/trisacrypto/daybreak/pkg/errors"
	"github.com/trisacrypto/daybreak/pkg/web/scene"
)

//============================================================================
// Application Pages
//============================================================================

// Dashboard is the root landing page for the server. HTML requests render the main
// dashboard page; API requests are rejected.
func (s *Server) DashboardPage(ctx context.Context, c *gin.Context) *errors.HTTP {
	data := scene.New(c).ForPage("dashboard")
	c.HTML(http.StatusOK, "dashboard/daybreak/index.html", data)
	return nil
}

func (s *Server) AboutPage(ctx context.Context, c *gin.Context) *errors.HTTP {
	data := scene.New(c).ForPage("about")
	data["Uptime"] = time.Since(s.started).String()

	healthy := s.IsHealthy()
	ready := s.IsReady()

	switch {
	case s.conf.Maintenance:
		data["Status"] = serverStatusMaintenance
	case healthy && ready:
		data["Status"] = serverStatusOK
	case healthy && !ready:
		data["Status"] = serverStatusNotReady
	case !healthy:
		data["Status"] = serverStatusUnhealthy
	}

	c.HTML(http.StatusOK, "pages/daybreak/about.html", data)
	return nil
}

//============================================================================
// Company Pages
//============================================================================

func (s *Server) CompanyDashboardPage(ctx context.Context, c *gin.Context) *errors.HTTP {
	data := scene.New(c).ForPage("companies:list")
	c.HTML(http.StatusOK, "dashboard/companies/list.html", data)
	return nil
}

func (s *Server) CompanyDetailPage(ctx context.Context, c *gin.Context) *errors.HTTP {
	return s.CompanyTemplate(ctx, c, "companies:detail", "pages/companies/detail.html")
}

func (s *Server) CompanyEditPage(ctx context.Context, c *gin.Context) *errors.HTTP {
	return s.CompanyTemplate(ctx, c, "companies:edit", "pages/companies/edit.html")
}

func (s *Server) CompanyTemplate(ctx context.Context, c *gin.Context, page, template string) *errors.HTTP {
	var (
		err       error
		companyID uuid.UUID
		model     *db.Company
		company   *api.Company
		out       scene.Scene
	)

	if companyID, err = uuid.Parse(c.Param("id")); err != nil {
		return api.Error(http.StatusNotFound, err, "invalid company ID")
	}

	if model, err = s.db.RetrieveCompany(ctx, companyID); err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			return api.Error(http.StatusNotFound, err, "company not found")
		}
		return api.Error(http.StatusInternalServerError, err, "failed to retrieve company")
	}

	company = api.NewCompany(model)
	out = scene.New(c).ForPage(page).WithAPIData(company)
	c.HTML(http.StatusOK, template, out)
	return nil
}

//============================================================================
// License Pages
//============================================================================

func (s *Server) LicenseDashboardPage(ctx context.Context, c *gin.Context) *errors.HTTP {
	data := scene.New(c).ForPage("licenses:list")
	c.HTML(http.StatusOK, "dashboard/licenses/list.html", data)
	return nil
}
