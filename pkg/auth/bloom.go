package auth

import (
	"sync"
	"uuid"

	"github.com/bits-and-blooms/bloom/v3"
	"github.com/trisacrypto/daybreak/pkg/errors"
	"go.rtnl.ai/ulid"
)

const (
	RequestCacheSize = 2097152
	RequestCacheFPP  = 0.02
)

type RequestCache struct {
	sync.Mutex
	cache   *bloom.BloomFilter
	counter uint64
}

// Checks that the request ID is not in the cache to prevent reply attacks that use the
// same request ID multiple times. The request ID is both checked and added to the cache.
// The cache is reset before it gets full to prevent the FP rate from becoming too high.
// This check also validates the request ID is a valid ULID or UUID and caches the 16
// byte representation of the request ID.
func (c *RequestCache) Check(requestID string) (ok bool, err error) {
	var rid []byte
	if rid, err = DecodeRequestID(requestID); err != nil {
		return false, err
	}

	c.Lock()
	defer c.Unlock()

	if c.cache == nil || c.counter >= RequestCacheSize {
		c.cache = bloom.NewWithEstimates(RequestCacheSize, RequestCacheFPP)
		c.counter = 0
	}

	// TestAndAdd returns true if the request ID was already in the cache (a replay).
	ok = !c.cache.TestAndAdd(rid)
	c.counter++
	return ok, nil
}

func DecodeRequestID(requestID string) ([]byte, error) {
	if uu, err := uuid.Parse(requestID); err == nil {
		return uu[:], nil
	}

	if uu, err := ulid.ParseStrict(requestID); err == nil {
		return uu[:], nil
	}

	return nil, errors.ErrInvalidRequestID
}
