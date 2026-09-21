package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// The public room statistics behind the homepage.
//
// Two kinds of number live here and they must never be confused:
//
//   - PRESENCE is measured NOW. It answers "who is in a room at this moment"
//     from unexpired server presence, and no time-window selector may change it.
//   - HISTORY is measured over a SELECTED WINDOW (24 hours, 7 days or 30 days).
//
// Everything is public-only by construction: `is_private = FALSE AND
// deleted_at IS NULL`. On top of that, the live figures also exclude rooms
// whose `expires_at` has passed — an expired room is archived, not live — while
// the historical figures keep counting the messages such a room still holds,
// because that work really did happen.
//
// A missing predicate here is a privacy leak, not a cosmetic bug.

// RoomActivationEventType is the room_events type that records the activation
// milestone: the first moment a room carried a two-way exchange.
//
// The metric derived from it is deliberately NOT recomputed from message
// history. A room is activated when the milestone was RECORDED, so periods
// before the milestone existed are reported as unavailable rather than as zero.
const RoomActivationEventType = "room.activated"

// activationMinAuthors is how many distinct non-system authors a room needs
// before it counts as a two-way exchange.
const activationMinAuthors = 2

// RoomStatsWindow is one selectable measurement window for the historical
// room statistics. The API owns this vocabulary: the browser renders the
// labels it is given and sends back a Value.
type RoomStatsWindow struct {
	// Value is the wire form: what a caller sends as ?window=.
	Value string
	// Label is how the control reads ("7 days").
	Label string
	// WindowText is how a metric states its own measurement period.
	WindowText string
	// Duration is the length of the window.
	Duration time.Duration
	// BucketUnit is the date_trunc unit for the time series ("hour"/"day").
	BucketUnit string
	// Buckets is how many points the series has.
	Buckets int
}

// RoomStatsWindows is the offered set, in display order. The first entry is
// the default.
var RoomStatsWindows = []RoomStatsWindow{
	{
		Value: "24h", Label: "24 hours", WindowText: "last 24 hours",
		Duration: 24 * time.Hour, BucketUnit: "hour", Buckets: 24,
	},
	{
		Value: "7d", Label: "7 days", WindowText: "last 7 days",
		Duration: 7 * 24 * time.Hour, BucketUnit: "day", Buckets: 7,
	},
	{
		Value: "30d", Label: "30 days", WindowText: "last 30 days",
		Duration: 30 * 24 * time.Hour, BucketUnit: "day", Buckets: 30,
	},
}

// DefaultRoomStatsWindow is what the homepage shows before anyone chooses.
func DefaultRoomStatsWindow() RoomStatsWindow {
	return RoomStatsWindows[0]
}

// RoomStatsWindowByValue resolves a wire value. An unknown value is rejected
// rather than silently coerced, so a caller can tell the difference between
// "no window given" and "a window I do not offer".
func RoomStatsWindowByValue(value string) (RoomStatsWindow, bool) {
	for _, w := range RoomStatsWindows {
		if w.Value == value {
			return w, true
		}
	}
	return RoomStatsWindow{}, false
}

// interval renders the window as a Postgres interval literal.
func (w RoomStatsWindow) interval() string {
	return fmt.Sprintf("%d seconds", int(w.Duration.Seconds()))
}

// bucketInterval renders one series step as a Postgres interval literal.
func (w RoomStatsWindow) bucketInterval() string {
	return "1 " + w.BucketUnit
}

// RoomPresenceStats is the "now" half of the room statistics. It is never
// windowed.
//
// VerifiedAgentsOnline counts agents whose identity was PROVEN for that room —
// they completed a handshake with their Solvr agent key and hold a per-agent
// room token. UnverifiedAgentsOnline counts the rest: presence claimed by name
// alone, which anyone can type. The two are reported separately so the page can
// say so out loud instead of mixing proven and claimed identities into one
// number.
type RoomPresenceStats struct {
	AgentsOnline           int
	VerifiedAgentsOnline   int
	UnverifiedAgentsOnline int
	RoomsWithAgentsOnline  int
}

// BucketCount is one step of a time series: the instant it starts and its count.
type BucketCount struct {
	BucketStart time.Time
	Count       int
}

// RoomWindowStats is the historical half: everything measured over the
// selected window.
type RoomWindowStats struct {
	// RoomsWithConversation counts distinct public rooms holding at least one
	// persisted, non-deleted, non-system message inside the window.
	RoomsWithConversation int
	// AgentMessages and HumanMessages are persisted, non-deleted messages.
	AgentMessages int
	HumanMessages int
	// UnverifiedAgentMessages is the subset of AgentMessages posted with the
	// shared room token, where authorship is a name rather than a proven id.
	UnverifiedAgentMessages int
	// TwoWayExchangeRooms counts distinct rooms whose activation milestone was
	// recorded inside the window.
	TwoWayExchangeRooms int
	// ActivationInstrumentedSince is the earliest recorded milestone, ever.
	// Nil means the milestone has never been recorded, so the metric is
	// unavailable rather than zero.
	ActivationInstrumentedSince *time.Time
	// Series is the message volume over the window, oldest bucket first.
	Series []BucketCount
}

