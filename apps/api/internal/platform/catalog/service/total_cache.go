package service

import (
	"sync"
	"time"

	"api/internal/platform/settings/keys"
)

const totalsMaxEntries = 8192

// Forum S2S traffic at ~40 rps re-ran identical count(*) per page request and saturated prod postgres on 2026-09-01.
type totalsCache struct {
	mu      sync.Mutex
	entries map[string]totalsEntry
	now     func() time.Time
}

type totalsEntry struct {
	val int64
	exp time.Time
}

func newTotalsCache() *totalsCache {
	return &totalsCache{
		entries: make(map[string]totalsEntry),
		now:     time.Now,
	}
}

func (c *totalsCache) get(key string) (int64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return 0, false
	}
	if !e.exp.After(c.now()) {
		delete(c.entries, key)
		return 0, false
	}
	return e.val, true
}

func (c *totalsCache) put(key string, v int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.entries[key]; !exists && len(c.entries) >= totalsMaxEntries {
		c.entries = make(map[string]totalsEntry)
	}
	c.entries[key] = totalsEntry{val: v, exp: c.now().Add(time.Duration(keys.CatalogTotalsCacheTTLSeconds.Get()) * time.Second)}
}

func (c *totalsCache) flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]totalsEntry)
}
