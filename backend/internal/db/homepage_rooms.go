package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// The room statistics behind the homepage.
//
// Two kinds of number live here and they must never be confused:
//
//   - PRESENCE is measured NOW. It answers "who is in a room at this moment"
//     from unexpired server presence, and no time-window selector may change it.
//   - HISTORY is measured over a SELECTED WINDOW (24 hours, 7 days or 30 days).
//
// Two kinds of read live here too, and they have different floors:
//
//   - a COUNT covers every room that has not been deleted, private ones
//     included (countedRoomPredicate). A room's existence and its volume are
//     platform facts; a count says how many and how busy, never which.
//   - anything that NAMES, LISTS or QUOTES a room — the recently completed
//     rooms here, the activity stream in homepage_activity.go — is public-only
//     (publicRoomPredicate): a private room's name, slug, description,
//     participants and entries never leave the API through this page.
//
// On top of either floor, the live figures also exclude rooms whose
// `expires_at` has passed — an expired room is archived, not live — while the
// historical figures keep counting the messages such a room still holds,
// because that work really did happen.
//
// Using the counted floor in a read that returns a room's identity is a privacy
// leak, not a cosmetic bug.

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
//
// AgentsOnline counts every room; PublicAgentsOnline is the part of it that is
// online in a public room, reported beside it so the page can say how much of
// the live figure a reader can actually go and watch.
type RoomPresenceStats struct {
	AgentsOnline           int
	PublicAgentsOnline     int
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
	// RoomsWithConversation counts distinct rooms holding at least one
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
	// AllRooms and PublicRooms are all-time totals: they never move with the
	// selector. AllRooms counts every non-deleted room, private ones included;
	// PublicRooms is the part of it anyone can read.
	AllRooms    int
	PublicRooms int
	// Messages24h is pinned to 24 hours whatever window was selected, because
	// the API-usage section states "last 24 hours" in its own right.
	Messages24h int
}

// publicRoomPredicate is the visibility floor for every read that names, lists
// or quotes a room, and for the public-only figures reported beside the counts.
const publicRoomPredicate = `r.deleted_at IS NULL AND r.is_private = FALSE`

// liveRoomPredicate additionally excludes archived (expired) rooms. The public
// activity stream (homepage_activity.go) reads through it.
const liveRoomPredicate = publicRoomPredicate + ` AND (r.expires_at IS NULL OR r.expires_at > NOW())`

// countedRoomPredicate is the floor for every COUNT: every room that has not
// been deleted, private ones included. Never use it in a read that returns a
// room's identity or contents.
const countedRoomPredicate = `r.deleted_at IS NULL`

// countedLiveRoomPredicate additionally excludes archived (expired) rooms from
// the live counts.
const countedLiveRoomPredicate = countedRoomPredicate + ` AND (r.expires_at IS NULL OR r.expires_at > NOW())`

// unexpiredPresence is the definition of "online": a heartbeat that has not
// outlived its own TTL.
const unexpiredPresence = `ap.last_seen > NOW() - (ap.ttl_seconds || ' seconds')::interval`

// verifiedPresence is true when the present member (agent_presence.agent_id,
// migration 000099) holds a live (unexpired, not rotated away) per-agent token for that
// same room.
const verifiedPresence = `EXISTS (
	SELECT 1 FROM room_agent_tokens rt
	 WHERE rt.room_id = ap.room_id
	   AND rt.agent_id = ap.agent_id
	   AND rt.rotated_at IS NULL
	   AND (rt.expires_at IS NULL OR rt.expires_at > NOW()))`

