package scene

import (
	"maps"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/trisacrypto/daybreak/pkg"
	"github.com/trisacrypto/daybreak/pkg/api/v1"
)

var (
	// Compute the version of the package at runtime so it is static for all contexts.
	version      = pkg.Version(false)
	shortVersion = pkg.Version(true)
	revision     = pkg.GitVersion
	buildDate    = pkg.BuildDate
	logoutURL    = "#"
)

// Keys for default Scene context items
const (
	Version      = "Version"
	ShortVersion = "ShortVersion"
	Revision     = "Revision"
	BuildDate    = "BuildDate"
	Page         = "Page"
	User         = "User"
	APIData      = "APIData"
	Path         = "Path"
	LogoutURL    = "LogoutURL"
)

type Scene map[string]any

func New(c *gin.Context) Scene {
	if c == nil {
		return Scene{
			Version:      version,
			ShortVersion: shortVersion,
			Revision:     revision,
			BuildDate:    buildDate,
			LogoutURL:    logoutURL,
		}
	}

	// Create the basic context
	context := Scene{
		Version:      version,
		ShortVersion: shortVersion,
		Revision:     revision,
		BuildDate:    buildDate,
		LogoutURL:    logoutURL,
	}

	// Set path from the gin context
	context[Path] = c.Request.URL.Path

	// Add the user info to the context
	session := sessions.Default(c)
	context[User] = session.Get("user")

	return context
}

func (s Scene) Update(o Scene) Scene {
	maps.Copy(s, o)
	return s
}

func (s Scene) WithAPIData(data any) Scene {
	s[APIData] = data
	return s
}

func (s Scene) ForPage(page string) Scene {
	s[Page] = page
	return s
}

//============================================================================
// API Data Helpers
//============================================================================

func (s Scene) Companies() []*api.Company {
	if data, ok := s[APIData]; ok {
		if out, ok := data.([]*api.Company); ok {
			return out
		}
	}
	return nil
}
