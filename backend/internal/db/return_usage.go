package db

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"
)

// Measuring return usage (idx 92 steps 2, 4 and 6).
//
// Growth here means completed or resumed useful work, not notifications sent or sessions left
// open. A USEFUL COLLABORATION is a room reaching its first two-way exchange (funnel
// first_two_way_exchange). The report keeps agents and humans apart and reports:
//
//   - the time between an identity's successive useful collaborations;
//   - rooms whose conversation resumed after a quiet day;
//   - what became of the rooms created in the window — a naturally completed one-off task is
//     not an onboarding failure;
//   - opt-in room notifications sent, and how many were followed by work — shown apart, and
//     never a growth metric;
//   - language demand from what people searched for (a heuristic over the query text).
//
// INTERNAL operator analytics: pseudonymous refs and counts only.

// ReturnGapBuckets counts gaps between successive collaborations by length.
type ReturnGapBuckets struct {
	Under1d    int `json:"under_1d"`
	From1To7d  int `json:"from_1_to_7d"`
	From7To28d int `json:"from_7_to_28d"`
	Over28d    int `json:"over_28d"`
}

// ReturnGapStats is the collaboration rhythm of one kind of identity.
type ReturnGapStats struct {
	Actors           int              `json:"actors"`
	ActorsWithRepeat int              `json:"actors_with_repeat"`
	Gaps             int              `json:"gaps"`
	MedianGapHours   *float64         `json:"median_gap_hours"`
	P90GapHours      *float64         `json:"p90_gap_hours"`
	Buckets          ReturnGapBuckets `json:"buckets"`
}

// ReturnCollaborations splits the rhythm by identity kind.
type ReturnCollaborations struct {
	Agents ReturnGapStats `json:"agents"`
	Humans ReturnGapStats `json:"humans"`
}

// RoomOutcomes is what became of the rooms created in the window, judged as of now.
type RoomOutcomes struct {
	Created                int `json:"created"`
	OnboardingFailure      int `json:"onboarding_failure"`
	StalledAfterActivation int `json:"stalled_after_activation"`
	CompletedOneOff        int `json:"completed_one_off"`
	CompletedThenReturned  int `json:"completed_then_returned"`
}

// ResumedRooms counts conversations that resumed after at least a day of quiet.
type ResumedRooms struct {
	Rooms       int `json:"rooms"`
	Resumptions int `json:"resumptions"`
}

// ReturnNotifications is the opt-in room notifications sent in the window.
type ReturnNotifications struct {
	SentByType       map[string]int `json:"sent_by_type"`
	ResumedWithin24h int            `json:"resumed_within_24h"`
	NotAGrowthMetric bool           `json:"not_a_growth_metric"`
}

// LanguageBucket is the search demand of one script or language marker.
type LanguageBucket struct {
	Bucket         string   `json:"bucket"`
	Queries        int      `json:"queries"`
	ZeroResults    int      `json:"zero_results"`
	ZeroResultRate *float64 `json:"zero_result_rate"`
}

// ReturnUsageReport is the return-usage report over one window.
type ReturnUsageReport struct {
	Collaborations ReturnCollaborations `json:"collaborations"`
	Resumed        ResumedRooms         `json:"resumed_rooms"`
	Outcomes       RoomOutcomes         `json:"room_outcomes"`
	Notifications  ReturnNotifications  `json:"notifications"`
	LanguageDemand []LanguageBucket     `json:"language_demand"`
	Definitions    map[string]string    `json:"definitions"`
	Caveats        []string             `json:"caveats"`
}

// ReturnUsageRepository measures return usage.
type ReturnUsageRepository struct {
	pool *Pool
}

// NewReturnUsageRepository creates the return-usage reader.
func NewReturnUsageRepository(pool *Pool) *ReturnUsageRepository {
	return &ReturnUsageRepository{pool: pool}
}

// resumeQuietGap is how long a room must have been quiet for a message to resume it.
const resumeQuietGap = 24 * time.Hour