// RoomPulse is everything the homepage's room section needs in one read.
type RoomPulse struct {
	Window   RoomStatsWindow
	Presence RoomPresenceStats
	Stats    RoomWindowStats
	// PublicRooms is an all-time total: it never moves with the selector.
	PublicRooms int
	// Messages24h is pinned to 24 hours whatever window was selected, because
	// the API-usage section states "last 24 hours" in its own right.
	Messages24h int
}

// publicRoomPredicate is the visibility floor for every query in this file.
const publicRoomPredicate = `r.deleted_at IS NULL AND r.is_private = FALSE`

// liveRoomPredicate additionally excludes archived (expired) rooms from the
// live figures.
const liveRoomPredicate = publicRoomPredicate + ` AND (r.expires_at IS NULL OR r.expires_at > NOW())`

// unexpiredPresence is the definition of "online": a heartbeat that has not
// outlived its own TTL.
const unexpiredPresence = `ap.last_seen > NOW() - (ap.ttl_seconds || ' seconds')::interval`

// verifiedPresence is true when the present agent name belongs to an agent
// holding a live per-agent token for that same room.
const verifiedPresence = `EXISTS (
	SELECT 1 FROM room_agent_tokens rt
	 WHERE rt.room_id = ap.room_id
	   AND rt.agent_id = ap.agent_name
	   AND (rt.expires_at IS NULL OR rt.expires_at > NOW()))`

// GetRoomPulse reads the whole room section for one selected window.
func (r *HomepageRepository) GetRoomPulse(ctx context.Context, window RoomStatsWindow) (RoomPulse, error) {
	pulse := RoomPulse{Window: window}

	query := `
		SELECT
			(SELECT COUNT(DISTINCT ap.agent_name)
			   FROM agent_presence ap JOIN rooms r ON r.id = ap.room_id
			  WHERE ` + liveRoomPredicate + ` AND ` + unexpiredPresence + `),
			(SELECT COUNT(DISTINCT ap.agent_name)
			   FROM agent_presence ap JOIN rooms r ON r.id = ap.room_id
			  WHERE ` + liveRoomPredicate + ` AND ` + unexpiredPresence + `
			    AND ` + verifiedPresence + `),
			(SELECT COUNT(DISTINCT ap.room_id)
			   FROM agent_presence ap JOIN rooms r ON r.id = ap.room_id
			  WHERE ` + liveRoomPredicate + ` AND ` + unexpiredPresence + `),
			(SELECT COUNT(DISTINCT m.room_id)
			   FROM messages m JOIN rooms r ON r.id = m.room_id
			  WHERE ` + publicRoomPredicate + ` AND m.deleted_at IS NULL
			    AND m.author_type <> 'system'
			    AND m.created_at > NOW() - $1::interval),
			(SELECT COUNT(*)
			   FROM messages m JOIN rooms r ON r.id = m.room_id
			  WHERE ` + publicRoomPredicate + ` AND m.deleted_at IS NULL
			    AND m.author_type = 'agent'
			    AND m.created_at > NOW() - $1::interval),
			(SELECT COUNT(*)
			   FROM messages m JOIN rooms r ON r.id = m.room_id
			  WHERE ` + publicRoomPredicate + ` AND m.deleted_at IS NULL
			    AND m.author_type = 'agent' AND m.author_id IS NULL
			    AND m.created_at > NOW() - $1::interval),
			(SELECT COUNT(*)
			   FROM messages m JOIN rooms r ON r.id = m.room_id
			  WHERE ` + publicRoomPredicate + ` AND m.deleted_at IS NULL
			    AND m.author_type = 'human'
			    AND m.created_at > NOW() - $1::interval),
			(SELECT COUNT(DISTINCT e.room_id)
			   FROM room_events e JOIN rooms r ON r.id = e.room_id
			  WHERE ` + publicRoomPredicate + ` AND e.event_type = $2
			    AND e.created_at > NOW() - $1::interval),
			(SELECT MIN(e.created_at)
			   FROM room_events e JOIN rooms r ON r.id = e.room_id
			  WHERE ` + publicRoomPredicate + ` AND e.event_type = $2),
			(SELECT COUNT(*) FROM rooms r WHERE ` + publicRoomPredicate + `),
			(SELECT COUNT(*)
			   FROM messages m JOIN rooms r ON r.id = m.room_id
			  WHERE ` + publicRoomPredicate + ` AND m.deleted_at IS NULL
			    AND m.author_type <> 'system'
			    AND m.created_at > NOW() - INTERVAL '24 hours')
	`

	err := r.pool.QueryRow(ctx, query, window.interval(), RoomActivationEventType).Scan(
		&pulse.Presence.AgentsOnline,
		&pulse.Presence.VerifiedAgentsOnline,
		&pulse.Presence.RoomsWithAgentsOnline,
		&pulse.Stats.RoomsWithConversation,
		&pulse.Stats.AgentMessages,
		&pulse.Stats.UnverifiedAgentMessages,
		&pulse.Stats.HumanMessages,
		&pulse.Stats.TwoWayExchangeRooms,
		&pulse.Stats.ActivationInstrumentedSince,
		&pulse.PublicRooms,
		&pulse.Messages24h,
	)
	if err != nil {
		LogQueryError(ctx, "GetRoomPulse", "rooms", err)
		return pulse, fmt.Errorf("get room pulse: %w", err)
	}

	pulse.Presence.UnverifiedAgentsOnline =
		pulse.Presence.AgentsOnline - pulse.Presence.VerifiedAgentsOnline

	series, err := r.messageSeries(ctx, window)
	if err != nil {
		return pulse, err
	}
	pulse.Stats.Series = series

	return pulse, nil
}

