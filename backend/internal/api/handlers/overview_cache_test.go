package handlers

import (
	"testing"
	"time"
)

// A snapshot whose build began before an invalidation may hold what that invalidation removed
// (idx 77 slice 7: a post deleted while the overview was being built stayed on it for the rest
// of the 30 s TTL), so SetIfCurrent stores only a snapshot built at the current generation.
func TestOverviewCache_SetIfCurrentKeepsOutASnapshotBuiltBeforeAnInvalidation(t *testing.T) {
	c := NewOverviewCache()
	built := c.Generation()
	c.Invalidate()
	if c.SetIfCurrent("24h", []byte(`stale`), time.Time{}, built) {
		t.Fatal("a snapshot built before the invalidation was stored")
	}
	if got, ok := c.Get("24h"); ok {
		t.Fatalf("served %q, a snapshot built before the invalidation", got)
	}

	current := c.Generation()
	if current == built {
		t.Fatal("Invalidate did not move the generation")
	}
	if !c.SetIfCurrent("24h", []byte(`fresh`), time.Time{}, current) {
		t.Fatal("a snapshot built at the current generation was not stored")
	}
	if got, ok := c.Get("24h"); !ok || string(got) != "fresh" {
		t.Fatalf("Get = %q, %v; want the fresh snapshot", got, ok)
	}

	var none *OverviewCache
	if none.Generation() != 0 || none.SetIfCurrent("24h", []byte(`x`), time.Time{}, 0) {
		t.Fatal("a nil cache stores nothing")
	}
}
