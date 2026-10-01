package handlers

import (
	"context"
	"log/slog"
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
	// generation counts invalidations. A snapshot whose build began before the latest one may
	// hold what that invalidation removed, so SetIfCurrent does not store it.
	generation uint64
}

type overviewCacheEntry struct {
	data     []byte
	cachedAt time.Time
	// until, when set, ends the entry before its TTL: the moment a room it may show expires.
	until time.Time
}

const (
	// overviewCacheTTL is the server-side cache lifetime. The response itself
	// is sent with Cache-Control: public, no-cache (roomContentCacheControl).
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
	if time.Since(e.cachedAt) > c.ttl || (!e.until.IsZero() && !time.Now().Before(e.until)) {
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
	c.SetUntil(window, data, time.Time{})
}

// SetUntil is Set for a payload that must not be served at or after until (zero: TTL only).
func (c *OverviewCache) SetUntil(window string, data []byte, until time.Time) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.storeLocked(window, data, until)
}

// storeLocked stores the entry, evicting the oldest when full. c.mu must be held for writing.
func (c *OverviewCache) storeLocked(window string, data []byte, until time.Time) {
	key := overviewCacheKey(window)
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
	c.entries[key] = &overviewCacheEntry{data: out, cachedAt: time.Now(), until: until}
}

// Invalidate drops every cache entry. Called when the public overview changes (a room
// going private, archived or deleted; a listed post removed or edited, migration 000123),
// so the public overview can never serve a stale preview of what just left it.
func (c *OverviewCache) Invalidate() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*overviewCacheEntry)
	c.generation++
}

// Generation returns the invalidation count. Read it before building a snapshot and hand it
// to SetIfCurrent.
func (c *OverviewCache) Generation() uint64 {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.generation
}

// SetIfCurrent is SetUntil for a snapshot whose build began at generation: it is stored only
// when no invalidation came since, otherwise a change committed during the build (and already
// announced) would be served for the rest of the TTL. Reports whether it stored.
func (c *OverviewCache) SetIfCurrent(window string, data []byte, until time.Time, generation uint64) bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.generation != generation {
		return false
	}
	c.storeLocked(window, data, until)
	return true
}

// snapshotDeadline is when an overview read now may no longer be served from the cache: the
// next moment a public room expires (zero when none will). Rooms expire by the clock and no
// notice announces it, so without this cap an expired room could stay on the homepage for up
// to the TTL after its own routes answer 404. ok is false when the deadline cannot be read;
// the snapshot is then not cached at all.
func (h *HomepageOverviewHandler) snapshotDeadline(ctx context.Context) (time.Time, bool) {
	next, err := h.homeRepo.NextPublicRoomExpiry(ctx)
	if err != nil {
		slog.Error("homepage overview: next room expiry failed; snapshot not cached", "error", err)
		return time.Time{}, false
	}
	if next == nil {
		return time.Time{}, true
	}
	return *next, true
}
