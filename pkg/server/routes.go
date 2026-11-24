package server

import (
	"net/http"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/trisacrypto/daybreak/pkg"
	"github.com/trisacrypto/daybreak/pkg/auth"
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
	aui := s.router.Group("/auth")
	{
		aui.GET("/login", Wrap("ui.auth.login", s.AuthLogin))
		aui.GET("/logout", Wrap("ui.auth.logout", s.AuthLogout))
		aui.GET("/callback", Wrap("ui.auth.callback", s.AuthCallback))
	}

	// Web UI routes (authenticated)
	uia := s.router.Group("", AcceptHTML, auth.LoginRequired)
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
	v1a := s.router.Group("/v1", auth.Authenticate(s))
	{
		// Sync and Download endpoints
		v1a.GET("/sync", auth.EnvoyUser, Wrap("api.daybreak.sync", s.DaybreakSync))
		v1a.GET("/download", auth.EnvoyDownload, Wrap("api.daybreak.download", s.DaybreakDownload))

		// Companies Resource
		v1a.GET("/companies", auth.EnvoyUser, Wrap("api.companies.list", s.ListCompanies))
		v1a.POST("/companies", auth.DaybreakContributor, Wrap("api.companies.create", s.CreateCompany))
		v1a.GET("/companies/:id", auth.EnvoyUser, Wrap("api.companies.detail", s.CompanyDetail))
		v1a.PUT("/companies/:id", auth.DaybreakEditor, Wrap("api.companies.update", s.UpdateCompany))
		v1a.DELETE("/companies/:id", auth.DaybreakEditor, Wrap("api.companies.delete", s.DeleteCompany))

		// Nested Company Contacts Helper Actions
		v1a.GET("/companies/:id/contacts", auth.EnvoyUser, Wrap("api.companies.contacts.list", s.ListCompanyContacts))
		v1a.POST("/companies/:id/contacts", auth.DaybreakContributor, Wrap("api.companies.contacts.create", s.CreateCompanyContact))

		// Contacts Resource
		v1a.GET("/contacts", auth.EnvoyUser, Wrap("api.contacts.list", s.ListContacts))
		v1a.POST("/contacts", auth.DaybreakEditor, Wrap("api.contacts.create", s.CreateContact))
		v1a.GET("/contacts/:id", auth.EnvoyUser, Wrap("api.contacts.detail", s.ContactDetail))
		v1a.PUT("/contacts/:id", auth.DaybreakEditor, Wrap("api.contacts.update", s.UpdateContact))
		v1a.DELETE("/contacts/:id", auth.DaybreakEditor, Wrap("api.contacts.delete", s.DeleteContact))

		// Licenses Resource
		v1a.GET("/licenses", auth.DaybreakLicensor, Wrap("api.licenses.list", s.ListLicenses))
		v1a.POST("/licenses", auth.DaybreakLicensor, Wrap("api.licenses.create", s.CreateLicense))
		v1a.GET("/licenses/:id", auth.DaybreakLicensor, Wrap("api.licenses.detail", s.LicenseDetail))
		v1a.GET("/licenses/:id/credentials", auth.DaybreakLicensor, Wrap("api.licenses.credentials", s.LicenseCredentials))
		v1a.PUT("/licenses/:id", auth.DaybreakLicensor, Wrap("api.licenses.update", s.UpdateLicense))
		v1a.DELETE("/licenses/:id", auth.DaybreakLicensor, Wrap("api.licenses.delete", s.DeleteLicense))

		// Informational endpoints and statistics
		v1a.GET("/countries", auth.EnvoyUser, Wrap("api.countries.counts", s.CountryCounts))

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