// Measure reports return usage for [from, to), judging outcomes as of now.
func (r *ReturnUsageRepository) Measure(ctx context.Context, from, to, now time.Time) (ReturnUsageReport, error) {
	rep := ReturnUsageReport{Definitions: returnDefinitions(), Caveats: returnCaveats()}
	var err error
	if rep.Collaborations.Agents, err = r.rhythm(ctx, agentCollaborationsSQL, from, to); err != nil {
		return rep, err
	}
	if rep.Collaborations.Humans, err = r.rhythm(ctx, humanCollaborationsSQL, from, to); err != nil {
		return rep, err
	}
	if rep.Outcomes, err = r.outcomes(ctx, from, to, now); err != nil {
		return rep, err
	}
	if rep.Resumed, err = r.resumed(ctx, from, to); err != nil {
		return rep, err
	}
	if rep.Notifications, err = r.notifications(ctx, from, to); err != nil {
		return rep, err
	}
	if rep.LanguageDemand, err = r.languageDemand(ctx, from, to); err != nil {
		return rep, err
	}
	return rep, nil
}

// agentCollaborationsSQL: every (agent, activated room, activation instant) up to `to`.
const agentCollaborationsSQL = `
	SELECT DISTINCT j.actor_ref, x.room_id::text, x.occurred_at
	  FROM funnel_events x
	  JOIN funnel_events j ON j.room_id = x.room_id AND j.event_name = 'participant_joined'
	   AND j.actor_type = 'agent' AND j.actor_ref IS NOT NULL
	 WHERE x.event_name = 'first_two_way_exchange' AND x.occurred_at < $1`

// humanCollaborationsSQL: every (identified human, activated room, activation instant) up to
// `to`, a human taking part through the room's own rows or the flow that created it.
const humanCollaborationsSQL = `
	SELECT DISTINCT h.actor_ref, x.room_id::text, x.occurred_at
	  FROM funnel_events x
	  LEFT JOIN funnel_events c ON c.room_id = x.room_id AND c.event_name = 'room_created'
	  JOIN funnel_events h ON h.actor_type = 'human' AND h.actor_ref IS NOT NULL
	   AND (h.room_id = x.room_id OR (c.flow_id IS NOT NULL AND h.flow_id = c.flow_id))
	 WHERE x.event_name = 'first_two_way_exchange' AND x.occurred_at < $1`

// rhythm turns (identity, activation) pairs into the gaps between successive collaborations
// whose later collaboration falls in the window.
func (r *ReturnUsageRepository) rhythm(ctx context.Context, query string, from, to time.Time) (ReturnGapStats, error) {
	rows, err := r.pool.Query(ctx, query, to)
	if err != nil {
		LogQueryError(ctx, "ReturnUsage.rhythm", "funnel_events", err)
		return ReturnGapStats{}, fmt.Errorf("return usage rhythm: %w", err)
	}
	defer rows.Close()
	byActor := map[string][]time.Time{}
	for rows.Next() {
		var ref, room string
		var at time.Time
		if err := rows.Scan(&ref, &room, &at); err != nil {
			return ReturnGapStats{}, fmt.Errorf("scan collaboration: %w", err)
		}
		byActor[ref] = append(byActor[ref], at)
	}
	if err := rows.Err(); err != nil {
		return ReturnGapStats{}, err
	}

	var stats ReturnGapStats
	var gaps []float64
	for _, times := range byActor {
		sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })
		inWindow, repeat := false, false
		for i, at := range times {
			if at.Before(from) {
				continue
			}
			inWindow = true
			if i > 0 {
				repeat = true
				gaps = append(gaps, at.Sub(times[i-1]).Hours())
			}
		}
		if inWindow {
			stats.Actors++
		}
		if repeat {
			stats.ActorsWithRepeat++
		}
	}
	stats.Gaps = len(gaps)
	sort.Float64s(gaps)
	stats.MedianGapHours = nearestRank(gaps, 0.5)
	stats.P90GapHours = nearestRank(gaps, 0.9)
	for _, h := range gaps {
		switch {
		case h < 24:
			stats.Buckets.Under1d++
		case h < 7*24:
			stats.Buckets.From1To7d++
		case h < 28*24:
			stats.Buckets.From7To28d++
		default:
			stats.Buckets.Over28d++
		}
	}
	return stats, nil
}

// nearestRank is the nearest-rank percentile of sorted values, or nil for none.
func nearestRank(sorted []float64, p float64) *float64 {
	if len(sorted) == 0 {
		return nil
	}
	i := int(math.Ceil(p*float64(len(sorted)))) - 1
	if i < 0 {
		i = 0
	}
	v := sorted[i]
	return &v
}

