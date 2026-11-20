package auth_test

import (
	"encoding/binary"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"uuid"

	"github.com/stretchr/testify/require"
	"github.com/trisacrypto/daybreak/pkg/auth"
	"github.com/trisacrypto/daybreak/pkg/errors"
	"go.rtnl.ai/ulid"
)

// The same 16 byte request ID encoded as a UUID and as a ULID.
// cSpell:ignore MYSCFMZ UUIDURN
const (
	requestUUID = "018f4e6a-7b2c-7d3e-9f40-a1b2c3d4e5f6"
	requestULID = "01HX76MYSCFMZ9YG51PB1X9SFP"
)

var requestBytes = []byte{
	0x01, 0x8f, 0x4e, 0x6a, 0x7b, 0x2c, 0x7d, 0x3e,
	0x9f, 0x40, 0xa1, 0xb2, 0xc3, 0xd4, 0xe5, 0xf6,
}

// Request IDs are derived from an index instead of being randomly generated so that
// the contents of the bloom filter, and therefore the test results, are reproducible.
func rawID(i uint64) (id [16]byte) {
	binary.BigEndian.PutUint64(id[8:], i)
	return id
}

func uuidFor(i uint64) string { return uuid.UUID(rawID(i)).String() }
func ulidFor(i uint64) string { return ulid.ULID(rawID(i)).String() }

// Runs fn in the specified number of goroutines, holding them at a starting gate so
// that they contend with each other as much as possible. Assertions must be made by
// the caller after this function returns, not inside of fn.
func concurrently(workers int, fn func(worker int)) {
	var wg sync.WaitGroup
	start := make(chan struct{})

	for w := range workers {
		wg.Go(func() {
			<-start
			fn(w)
		})
	}

	close(start)
	wg.Wait()
}

func TestDecodeRequestID(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		tests := []struct {
			name      string
			requestID string
		}{
			{"UUID", requestUUID},
			{"UUIDUpperCase", strings.ToUpper(requestUUID)},
			{"UUIDBraces", "{" + requestUUID + "}"},
			{"UUIDURN", "urn:uuid:" + requestUUID},
			{"UUIDNoHyphens", strings.ReplaceAll(requestUUID, "-", "")},
			{"ULID", requestULID},
			{"ULIDLowerCase", strings.ToLower(requestULID)},
		}

		// Every encoding must decode to the same bytes, otherwise a request could be
		// replayed by changing how its ID is encoded.
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				rid, err := auth.DecodeRequestID(tc.requestID)
				require.NoError(t, err)
				require.Equal(t, requestBytes, rid)
			})
		}
	})

	t.Run("Invalid", func(t *testing.T) {
		tests := []struct {
			name      string
			requestID string
		}{
			{"Whitespace", " "},
			{"Text", "not-a-request-id"},
			{"Number", "12345"},
			{"UUIDLeadingSpace", " " + requestUUID},
			{"UUIDTrailingSpace", requestUUID + " "},
			{"UUIDTruncated", requestUUID[:len(requestUUID)-1]},
			{"UUIDTooLong", requestUUID + "0"},
			{"UUIDNotHex", strings.Repeat("g", 32)},
			{"UUIDBadSeparator", strings.Replace(requestUUID, "-", "_", 1)},
			{"ULIDTrailingSpace", requestULID + " "},
			{"ULIDTruncated", requestULID[:len(requestULID)-1]},
			{"ULIDTooLong", requestULID + "0"},
			// The first character of a ULID cannot be greater than 7 or it overflows 128 bits.
			{"ULIDOverflow", "8" + requestULID[1:]},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				rid, err := auth.DecodeRequestID(tc.requestID)
				require.ErrorIs(t, err, errors.ErrInvalidRequestID)
				require.Nil(t, rid)
			})
		}
	})
}

func TestRequestCache(t *testing.T) {
	t.Run("Unseen", func(t *testing.T) {
		// The zero value of the cache must be usable without any initialization.
		cache := &auth.RequestCache{}

		for i := range uint64(128) {
			requestID := uuidFor(i)
			if i%2 == 1 {
				requestID = ulidFor(i)
			}

			ok, err := cache.Check(requestID)
			require.NoError(t, err)
			require.True(t, ok, "expected unseen request ID %q to be accepted", requestID)
		}
	})

	t.Run("Replay", func(t *testing.T) {
		cache := &auth.RequestCache{}

		ok, err := cache.Check(requestUUID)
		require.NoError(t, err)
		require.True(t, ok, "expected the first use of the request ID to be accepted")

		for range 3 {
			ok, err = cache.Check(requestUUID)
			require.NoError(t, err)
			require.False(t, ok, "expected the replayed request ID to be rejected")
		}

		// A replay must not prevent other request IDs from being accepted.
		ok, err = cache.Check(uuidFor(1))
		require.NoError(t, err)
		require.True(t, ok, "expected an unseen request ID to be accepted after a replay")
	})

	t.Run("ReplayEncoding", func(t *testing.T) {
		tests := []struct {
			name   string
			first  string
			replay string
		}{
			{"UUIDUpperCase", requestUUID, strings.ToUpper(requestUUID)},
			{"UUIDBraces", requestUUID, "{" + requestUUID + "}"},
			{"UUIDURN", requestUUID, "urn:uuid:" + requestUUID},
			{"UUIDNoHyphens", requestUUID, strings.ReplaceAll(requestUUID, "-", "")},
			{"ULIDLowerCase", requestULID, strings.ToLower(requestULID)},
			{"UUIDAsULID", requestUUID, requestULID},
			{"ULIDAsUUID", requestULID, requestUUID},
		}

		// The decoded bytes are cached, so a replay is caught however the ID is encoded.
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				cache := &auth.RequestCache{}

				ok, err := cache.Check(tc.first)
				require.NoError(t, err)
				require.True(t, ok, "expected the first use of the request ID to be accepted")

				ok, err = cache.Check(tc.replay)
				require.NoError(t, err)
				require.False(t, ok, "expected %q to be rejected as a replay of %q", tc.replay, tc.first)
			})
		}
	})

	t.Run("Invalid", func(t *testing.T) {
		cache := &auth.RequestCache{}

		for _, requestID := range []string{" ", "not-a-request-id", requestUUID + "0", requestULID + "0"} {
			ok, err := cache.Check(requestID)
			require.ErrorIs(t, err, errors.ErrInvalidRequestID, "expected %q to be invalid", requestID)
			require.False(t, ok, "expected invalid request ID %q to be rejected", requestID)
		}

		// Invalid request IDs must not prevent valid request IDs from being accepted.
		ok, err := cache.Check(requestUUID)
		require.NoError(t, err)
		require.True(t, ok, "expected a valid request ID to be accepted after invalid ones")
	})

	t.Run("Independent", func(t *testing.T) {
		alpha, bravo := &auth.RequestCache{}, &auth.RequestCache{}

		ok, err := alpha.Check(requestUUID)
		require.NoError(t, err)
		require.True(t, ok)

		ok, err = bravo.Check(requestUUID)
		require.NoError(t, err)
		require.True(t, ok, "expected caches to not share request IDs")
	})
}

