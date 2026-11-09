package traffic

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// A page view's source key (SPEC.md 27.6, Release A of the 2026-11-08 SEO audit) tells one
// visitor's views of one day apart from another's without storing who they are: the same
// address and user agent give the same key within a UTC day, a different one the next day,
// and no address gives no key at all.
func TestPageViewSourceKey(t *testing.T) {
	day := time.Date(2026, 11, 8, 13, 12, 0, 0, time.UTC)
	later := day.Add(10 * time.Hour)
	nextDay := day.Add(24 * time.Hour)
	secret := []byte("web-report-secret")

	a := PageViewSourceKey(secret, day, "203.0.113.7", "Mozilla/5.0 Chrome")
	assert.Len(t, a, 16)
	assert.Regexp(t, `^[0-9a-f]{16}$`, a)
	assert.Equal(t, a, PageViewSourceKey(secret, later, "203.0.113.7", "Mozilla/5.0 Chrome"), "same source, same day")
	assert.NotEqual(t, a, PageViewSourceKey(secret, nextDay, "203.0.113.7", "Mozilla/5.0 Chrome"), "the key changes with the day")
	assert.NotEqual(t, a, PageViewSourceKey(secret, day, "203.0.113.8", "Mozilla/5.0 Chrome"), "another address")
	assert.NotEqual(t, a, PageViewSourceKey(secret, day, "203.0.113.7", "curl/8"), "another user agent")
	assert.NotEqual(t, a, PageViewSourceKey([]byte("other-secret"), day, "203.0.113.7", "Mozilla/5.0 Chrome"), "another secret")
	assert.Equal(t, "", PageViewSourceKey(secret, day, "", "Mozilla/5.0 Chrome"), "no address, no key")
	assert.Equal(t, "", PageViewSourceKey(secret, day, "   ", "Mozilla/5.0 Chrome"), "a blank address, no key")
}

func TestReasons_NameTheBurstRules(t *testing.T) {
	assert.Contains(t, Reasons, ReasonPageViewBurst)
	assert.Contains(t, Reasons, ReasonPageViewBurstLegacy)
	assert.Equal(t, Reason("page_view_burst"), ReasonPageViewBurst)
	assert.Equal(t, Reason("page_view_burst_legacy"), ReasonPageViewBurstLegacy)
}
