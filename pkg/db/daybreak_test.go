package db_test

import (
	"context"
	"time"
)

func (s *Suite) TestDaybreak() {
	require := s.Require()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// There should be no modified timestamp until it is set.
	modified, err := s.db.LastModified(ctx)
	require.NoError(err)
	require.True(modified.IsZero())

	// Set the modified timestamp.
	err = s.db.UpdateModified(ctx)
	require.NoError(err)

	// The modified timestamp should be set.
	modified, err = s.db.LastModified(ctx)
	require.NoError(err)
	require.False(modified.IsZero())
}