// outcomes classifies the rooms created in the window: never activated (onboarding failure),
// activated but not completed (stalled), completed with no return of its creator (a natural
// one-off), completed and its creator came back for other work.
func (r *ReturnUsageRepository) outcomes(ctx context.Context, from, to, now time.Time) (RoomOutcomes, error) {
	var o RoomOutcomes
	err := r.pool.QueryRow(ctx, `
		WITH created AS (
			SELECT c.room_id, c.actor_ref, x.occurred_at AS activated_at,
			       (rm.archived_at IS NOT NULL OR EXISTS (
			           SELECT 1 FROM room_entries d WHERE d.room_id = c.room_id AND d.kind = 'event'
			              AND lower(d.event_type) IN ('done', 'task.done'))) AS completed
			  FROM funnel_events c
			  LEFT JOIN funnel_events x ON x.room_id = c.room_id AND x.event_name = 'first_two_way_exchange' AND x.occurred_at <= $3
			  LEFT JOIN rooms rm ON rm.id = c.room_id
			 WHERE c.event_name = 'room_created' AND c.occurred_at >= $1 AND c.occurred_at < $2
		)
		SELECT COUNT(*)::int,
		       COUNT(*) FILTER (WHERE activated_at IS NULL)::int,
		       COUNT(*) FILTER (WHERE activated_at IS NOT NULL AND NOT completed)::int,
		       COUNT(*) FILTER (WHERE activated_at IS NOT NULL AND completed AND NOT returned)::int,
		       COUNT(*) FILTER (WHERE activated_at IS NOT NULL AND completed AND returned)::int
		  FROM (SELECT created.*, (created.actor_ref IS NOT NULL AND EXISTS (
		            SELECT 1 FROM funnel_events o
		             WHERE o.actor_ref = created.actor_ref AND o.room_id IS DISTINCT FROM created.room_id
		               AND o.event_name IN ('room_created', 'participant_joined')
		               AND o.occurred_at > created.activated_at AND o.occurred_at <= $3)) AS returned
		          FROM created) classified`, from, to, now).Scan(
		&o.Created, &o.OnboardingFailure, &o.StalledAfterActivation, &o.CompletedOneOff, &o.CompletedThenReturned)
	if err != nil {
		LogQueryError(ctx, "ReturnUsage.outcomes", "funnel_events", err)
		return o, fmt.Errorf("return usage outcomes: %w", err)
	}
	return o, nil
}