// GetRoomPulse reads the whole room section for one selected window. Every
// figure counts every non-deleted room, private ones included, except the two
// public-only figures reported beside them (PublicAgentsOnline, PublicRooms).
func (r *HomepageRepository) GetRoomPulse(ctx context.Context, window RoomStatsWindow) (RoomPulse, error) {
	pulse := RoomPulse{Window: window}

	query := `
		SELECT
			(SELECT COUNT(DISTINCT ap.agent_id)
			   FROM agent_presence ap JOIN rooms r ON r.id = ap.room_id
			  WHERE ` + countedLiveRoomPredicate + ` AND ` + unexpiredPresence + `),
			(SELECT COUNT(DISTINCT ap.agent_id)
			   FROM agent_presence ap JOIN rooms r ON r.id = ap.room_id
			  WHERE ` + liveRoomPredicate + ` AND ` + unexpiredPresence + `),
			(SELECT COUNT(DISTINCT ap.agent_id)
			   FROM agent_presence ap JOIN rooms r ON r.id = ap.room_id
			  WHERE ` + countedLiveRoomPredicate + ` AND ` + unexpiredPresence + `
			    AND ` + verifiedPresence + `),
			(SELECT COUNT(DISTINCT ap.room_id)
			   FROM agent_presence ap JOIN rooms r ON r.id = ap.room_id
			  WHERE ` + countedLiveRoomPredicate + ` AND ` + unexpiredPresence + `),
			(SELECT COUNT(DISTINCT m.room_id)
			   FROM messages m JOIN rooms r ON r.id = m.room_id
			  WHERE ` + countedRoomPredicate + ` AND m.deleted_at IS NULL
			    AND m.author_type <> 'system'
			    AND m.created_at > NOW() - $1::interval),
			(SELECT COUNT(*)
			   FROM messages m JOIN rooms r ON r.id = m.room_id
			  WHERE ` + countedRoomPredicate + ` AND m.deleted_at IS NULL
			    AND m.author_type = 'agent'
			    AND m.created_at > NOW() - $1::interval),
			(SELECT COUNT(*)
			   FROM messages m JOIN rooms r ON r.id = m.room_id
			  WHERE ` + countedRoomPredicate + ` AND m.deleted_at IS NULL
			    AND m.author_type = 'agent' AND m.author_id IS NULL
			    AND m.created_at > NOW() - $1::interval),
			(SELECT COUNT(*)
			   FROM messages m JOIN rooms r ON r.id = m.room_id
			  WHERE ` + countedRoomPredicate + ` AND m.deleted_at IS NULL
			    AND m.author_type = 'human'
			    AND m.created_at > NOW() - $1::interval),
			(SELECT COUNT(DISTINCT e.room_id)
			   FROM room_events e JOIN rooms r ON r.id = e.room_id
			  WHERE ` + countedRoomPredicate + ` AND e.event_type = $2
			    AND e.created_at > NOW() - $1::interval),
			(SELECT MIN(e.created_at)
			   FROM room_events e JOIN rooms r ON r.id = e.room_id
			  WHERE ` + countedRoomPredicate + ` AND e.event_type = $2),
			(SELECT COUNT(*) FROM rooms r WHERE ` + countedRoomPredicate + `),
			(SELECT COUNT(*) FROM rooms r WHERE ` + publicRoomPredicate + `),
			(SELECT COUNT(*)
			   FROM messages m JOIN rooms r ON r.id = m.room_id
			  WHERE ` + countedRoomPredicate + ` AND m.deleted_at IS NULL
			    AND m.author_type <> 'system'
			    AND m.created_at > NOW() - INTERVAL '24 hours')
	`

	err := r.pool.QueryRow(ctx, query, window.interval(), RoomActivationEventType).Scan(
		&pulse.Presence.AgentsOnline,
		&pulse.Presence.PublicAgentsOnline,
		&pulse.Presence.VerifiedAgentsOnline,
		&pulse.Presence.RoomsWithAgentsOnline,
		&pulse.Stats.RoomsWithConversation,
		&pulse.Stats.AgentMessages,
		&pulse.Stats.UnverifiedAgentMessages,
		&pulse.Stats.HumanMessages,
		&pulse.Stats.TwoWayExchangeRooms,
		&pulse.Stats.ActivationInstrumentedSince,
		&pulse.AllRooms,
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
// zeros filled in for quiet steps so the series never has gaps. Like every
// count, it covers every non-deleted room, private ones included.
func (r *HomepageRepository) messageSeries(ctx context.Context, window RoomStatsWindow) ([]BucketCount, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT date_trunc($1, m.created_at) AS bucket, COUNT(*)
		  FROM messages m JOIN rooms r ON r.id = m.room_id
		 WHERE `+countedRoomPredicate+` AND m.deleted_at IS NULL
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
// participant. Concurrent callers (agents' first messages landing at once, on
// any API instance) are serialized per room by a transaction-scoped advisory
// lock, so the existence check always sees a milestone another caller committed.
func (r *RoomEventRepository) RecordActivation(ctx context.Context, roomID uuid.UUID) (bool, error) {
	var recorded bool
	err := r.pool.WithTx(ctx, func(tx Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1, hashtext($2))`,
			roomActivationLockClass, roomID.String()); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `
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
			return err
		}
		recorded = tag.RowsAffected() > 0
		return nil
	})
	if err != nil {
		LogQueryError(ctx, "RecordActivation", "room_events", err)
		return false, fmt.Errorf("record room activation: %w", err)
	}
	return recorded, nil
}

// roomActivationLockClass namespaces RecordActivation's per-room advisory lock
// (the two-key form: class, hashtext(room id)).
const roomActivationLockClass = 7301

// RecentCompletedRoom is a public room that had activity in the window but
// currently has no agents online. It lets the homepage show recent completed
// collaborations as a meaningful offline fallback instead of an empty presence.
type RecentCompletedRoom struct {
	RoomID       uuid.UUID `json:"room_id"`
	Slug         string    `json:"slug"`
	DisplayName  string    `json:"display_name"`
	Description  *string   `json:"description"`
	MessageCount int       `json:"message_count"`
	LastActiveAt time.Time `json:"last_active_at"`
}

// GetRecentCompletedRooms returns public, non-deleted rooms that had at least
// one non-system message within the window but currently have NO unexpired
// agent presence. These are "completed collaborations" — rooms where work
// happened recently, just not right now.
//
// The query excludes:
//   - private rooms (is_private = TRUE)
//   - deleted rooms (deleted_at IS NOT NULL)
//   - expired rooms (expires_at < NOW()) — they are archived, not active
//   - rooms with any current unexpired presence
//   - rooms with no non-system messages in the window
//
// Results are ordered by recency of last activity, newest first.
func (r *HomepageRepository) GetRecentCompletedRooms(ctx context.Context, window RoomStatsWindow) ([]RecentCompletedRoom, error) {
	query := `
		SELECT r.id, r.slug, r.display_name, r.description, r.message_count, r.last_active_at
		  FROM rooms r
		 WHERE ` + publicRoomPredicate + `
		   AND (r.expires_at IS NULL OR r.expires_at > NOW())
		   AND r.last_active_at > NOW() - $1::interval
		   AND EXISTS (
		         SELECT 1 FROM messages m
		          WHERE m.room_id = r.id
		            AND m.deleted_at IS NULL
		            AND m.author_type <> 'system'
		            AND m.created_at > NOW() - $1::interval
		       )
		   AND NOT EXISTS (
		         SELECT 1 FROM agent_presence ap
		          WHERE ap.room_id = r.id
		            AND ` + unexpiredPresence + `
		       )
		 ORDER BY r.last_active_at DESC
		 LIMIT 10
	`

	rows, err := r.pool.Query(ctx, query, window.interval())
	if err != nil {
		LogQueryError(ctx, "GetRecentCompletedRooms", "rooms", err)
		return nil, fmt.Errorf("get recent completed rooms: %w", err)
	}
	defer rows.Close()

	var results []RecentCompletedRoom
	for rows.Next() {
		var room RecentCompletedRoom
		if err := rows.Scan(
			&room.RoomID,
			&room.Slug,
			&room.DisplayName,
			&room.Description,
			&room.MessageCount,
			&room.LastActiveAt,
		); err != nil {
			return nil, fmt.Errorf("scan recent completed room: %w", err)
		}
		results = append(results, room)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent completed rooms: %w", err)
	}
	return results, nil
}

// NextPublicRoomExpiry returns the earliest expires_at still ahead among public, undeleted
// rooms, or nil when none is set. A cached public overview must not outlive it: the room
// leaves every listing at that moment and nothing announces it.
func (r *HomepageRepository) NextPublicRoomExpiry(ctx context.Context) (*time.Time, error) {
	var next *time.Time
	err := r.pool.QueryRow(ctx,
		`SELECT MIN(r.expires_at) FROM rooms r WHERE `+publicRoomPredicate+` AND r.expires_at > NOW()`,
	).Scan(&next)
	if err != nil {
		LogQueryError(ctx, "NextPublicRoomExpiry", "rooms", err)
		return nil, fmt.Errorf("next public room expiry: %w", err)
	}
	return next, nil
}
