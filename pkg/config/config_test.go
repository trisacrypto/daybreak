package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/trisacrypto/daybreak/pkg/config"
	"go.rtnl.ai/confire/contest"
	"go.rtnl.ai/x/rlog"
)

// The values in the test environment variables should result a configuration that is
// equal to the validConfig variable when processed. Anytime a configuration value is
// added to this package, it should be added to this map and have a non-default value
// for testing and validation purposes.
var testEnv = contest.Env{
	"DAYBREAK_MAINTENANCE":         "true",
	"DAYBREAK_MODE":                "debug",
	"DAYBREAK_LOG_LEVEL":           "debug",
	"DAYBREAK_CONSOLE_LOG":         "true",
	"DAYBREAK_BIND_ADDR":           ":8888",
	"DAYBREAK_ORIGIN":              "http://localhost:8888",
	"DAYBREAK_ALLOW_ORIGINS":       "http://localhost:8888",
	"DAYBREAK_DATABASE_URL":        "postgres://localhost:6543/accounts?sslmode=disable",
	"DAYBREAK_READ_HEADER_TIMEOUT": "8s",
	"DAYBREAK_WRITE_TIMEOUT":       "16s",
	"DAYBREAK_IDLE_TIMEOUT":        "32s",
	"DAYBREAK_SHUTDOWN_TIMEOUT":    "64s",
	"DAYBREAK_STATIC_SERVE":        "false",
	"DAYBREAK_STATIC_ROOT":         "../web/static",
	"DAYBREAK_STATIC_URL":          "https://example.com/static",
}

// This config should always pass validation and should match the testEnv.
var validConfig = config.Config{
	Maintenance:       true,
	Mode:              gin.DebugMode,
	LogLevel:          rlog.LevelDecoder(rlog.LevelDebug),
	ConsoleLog:        true,
	BindAddr:          ":8888",
	Origin:            "http://localhost:8888",
	AllowOrigins:      []string{"http://localhost:8888"},
	DatabaseURL:       "postgres://localhost:6543/accounts?sslmode=disable",
	ReadHeaderTimeout: 8 * time.Second,
	WriteTimeout:      16 * time.Second,
	IdleTimeout:       32 * time.Second,
	ShutdownTimeout:   64 * time.Second,
	Static: config.StaticConfig{
		Serve: false,
		Root:  "../web/static",
		URL:   "https://example.com/static",
	},
}

func TestConfig(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		t.Cleanup(testEnv.Set())

		conf, err := config.New()
		require.NoError(t, err, "could not process config from environment")
		require.Equal(t, validConfig, *conf, "valid config should be equal to the expected valid config")
	})

}

func TestDefaultConfig(t *testing.T) {
	// Remove any environment variables that may be set
	t.Cleanup(testEnv.Clear())

	// Required environment variables should be set
	os.Setenv("DAYBREAK_STATIC_ROOT", filepath.Join(os.Getenv("GOPATH"), "src", "github.com", "trisacrypto", "daybreak", "pkg", "web", "static"))

	// Ensure the default config is valid
	conf, err := config.New()
	require.NoError(t, err, "could not process config from environment")
	require.NotEmpty(t, conf, "processed config should not be empty")
}
