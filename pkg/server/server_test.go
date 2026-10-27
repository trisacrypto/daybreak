package server

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/trisacrypto/daybreak/pkg/api/v1"
	"github.com/trisacrypto/daybreak/pkg/config"
	"github.com/trisacrypto/daybreak/pkg/db"
	"go.rtnl.ai/x/rlog"
)

func TestServer(t *testing.T) {
	suite.Run(t, new(Suite))
}

// This test suite is used to test the server functionality. It sets up a sqlite
// database for and runs the server on a live local port using an http test TLS server.
// API requests can then be made with the client on the suite to execute requests
// against the server for testing purposes. Note that the database is not reset after
// each test, you must call ResetDB() to reset the database after each test.
type Suite struct {
	suite.Suite
	server *Server
	client api.Client
	conf   config.Config
	db     string
}

func (s *Suite) SetupSuite() {
	require := s.Require()
	s.db = s.T().TempDir() + "/daybreak_test.db"
	s.conf = config.Config{
		Mode:              "test",
		Maintenance:       false,
		LogLevel:          rlog.LevelDecoder(rlog.LevelError),
		ConsoleLog:        false,
		BindAddr:          "127.0.0.1:0",
		Origin:            "http://localhost",
		AllowOrigins:      []string{"http://localhost"},
		DatabaseURL:       "sqlite3:///" + s.db,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      2 * time.Second,
		IdleTimeout:       2 * time.Second,
		ShutdownTimeout:   10 * time.Second,
		Static: config.StaticConfig{
			Serve: true,
			Root:  "pkg/web/static",
			URL:   "/static",
		},
		Auth: config.AuthConfig{
			SessionKey: "ZYAPVCp/BMoFvrJL+fEmDpmWevm318AHuP1bXC3Ml34=",
		},
	}

	err := config.Set(s.conf)
	require.NoError(err, "could not set config")

	s.server, err = New()
	require.NoError(err, "could not create server")

	s.LoadFixtures()

	go s.server.Serve()
	time.Sleep(100 * time.Millisecond)

	s.client, err = api.New(s.server.URL().String())
	require.NoError(err, "could not create client")
}

func (s *Suite) TearDownSuite() {
	s.server.Shutdown()
	os.Remove(s.db)
}

func (s *Suite) ResetDB() {
	err := s.server.db.Close()
	s.Require().NoError(err, "could not close database")

	os.Remove(s.db)
	s.server.db, err = db.Open(s.conf.DatabaseDSN())
	s.Require().NoError(err, "could not open database")
	s.LoadFixtures()
}

// LoadFixtures applies every *.sql file in the testdata directory to the server's
// database in lexicographic filename order (e.g. 0001_companies.sql before
// 0002_contacts.sql). It is a no-op if the directory contains no SQL files.
func (s *Suite) LoadFixtures() {
	require := s.Require()

	paths, err := filepath.Glob(filepath.Join("testdata", "*.sql"))
	require.NoError(err, "could not list sql fixtures in testdata")
	if len(paths) == 0 {
		return
	}
	sort.Strings(paths)

	for _, path := range paths {
		data, err := os.ReadFile(path)
		require.NoError(err, "could not read fixture %s", path)

		_, err = s.server.db.Exec(string(data))
		require.NoError(err, "could not apply fixture %s", path)
	}
}