// resumed counts messages in the window that came after at least a day of quiet in their room.
func (r *ReturnUsageRepository) resumed(ctx context.Context, from, to time.Time) (ResumedRooms, error) {
	var out ResumedRooms
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT room_id)::int, COUNT(*)::int FROM (
			SELECT e.room_id, e.created_at,
			       (SELECT MAX(p.created_at) FROM room_entries p
			         WHERE p.room_id = e.room_id AND p.kind = 'message' AND p.deleted_at IS NULL
			           AND p.sequence < e.sequence) AS previous
			  FROM room_entries e
			 WHERE e.kind = 'message' AND e.deleted_at IS NULL AND e.created_at >= $1 AND e.created_at < $2
		) m WHERE previous IS NOT NULL AND created_at - previous >= $3::interval`,
		from, to, resumeQuietGap.String()).Scan(&out.Rooms, &out.Resumptions)
	if err != nil {
		LogQueryError(ctx, "ReturnUsage.resumed", "room_entries", err)
		return out, fmt.Errorf("return usage resumed rooms: %w", err)
	}
	return out, nil
}

// notifications counts the opt-in room events sent in the window, and those whose recipient
// posted in that room within a day — shown apart, never as growth.
func (r *ReturnUsageRepository) notifications(ctx context.Context, from, to time.Time) (ReturnNotifications, error) {
	out := ReturnNotifications{SentByType: map[string]int{}, NotAGrowthMetric: true}
	rows, err := r.pool.Query(ctx, `
		SELECT n.type, COUNT(*)::int,
		       COUNT(*) FILTER (WHERE EXISTS (
		           SELECT 1 FROM room_entries e WHERE e.room_id = n.room_id
		              AND e.author_id = COALESCE(n.user_id::text, n.agent_id)
		              AND e.created_at > n.created_at AND e.created_at <= n.created_at + INTERVAL '24 hours'))::int
		  FROM notifications n
		 WHERE n.schema_version = 3 AND n.created_at >= $1 AND n.created_at < $2
		 GROUP BY n.type`, from, to)
	if err != nil {
		LogQueryError(ctx, "ReturnUsage.notifications", "notifications", err)
		return out, fmt.Errorf("return usage notifications: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var typ string
		var sent, resumed int
		if err := rows.Scan(&typ, &sent, &resumed); err != nil {
			return out, fmt.Errorf("scan notification count: %w", err)
		}
		out.SentByType[typ] = sent
		out.ResumedWithin24h += resumed
	}
	return out, rows.Err()
}

// languageBuckets is every bucket the language report states, in order, empty ones included.
var languageBuckets = []string{"han", "japanese_korean", "portuguese_marked", "other_non_latin", "latin_other"}

// languageDemand classifies the window's search queries by script and Portuguese markers.
func (r *ReturnUsageRepository) languageDemand(ctx context.Context, from, to time.Time) ([]LanguageBucket, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT bucket, COUNT(*)::int, COUNT(*) FILTER (WHERE results_count = 0)::int FROM (
			SELECT results_count, CASE
				WHEN query ~ '[一-鿿]' THEN 'han'
				WHEN query ~ '[぀-ヿ가-힯]' THEN 'japanese_korean'
				WHEN query ~* '[ãõç]' OR query ~* '\m(não|nao|como|para|erro|está|você|voce|porque)\M' THEN 'portuguese_marked'
				WHEN query ~ '[^\x01-\x7fÀ-ɏ]' THEN 'other_non_latin'
				ELSE 'latin_other' END AS bucket
			  FROM search_queries WHERE searched_at >= $1 AND searched_at < $2
		) q GROUP BY bucket`, from, to)
	if err != nil {
		LogQueryError(ctx, "ReturnUsage.language", "search_queries", err)
		return nil, fmt.Errorf("return usage language demand: %w", err)
	}
	defer rows.Close()
	counts := map[string][2]int{}
	for rows.Next() {
		var bucket string
		var n, zero int
		if err := rows.Scan(&bucket, &n, &zero); err != nil {
			return nil, fmt.Errorf("scan language bucket: %w", err)
		}
		counts[bucket] = [2]int{n, zero}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]LanguageBucket, 0, len(languageBuckets))
	for _, b := range languageBuckets {
		c := counts[b]
		lb := LanguageBucket{Bucket: b, Queries: c[0], ZeroResults: c[1]}
		if c[0] > 0 {
			rate := float64(c[1]) / float64(c[0])
			lb.ZeroResultRate = &rate
		}
		out = append(out, lb)
	}
	return out, nil
}

func returnDefinitions() map[string]string {
	return map[string]string{
		"useful_collaboration":     "a room reaching its first two-way exchange (funnel first_two_way_exchange)",
		"collaborations.gaps":      "time between an identity's successive useful collaborations whose later one is in the window; agents and humans apart",
		"resumed_rooms":            "messages in the window that came after at least 24 h of quiet in their room",
		"onboarding_failure":       "a room created in the window that never reached a two-way exchange",
		"stalled_after_activation": "activated, but not archived and no DONE / task.done event",
		"completed_one_off":        "activated and completed (archived, or a DONE / task.done event), its creator not seen in another room since",
		"completed_then_returned":  "activated and completed, and its creator came back to another room afterwards",
		"notifications":            "opt-in room notifications (schema 3) sent in the window; resumed = the recipient posted in that room within 24 h",
		"language_demand":          "search queries by script (han, japanese_korean, other_non_latin) or Portuguese markers; the rest latin_other",
	}
}

func returnCaveats() []string {
	return []string{
		"Notifications sent are not growth: growth is completed or resumed work.",
		"A one-off task that was completed is not an onboarding failure.",
		"Language buckets are a heuristic over query text, not verified geography or conversion; geography is the GA4 export.",
		"Humans are counted only from identified rows; anonymous visitors are not guessed into people.",
	}
}
