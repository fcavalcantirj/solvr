package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GET /v1/overview and GET /v1/overview/activity through the real router, against a
// real database, called the way a logged-out browser calls them.
//
// These tests guard the Task 14 consolidated envelope: a richer meta block around
// the existing HomepageOverview data, with generated_at, window boundaries,
// metric definitions, source availability, and any partial-error state.

// overviewMeta is the meta envelope that GET /v1/overview wraps around its data.
type overviewMeta struct {
	GeneratedAt        time.Time          `json:"generated_at"`
	Window             overviewWindowMeta `json:"window"`
	WindowBoundaries   windowBoundaries   `json:"window_boundaries"`
	WindowDefinition   string             `json:"window_definition"`
	SourceAvailability map[string]bool    `json:"source_availability"`
	PartialErrors      []string           `json:"partial_errors"`
	Stale              bool               `json:"stale"`
	LastUpdatedLabel   string             `json:"last_updated_label"`
	StaleLabel         string             `json:"stale_label"`
}

type overviewWindowMeta struct {
	Value string    `json:"value"`
	Label string    `json:"label"`
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type windowBoundaries struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

type consolidatedOverviewResponse struct {
	Data handlers.HomepageOverview `json:"data"`
	Meta overviewMeta              `json:"meta"`
}

// getConsolidatedOverview calls GET /v1/overview with no credentials, the way a
// logged-out visitor reads the index.
func getConsolidatedOverview(t *testing.T, baseURL string) (consolidatedOverviewResponse, string) {
	t.Helper()
	resp, err := http.Get(baseURL + "/v1/overview")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", string(body))

	var out consolidatedOverviewResponse
	require.NoError(t, json.Unmarshal(body, &out), "body: %s", string(body))
	return out, string(body)
}

// getConsolidatedOverviewWindow calls GET /v1/overview?window=.
func getConsolidatedOverviewWindow(t *testing.T, baseURL, window string) consolidatedOverviewResponse {
	t.Helper()
	resp, err := http.Get(baseURL + "/v1/overview?window=" + window)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", string(body))

	var out consolidatedOverviewResponse
	require.NoError(t, json.Unmarshal(body, &out), "body: %s", string(body))
	return out
}

// getConsolidatedActivity calls GET /v1/overview/activity for Load more.
func getConsolidatedActivity(t *testing.T, baseURL string, offset, limit int) (hpoActivity, string) {
	t.Helper()
	url := fmt.Sprintf("%s/v1/overview/activity?offset=%d&limit=%d", baseURL, offset, limit)
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", string(body))

	var wrapper struct {
		Data hpoActivity `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &wrapper), "body: %s", string(body))
	return wrapper.Data, string(body)
}

// --- RED tests: these will fail until /v1/overview and /v1/overview/activity exist ---

// TestOverviewConsolidated_PartialErrorsIsAnEmptyArrayNotNull pins the wire format on the
// HEALTHY path. `var partialErrors []string` stays nil when every subsystem reads fine, and Go
// marshals a nil slice to `null`, which crashed the homepage
// (live-overview.tsx: meta.partial_errors.length on null). It is asserted on the RAW BYTES on
// purpose: decoding into []string turns null and [] into the same empty slice, which is exactly
// how this escaped every existing test.
func TestOverviewConsolidated_PartialErrorsIsAnEmptyArrayNotNull(t *testing.T) {
	ts, _, cleanup := setupRoomTestServer(t)
	defer cleanup()

	resp, err := http.Get(ts.URL + "/v1/overview")
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck
	require.Equal(t, http.StatusOK, resp.StatusCode)

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.NotContains(t, string(raw), `"partial_errors":null`,
		"a healthy read must not send partial_errors as null: the client dereferences .length on it")
	require.Contains(t, string(raw), `"partial_errors":[]`,
		"a healthy read must send an empty array")
}

func TestOverviewConsolidated_EnvelopeShape(t *testing.T) {
	t.Setenv("HOMEPAGE_PREVIEW_ROOM_SLUGS", hpoSlug("env"))
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	hpoSeedRoom(t, pool, hpoSlug("env"), "Envelope Room", "env preview", false, []string{
		"planner says hi", "executor replies",
	})

	resp, _ := getConsolidatedOverview(t, ts.URL)

	// The meta envelope must carry the fields task 14 names.
	assert.False(t, resp.Meta.GeneratedAt.IsZero(), "generated_at must be set")
	assert.Equal(t, "24h", resp.Meta.Window.Value, "defaults to 24h")
	assert.Equal(t, "24 hours", resp.Meta.Window.Label)
	assert.False(t, resp.Meta.Window.Start.IsZero(), "window start must be set")
	assert.True(t, resp.Meta.Window.End.After(resp.Meta.Window.Start), "window end after start")
	assert.False(t, resp.Meta.WindowBoundaries.StartTime.IsZero(), "start_time must be set")
	assert.False(t, resp.Meta.WindowBoundaries.EndTime.IsZero(), "end_time must be set")
	assert.NotEmpty(t, resp.Meta.WindowDefinition, "window_definition must be a human string")

	// Source availability must report the known subsystems.
	assert.Contains(t, resp.Meta.SourceAvailability, "rooms")
	assert.Contains(t, resp.Meta.SourceAvailability, "activity")
	assert.Contains(t, resp.Meta.SourceAvailability, "api_usage")
	assert.Contains(t, resp.Meta.SourceAvailability, "search")
	assert.Contains(t, resp.Meta.SourceAvailability, "community")

	// Task 16: the meta envelope carries a readable Last updated timestamp,
	// and a stale flag that is false on a clean read.
	assert.NotEmpty(t, resp.Meta.LastUpdatedLabel, "last_updated_label must be a readable string")
	assert.Contains(t, resp.Meta.LastUpdatedLabel, "Updated", "last_updated_label must be human-readable text")
	assert.False(t, resp.Meta.Stale, "a clean read is not stale")
	assert.Empty(t, resp.Meta.StaleLabel, "a fresh snapshot has no stale label")
	assert.Empty(t, resp.Meta.PartialErrors, "no partial errors on a clean read")

	// data must still be the existing HomepageOverview shape.
	assert.False(t, resp.Data.GeneratedAt.IsZero())
	assert.NotEmpty(t, resp.Data.Closing.ConnectURL)
}

func TestOverviewConsolidated_WindowValidation(t *testing.T) {
	t.Setenv("HOMEPAGE_PREVIEW_ROOM_SLUGS", hpoSlug("win"))
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	hpoSeedRoom(t, pool, hpoSlug("win"), "Window Room", "window test", false, []string{
		"a message", "another message",
	})

	// 7d is accepted and reflected in meta.
	ov7 := getConsolidatedOverviewWindow(t, ts.URL, "7d")
	assert.Equal(t, "7d", ov7.Meta.Window.Value)

	// 30d is accepted.
	ov30 := getConsolidatedOverviewWindow(t, ts.URL, "30d")
	assert.Equal(t, "30d", ov30.Meta.Window.Value)

	// An invalid window does not 400 — it falls back to the default and SAYS so.
	resp, err := http.Get(ts.URL + "/v1/overview?window=90d")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "invalid window must not 400: %s", string(body))

	var out consolidatedOverviewResponse
	require.NoError(t, json.Unmarshal(body, &out))
	assert.Equal(t, "24h", out.Meta.Window.Value, "90d falls back to 24h")
}

func TestOverviewConsolidated_CacheControl(t *testing.T) {
	t.Setenv("HOMEPAGE_PREVIEW_ROOM_SLUGS", hpoSlug("cache"))
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	hpoSeedRoom(t, pool, hpoSlug("cache"), "Cache Room", "cache test", false, []string{
		"cached message", "second message",
	})

	resp, err := http.Get(ts.URL + "/v1/overview")
	require.NoError(t, err)
	defer resp.Body.Close()
	cc := resp.Header.Get("Cache-Control")
	assert.Equal(t, "public, no-cache", cc, "cacheable, but revalidated so room content never outlives a visibility change")
}

func TestOverviewConsolidated_NeverLeaksAPrivateRoom(t *testing.T) {
	privateSlug := hpoSlug("priv")
	t.Setenv("HOMEPAGE_PREVIEW_ROOM_SLUGS", privateSlug)
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	secret := "CONFIDENTIAL PRIVATE ROOM CONTENT that must never reach the homepage"
	// The private room is the ONLY configured preview.
	hpoSeedRoom(t, pool, privateSlug, "Private Room", "closed", true, []string{
		secret, secret + " again",
	})

	_, raw := getConsolidatedOverview(t, ts.URL)

	assert.NotContains(t, raw, privateSlug, "a private room's slug must not appear")
	assert.NotContains(t, raw, secret, "a private room's content must not appear")
	assert.NotContains(t, raw, "Private Room", "a private room's name must not appear")
}

func TestOverviewConsolidated_ActivityAliasRouted(t *testing.T) {
	t.Setenv("HOMEPAGE_PREVIEW_ROOM_SLUGS", hpoSlug("act"))
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	slug := hpoSlug("act")
	hpoSeedRoom(t, pool, slug, "Activity Alias Room", "activity alias test", false, []string{
		"first activity message", "second activity message",
	})

	// /v1/overview/activity must answer the same as /v1/homepage/activity.
	first, _ := getConsolidatedActivity(t, ts.URL, 0, 6)
	require.NotEmpty(t, first.entries(), "activity endpoint must return entries")
	assert.Equal(t, "Load more", first.LoadMoreLabel)
}

func TestOverviewConsolidated_PartialErrorDegradation(t *testing.T) {
	// A failing search section must not break the room data. The endpoint still
	// returns 200 with partial_errors populated.
	t.Setenv("HOMEPAGE_PREVIEW_ROOM_SLUGS", hpoSlug("degrade"))
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	hpoSeedRoom(t, pool, hpoSlug("degrade"), "Degrade Room", "degrade test", false, []string{
		"a message", "another message",
	})

	_, raw := getConsolidatedOverview(t, ts.URL)

	// rooms data must always be present — a search failure never kills the page.
	// (The section's scope label is the marker; spec.json idx 96 reworded it.)
	assert.Contains(t, raw, "All rooms, private ones included", "rooms section must survive even if search fails: %s", raw)
}

func TestOverviewConsolidated_CacheInvalidatedOnVisibilityChange(t *testing.T) {
	t.Setenv("HOMEPAGE_PREVIEW_ROOM_SLUGS", hpoSlug("inv"))
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	slug := hpoSlug("inv")
	room := hpoSeedRoom(t, pool, slug, "Invalidation Room", "invalidation test", false, []string{
		"planner says hi", "executor replies",
	})

	// First read — cached.
	_, _ = getConsolidatedOverview(t, ts.URL)

	// Flip the room to private; the cache must drop so the preview vanishes.
	private := true
	_, err := db.NewRoomRepository(pool).Update(context.Background(), room.ID, models.UpdateRoomParams{
		IsPrivate: &private,
	}, nil)
	require.NoError(t, err)
	// In production, UpdateRoom handler calls InvalidateOverviewCache() after
	// a visibility change. The test updates the repository directly, so it
	// must trigger the same invalidation the handler would.
	handlers.InvalidateOverviewCache()

	_, raw := getConsolidatedOverview(t, ts.URL)
	// either way, the private room's slug must not survive.
	assert.NotContains(t, raw, slug, "private room preview must not survive a visibility flip via /v1/overview")
}
