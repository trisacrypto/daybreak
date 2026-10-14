package server

import (
	"context"
	"time"

	"github.com/trisacrypto/daybreak/pkg"
)

func (s *Suite) TestStatus() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require := s.Require()
	status, err := s.client.Status(ctx)
	require.NoError(err, "could not get status")
	require.Equal("ok", status.Status, "status should be ok")
	require.Equal(pkg.Version(false), status.Version, "version should be package version")
	require.NotEmpty(status.Uptime, "uptime should be set")
}
