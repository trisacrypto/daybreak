package server

import (
	"net/http"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/trisacrypto/daybreak/pkg"
	"github.com/trisacrypto/daybreak/pkg/web"
	"go.rtnl.ai/gimlet/htmx"
	"go.rtnl.ai/gimlet/logger"
)

// Sets up the server's middleware and routes.
func (s *Server) setupRoutes() (err error) {
	// Setup HTML template renderer.
	// NOTE: this mut be run before any routes are created to support cached static
	// HTML templates that are rendered from configuration or in memory data.
	if s.router.HTMLRender, err = web.NewRender(web.Templates()); err != nil {
		return err
	}

	// Setup the session store middleware for authentication.
	var store sessions.Store
	if store, err = s.db.Sessions(s.conf.Auth.SessionKeyPairs()...); err != nil {
		return err
	}

	// Application Middleware
	// NOTE: ordering is important to how middleware is handled
	middlewares := []gin.HandlerFunc{
		// Panic recovery middleware
		gin.Recovery(),

		// Optional logging middleware
		logger.Logger(pkg.ServiceName, pkg.Version(false)),

		// Return unavailable if the server is in maintenance mode
		s.Maintenance(),

		// CORS configuration allows the front-end to make cross-origin requests
		cors.New(s.conf.CORS()),

		// Session store middleware
		sessions.Sessions("daybreak", store),
	}

	// Kubernetes liveness probes added before middleware.
	s.router.GET("/healthz", gin.WrapF(s.Healthz))
	s.router.GET("/livez", gin.WrapF(s.Healthz))
	s.router.GET("/readyz", gin.WrapF(s.Readyz))

	// Add the middleware to the router
	for _, middleware := range middlewares {
		if middleware != nil {
			s.router.Use(middleware)
		}
	}

	// NotFound and NotAllowed routes
	s.router.NoRoute(s.NotFound)
	s.router.NoMethod(s.NotAllowed)

	// Error routes
	s.router.GET("/not-found", s.NotFound)
	s.router.GET("/not-allowed", s.NotAllowed)
	s.router.GET("/error", s.InternalError)

	// Static Files (if enabled)
	if s.conf.Static.Serve {
		s.router.StaticFS(s.conf.Static.URL, http.FS(os.DirFS(s.conf.Static.Root)))
	}

	// Authentication routes
	s.router.GET("/login", AcceptHTML, Wrap("ui.login", s.LoginPage))
	auth := s.router.Group("/auth")
	{
		auth.GET("/login", Wrap("ui.auth.login", s.AuthLogin))
		auth.GET("/logout", Wrap("ui.auth.logout", s.AuthLogout))
		auth.GET("/callback", Wrap("ui.auth.callback", s.AuthCallback))
	}

	// Web UI routes (authenticated)
	uia := s.router.Group("", AcceptHTML, LoginRequired)
	{
		uia.GET("/", Wrap("ui.dashboard", s.DashboardPage))
		uia.GET("/about", Wrap("ui.about", s.AboutPage))
		uia.GET("/companies", Wrap("ui.companies.list", s.CompanyDashboardPage))
		uia.GET("/companies/:id", Wrap("ui.company.detail", s.CompanyDetailPage))
		uia.GET("/companies/:id/edit", Wrap("ui.company.edit", s.CompanyEditPage))
		uia.GET("/licenses", Wrap("ui.licenses.list", s.LicenseDashboardPage))
		uia.GET("/licenses/:id", Wrap("ui.licenses.detail", s.LicenseDetailPage))
		uia.GET("/licenses/:id/edit", Wrap("ui.licenses.edit", s.LicenseEditPage))
		uia.GET("/licenses/:id/credentials", Wrap("ui.licenses.credentials", s.DownloadLicense))
	}

	// API routes (unauthenticated)
	v1o := s.router.Group("/v1")
	{
		v1o.GET("/status", s.Status)
	}

	// Authenticated API Routes
	v1a := s.router.Group("/v1", LoginRequired)
	{
		// Companies Resource
		v1a.GET("/companies", Wrap("api.companies.list", s.ListCompanies))
		v1a.POST("/companies", Wrap("api.companies.create", s.CreateCompany))
		v1a.GET("/companies/:id", Wrap("api.companies.retrieve", s.RetrieveCompany))
		v1a.PUT("/companies/:id", Wrap("api.companies.update", s.UpdateCompany))
		v1a.DELETE("/companies/:id", Wrap("api.companies.delete", s.DeleteCompany))

		// Nested Company Contacts Helper Actions
		v1a.GET("/companies/:id/contacts", Wrap("api.companies.contacts.list", s.ListCompanyContacts))
		v1a.POST("/companies/:id/contacts", Wrap("api.companies.contacts.create", s.CreateCompanyContact))

		// Contacts Resource
		v1a.GET("/contacts", Wrap("api.contacts.list", s.ListContacts))
		v1a.POST("/contacts", Wrap("api.contacts.create", s.CreateContact))
		v1a.GET("/contacts/:id", Wrap("api.contacts.retrieve", s.RetrieveContact))
		v1a.PUT("/contacts/:id", Wrap("api.contacts.update", s.UpdateContact))
		v1a.DELETE("/contacts/:id", Wrap("api.contacts.delete", s.DeleteContact))

		// Licenses Resource
		v1a.GET("/licenses", Wrap("api.licenses.list", s.ListLicenses))
		v1a.POST("/licenses", Wrap("api.licenses.create", s.CreateLicense))
		v1a.GET("/licenses/:id", Wrap("api.licenses.retrieve", s.RetrieveLicense))
		v1a.GET("/licenses/:id/credentials", Wrap("api.licenses.credentials", s.LicenseCredentials))
		v1a.PUT("/licenses/:id", Wrap("api.licenses.update", s.UpdateLicense))
		v1a.DELETE("/licenses/:id", Wrap("api.licenses.delete", s.DeleteLicense))

		// Informational endpoints and statistics
		v1a.GET("/countries", Wrap("api.countries.counts", s.CountryCounts))

		// HTMX only endpoints for the dashboard
		dash := v1a.Group("/dashboard", htmx.Only)
		{
			dash.GET("/info", Wrap("api.dashboard.info", s.DashboardInfo))
			dash.GET("/counterparties", Wrap("api.dashboard.counterparties", s.DashboardCounterparties))
			dash.GET("/licenses", Wrap("api.dashboard.licenses", s.DashboardLicenses))
		}
	}

	return nil
}
