package handlers

import (
	"sync"
	"time"
)

// overviewCacheInvalidator is a package-level hook that the room write path
// calls when a room's visibility changes or it is moderated. It is wired to the
// OverviewCache at router setup time. Using a function variable here avoids
// threading the cache through every handler constructor.
var overviewCacheInvalidator func()

// SetOverviewCacheInvalidator registers the cache invalidation callback so the
// room update path can notify it of visibility/moderation changes.
func SetOverviewCacheInvalidator(fn func()) {
	overviewCacheInvalidator = fn
}

// InvalidateOverviewCache fires the registered invalidation callback. Called
// from room write paths; a nil callback (no cache configured) is a no-op.
func InvalidateOverviewCache() {
	if overviewCacheInvalidator != nil {
		overviewCacheInvalidator()
	}
}

// OverviewCache is a bounded in-process cache for the consolidated overview
// payload. The homepage is the most-visited page in Solvr, and every request
// reads across five repository tables; a 30-second cache means one database
// round trip per visitor batch, not one per visitor.
//
// Cache keys include the window parameter, so ?window=24h, ?window=7d and
// ?window=30d are cached independently. The cache is invalidated on any
// visibility or moderation change that could alter the public preview or
// activity surface — see Invalidate.
type OverviewCache struct {
	mu         sync.RWMutex
	ttl        time.Duration
	entries    map[string]*overviewCacheEntry
	maxEntries int
}

type overviewCacheEntry struct {
	data     []byte
	cachedAt time.Time
}

const (
	// overviewCacheTTL is the server-side cache lifetime, matching the
	// Cache-Control: public, max-age=30 header on the response.
	overviewCacheTTL = 30 * time.Second

	// overviewCacheMaxEntries bounds the cache so it cannot grow unbounded
	// if a visitor enumerates arbitrary window values.
	overviewCacheMaxEntries = 6
)

// NewOverviewCache returns a fresh bounded cache with a 30-second TTL.
func NewOverviewCache() *OverviewCache {
	return &OverviewCache{
		ttl:        overviewCacheTTL,
		entries:    make(map[string]*overviewCacheEntry),
		maxEntries: overviewCacheMaxEntries,
	}
}

// cacheKey builds a cache key from the request path and window parameter.
func overviewCacheKey(window string) string {
	if window == "" {
		window = "24h"
	}
	return "/v1/overview?window=" + window
}

// Get returns the cached payload and true when the entry is fresh.
func (c *OverviewCache) Get(window string) ([]byte, bool) {
	if c == nil {
		return nil, false
	}
	key := overviewCacheKey(window)
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if time.Since(e.cachedAt) > c.ttl {
		return nil, false
	}
	// Return a copy so a caller mutating the slice cannot poison the cache.
	out := make([]byte, len(e.data))
	copy(out, e.data)
	return out, true
}

// Set stores a payload under the window-scoped key, evicting the oldest entry
// when the cache is full.
func (c *OverviewCache) Set(window string, data []byte) {
	if c == nil {
		return
	}
	key := overviewCacheKey(window)
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= c.maxEntries {
		// Evict the oldest entry to bound memory.
		var oldestKey string
		var oldestTime time.Time
		for k, e := range c.entries {
			if oldestTime.IsZero() || e.cachedAt.Before(oldestTime) {
				oldestKey, oldestTime = k, e.cachedAt
			}
		}
		delete(c.entries, oldestKey)
	}
	out := make([]byte, len(data))
	copy(out, data)
	c.entries[key] = &overviewCacheEntry{data: out, cachedAt: time.Now()}
}

// Invalidate drops every cache entry. Called from the room write path when a
// room's visibility changes or it is moderated, so the public overview can never
// serve a stale preview of a room that just went private.
func (c *OverviewCache) Invalidate() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*overviewCacheEntry)
}