func TestRequestCacheReset(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test that fills the request cache to capacity in short mode")
	}

	cache := &auth.RequestCache{}
	check := func(requestID string) bool {
		ok, err := cache.Check(requestID)
		require.NoError(t, err)
		return ok
	}

	first := uuidFor(0)
	require.True(t, check(first), "expected the first use of the request ID to be accepted")

	// Fill the cache with unseen request IDs until there is room for one more check.
	// These should all be accepted, but as the bloom filter fills up some are rejected
	// as false positives; over the life of the cache this must stay below the FPP.
	var rejected int
	for i := uint64(1); i < auth.RequestCacheSize-1; i++ {
		if !check(uuidFor(i)) {
			rejected++
		}
	}

	fpr := float64(rejected) / float64(auth.RequestCacheSize)
	require.LessOrEqual(t, fpr, auth.RequestCacheFPP, "%d unseen request IDs were rejected while filling the cache", rejected)

	// The cache must not forget request IDs before it reaches capacity.
	require.False(t, check(first), "expected a replay to be rejected when the cache is at capacity")

	// The cache is now at capacity so the next check resets it. The reset must be safe
	// when concurrent requests race to trigger it: every request below has an unseen
	// ID so all of them must be accepted no matter which one performs the reset.
	const workers = 32
	var accepted, failed atomic.Int64

	concurrently(workers, func(w int) {
		ok, err := cache.Check(uuidFor(auth.RequestCacheSize + uint64(w)))
		switch {
		case err != nil:
			failed.Add(1)
		case ok:
			accepted.Add(1)
		}
	})

	require.Zero(t, failed.Load(), "expected no errors checking request IDs during a reset")
	require.Equal(t, int64(workers), accepted.Load(), "expected all unseen request IDs to be accepted during a reset")

	// The reset forgets all previous request IDs but tracks new ones.
	require.True(t, check(first), "expected the cache to be reset after reaching capacity")
	require.False(t, check(first), "expected a replay to be rejected after the cache is reset")
}

func TestRequestCacheConcurrency(t *testing.T) {
	const (
		workers  = 32
		requests = 256
	)

	// Workers alternate between UUIDs and ULIDs to exercise both decoding paths.
	requestID := func(worker int, i uint64) string {
		if worker%2 == 1 {
			return ulidFor(i)
		}
		return uuidFor(i)
	}

	t.Run("Unseen", func(t *testing.T) {
		// Every worker checks its own unseen request IDs so all of them must be
		// accepted. The cache starts uninitialized, so the workers also race to
		// initialize it on the first check.
		cache := &auth.RequestCache{}
		var accepted, failed atomic.Int64

		concurrently(workers, func(w int) {
			for i := range uint64(requests) {
				ok, err := cache.Check(requestID(w, uint64(w)*requests+i))
				switch {
				case err != nil:
					failed.Add(1)
				case ok:
					accepted.Add(1)
				}
			}
		})

		require.Zero(t, failed.Load(), "expected no errors checking request IDs")
		require.Equal(t, int64(workers*requests), accepted.Load(), "expected all unseen request IDs to be accepted")
	})

	t.Run("Replay", func(t *testing.T) {
		// Every worker checks the same request IDs in the same order, so each request
		// ID must be accepted for exactly one worker: the check and the add have to be
		// atomic or two concurrent requests with the same ID could both be accepted.
		cache := &auth.RequestCache{}
		var failed atomic.Int64
		accepted := make([]atomic.Int64, requests)

		concurrently(workers, func(w int) {
			for i := range uint64(requests) {
				ok, err := cache.Check(requestID(w, i))
				switch {
				case err != nil:
					failed.Add(1)
				case ok:
					accepted[i].Add(1)
				}
			}
		})

		require.Zero(t, failed.Load(), "expected no errors checking request IDs")
		for i := range accepted {
			require.Equal(t, int64(1), accepted[i].Load(), "expected request ID %d to be accepted exactly once out of %d concurrent checks", i, workers)
		}
	})
}