// messageSeries returns exactly window.Buckets buckets, oldest first, with
// zeros filled in for quiet steps so the series never has gaps.
func (r *HomepageRepository) messageSeries(ctx context.Context, window RoomStatsWindow) ([]BucketCount, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT date_trunc($1, m.created_at) AS bucket, COUNT(*)
		  FROM messages m JOIN rooms r ON r.id = m.room_id
		 WHERE `+publicRoomPredicate+` AND m.deleted_at IS NULL
		   AND m.author_type <> 'system'
		   AND m.created_at >= date_trunc($1, NOW()) - ($2::int - 1) * $3::interval
		 GROUP BY bucket
	`, window.BucketUnit, window.Buckets, window.bucketInterval())
	if err != nil {
		LogQueryError(ctx, "messageSeries", "messages", err)
		return nil, fmt.Errorf("message series: %w", err)
	}
	defer rows.Close()

	counts := make(map[time.Time]int, window.Buckets)
	for rows.Next() {
		var bucket time.Time
		var count int
		if err := rows.Scan(&bucket, &count); err != nil {
			return nil, fmt.Errorf("scan series bucket: %w", err)
		}
		counts[truncateToBucket(bucket.UTC(), window.BucketUnit)] = count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	current := truncateToBucket(time.Now().UTC(), window.BucketUnit)
	step := bucketStep(window.BucketUnit)

	series := make([]BucketCount, 0, window.Buckets)
	for i := window.Buckets - 1; i >= 0; i-- {
		start := current.Add(-time.Duration(i) * step)
		series = append(series, BucketCount{BucketStart: start, Count: counts[start]})
	}
	return series, nil
}

// truncateToBucket mirrors Postgres date_trunc for the units this file uses.
func truncateToBucket(t time.Time, unit string) time.Time {
	if unit == "day" {
		return t.Truncate(24 * time.Hour)
	}
	return t.Truncate(time.Hour)
}

// bucketStep is the distance between two consecutive buckets.
func bucketStep(unit string) time.Duration {
	if unit == "day" {
		return 24 * time.Hour
	}
	return time.Hour
}

// RecordActivation records the activation milestone for a room the first time
// the room carries a two-way exchange — messages from at least two distinct
// non-system authors. It reports whether it recorded one.
//
// It is idempotent: a room is activated once, and every later message leaves
// the milestone alone. Deleted and system messages never count as a second
// participant.
func (r *RoomEventRepository) RecordActivation(ctx context.Context, roomID uuid.UUID) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO room_events (room_id, event_type, actor, payload)
		SELECT $1, $2, 'system', jsonb_build_object('distinct_authors', c.authors)
		  FROM (SELECT COUNT(DISTINCT agent_name) AS authors
		          FROM messages
		         WHERE room_id = $1 AND deleted_at IS NULL AND author_type <> 'system') c
		 WHERE c.authors >= $3
		   AND NOT EXISTS (
		       SELECT 1 FROM room_events e
		        WHERE e.room_id = $1 AND e.event_type = $2)
	`, roomID, RoomActivationEventType, activationMinAuthors)
	if err != nil {
		LogQueryError(ctx, "RecordActivation", "room_events", err)
		return false, fmt.Errorf("record room activation: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}
