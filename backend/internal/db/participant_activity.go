package db

import (
	"context"
	"fmt"
	"time"

	"github.com/fcavalcantirj/solvr/internal/growth"
)

// Counting monthly active participants (spec.json idx 86).
//
// A participant is an IDENTITY that did something useful inside a rolling 30-day window: a
// human account (users.id) or an agent identity (agents.id). growth/definitions.go states the
// definition in words; this file is the same definition in SQL, and the two must say the same
// thing. Everything is computed when it is read, from the canonical records — post views,
// searches, posts, replies, votes, bookmarks, follows, room entries and the funnel's server
// room_created step — and never from the legacy contribution tables.
//
// What makes a figure honest here:
//   - IDENTITIES, NOT SESSIONS. Each source contributes (actor_type, actor_id) and the counts
//     are COUNT(DISTINCT) over the whole window, so an agent's two CLI sessions, a human's two
//     devices and a hundred messages are each one identity.
//   - ACTIONS, NOT ACCOUNTS. Rows that record that an identity merely EXISTS or is ALIVE —
//     registration, sign-in, heartbeat/briefing/presence, an owner-granted membership, a
//     referral, a notification — are not sources at all. An invited human appears only after
//     acting.
//   - EXCLUSIONS ARE MARKERS, NOT GUESSES. Tombstoned or banned identities, suspended agents and
//     searches from KnownMonitoringAgents are excluded. No owner/test flag exists, so nothing
//     else is dropped by guess.
//   - ANONYMOUS IS EVENTS. Anonymous searches, post views and browser funnel steps are counted
//     as events; a visitor count would need an identifier Solvr does not store. An anonymous
//     connection flow is linked to an identity only by its server-issued flow_id.
//
// The room_created step stores a pseudonymous actor_ref (PseudonymizeActor); the SQL recomputes
// that hash per agent to name the creator. It is the same deterministic, unsalted mapping the
// funnel wrote, so no identity is guessed.

// actorRefSQL recomputes PseudonymizeActor(a.id) in SQL.
const actorRefSQL = `encode(substring(sha256(convert_to('funnel:' || a.id, 'UTF8')) from 1 for 16), 'hex')`

// monitoredUserAgentsCTE classifies the DISTINCT user agents of the searches in [$1, $2) against
// the known monitors in $3 — once per agent string, never per search (see monitoredExpr).
const monitoredUserAgentsCTE = `
	monitored AS (
		SELECT ua FROM (
			SELECT DISTINCT COALESCE(user_agent, '') AS ua
			  FROM search_queries
			 WHERE searched_at >= $1 AND searched_at < $2
		) d
		 WHERE ua ILIKE ANY($3::text[])
	)`

// qualifyingActivityCTE yields eligible(actor_type, actor_id, occurred_at, action): one row per
// qualifying action in [$1, $2) by a counted identity. $3 is the monitoring patterns.
const qualifyingActivityCTE = monitoredUserAgentsCTE + `,
	activity AS (
		SELECT viewer_type::text AS actor_type, viewer_id::text AS actor_id, viewed_at AS occurred_at, 'read' AS action
		  FROM post_views
		 WHERE viewer_type IN ('human', 'agent') AND viewer_id IS NOT NULL
		   AND viewed_at >= $1 AND viewed_at < $2
		UNION ALL
		SELECT searcher_type::text, searcher_id::text, searched_at, 'search'
		  FROM search_queries
		 WHERE searcher_type IN ('human', 'agent') AND searcher_id IS NOT NULL
		   AND searched_at >= $1 AND searched_at < $2
		   AND COALESCE(user_agent, '') NOT IN (SELECT ua FROM monitored)
		UNION ALL
		SELECT CASE WHEN author_human_id IS NOT NULL THEN 'human' ELSE 'agent' END,
		       COALESCE(author_human_id::text, author_agent_id::text), created_at, 'create'
		  FROM posts
		 WHERE (author_human_id IS NOT NULL OR author_agent_id IS NOT NULL) AND deleted_at IS NULL
		   AND created_at >= $1 AND created_at < $2
		UNION ALL
		SELECT CASE WHEN author_human_id IS NOT NULL THEN 'human' ELSE 'agent' END,
		       COALESCE(author_human_id::text, author_agent_id::text), created_at, 'create'
		  FROM replies
		 WHERE (author_human_id IS NOT NULL OR author_agent_id IS NOT NULL) AND deleted_at IS NULL
		   AND created_at >= $1 AND created_at < $2
		UNION ALL
		SELECT voter_type::text, voter_id::text, created_at, 'participate'
		  FROM votes
		 WHERE created_at >= $1 AND created_at < $2
		UNION ALL
		SELECT user_type::text, user_id::text, created_at, 'participate'
		  FROM bookmarks
		 WHERE created_at >= $1 AND created_at < $2
		UNION ALL
		SELECT 'human', follower_id::text, created_at, 'participate'
		  FROM follows
		 WHERE follower_type = 'human' AND created_at >= $1 AND created_at < $2
		UNION ALL
		SELECT author_type::text, author_id::text, created_at, 'room'
		  FROM room_entries
		 WHERE author_type IN ('human', 'agent') AND author_id IS NOT NULL AND deleted_at IS NULL
		   AND created_at >= $1 AND created_at < $2
		UNION ALL
		SELECT 'agent', a.id::text, f.occurred_at, 'room'
		  FROM funnel_events f
		  JOIN agents a ON f.actor_ref = ` + actorRefSQL + `
		 WHERE f.event_name = 'room_created' AND f.actor_type = 'agent' AND f.actor_ref IS NOT NULL
		   AND f.occurred_at >= $1 AND f.occurred_at < $2
	),
	eligible AS (
		SELECT x.actor_type, x.actor_id, x.occurred_at, x.action
		  FROM activity x
		  JOIN users u ON x.actor_type = 'human' AND u.id::text = x.actor_id
		 WHERE u.deleted_at IS NULL
		UNION ALL
		SELECT x.actor_type, x.actor_id, x.occurred_at, x.action
		  FROM activity x
		  JOIN agents a ON x.actor_type = 'agent' AND a.id = x.actor_id
		 WHERE a.deleted_at IS NULL AND a.status IS DISTINCT FROM 'suspended'
		   AND NOT EXISTS (SELECT 1 FROM banned_identities b WHERE b.kind = 'agent_id' AND b.value = a.id)
	)`

// participantActions is every action family, in report order.
var participantActions = []string{"read", "search", "create", "participate", "room"}

// ParticipantActivityRepository measures monthly active participants from canonical records.
type ParticipantActivityRepository struct {
	pool *Pool
}

// NewParticipantActivityRepository creates the participant counter.
func NewParticipantActivityRepository(pool *Pool) *ParticipantActivityRepository {
	return &ParticipantActivityRepository{pool: pool}
}

// Measure counts the rolling window [end-30d, end) and the window before it, [end-60d, end-30d).
func (r *ParticipantActivityRepository) Measure(ctx context.Context, end time.Time) (growth.ParticipantMeasures, error) {
	end = end.UTC()
	m := growth.ParticipantMeasures{
		WindowEnd:           end,
		WindowStart:         end.Add(-ParticipantWindow),
		PreviousWindowStart: end.Add(-2 * ParticipantWindow),
	}
	if err := r.measureIdentities(ctx, &m); err != nil {
		return growth.ParticipantMeasures{}, err
	}
	if err := r.measureActions(ctx, &m); err != nil {
		return growth.ParticipantMeasures{}, err
	}
	if err := r.measureAnonymousAndTraffic(ctx, &m); err != nil {
		return growth.ParticipantMeasures{}, err
	}
	return m, nil
}

// measureIdentities fills the identity counts of both windows, the returning identities and
// the human/agent overlap of the current window.
func (r *ParticipantActivityRepository) measureIdentities(ctx context.Context, m *growth.ParticipantMeasures) error {
	err := r.pool.QueryRow(ctx, `
		WITH `+qualifyingActivityCTE+`,
		current_ids AS (SELECT DISTINCT actor_type, actor_id FROM eligible WHERE occurred_at >= $4),
		previous_ids AS (SELECT DISTINCT actor_type, actor_id FROM eligible WHERE occurred_at < $4)
		SELECT
			(SELECT COUNT(*) FROM current_ids WHERE actor_type = 'human'),
			(SELECT COUNT(*) FROM current_ids WHERE actor_type = 'agent'),
			(SELECT COUNT(*) FROM previous_ids WHERE actor_type = 'human'),
			(SELECT COUNT(*) FROM previous_ids WHERE actor_type = 'agent'),
			(SELECT COUNT(*) FROM current_ids c JOIN previous_ids p
			    ON p.actor_type = c.actor_type AND p.actor_id = c.actor_id WHERE c.actor_type = 'human'),
			(SELECT COUNT(*) FROM current_ids c JOIN previous_ids p
			    ON p.actor_type = c.actor_type AND p.actor_id = c.actor_id WHERE c.actor_type = 'agent'),
			(SELECT COUNT(*) FROM current_ids c JOIN agents a ON a.id = c.actor_id
			  WHERE c.actor_type = 'agent' AND a.human_id IS NOT NULL
			    AND EXISTS (SELECT 1 FROM current_ids h
			                 WHERE h.actor_type = 'human' AND h.actor_id = a.human_id::text)),
			(SELECT COUNT(*) FROM current_ids c JOIN agents a ON a.id = c.actor_id
			  WHERE c.actor_type = 'agent' AND a.human_id IS NULL)
	`, m.PreviousWindowStart, m.WindowEnd, KnownMonitoringAgents, m.WindowStart).Scan(
		&m.Humans, &m.Agents, &m.PreviousHumans, &m.PreviousAgents,
		&m.ReturningHumans, &m.ReturningAgents, &m.KnownOverlap, &m.UnresolvedOverlap)
	if err != nil {
		LogQueryError(ctx, "ParticipantIdentities", "participant_activity", err)
		return fmt.Errorf("measure participant identities: %w", err)
	}
	return nil
}

// measureActions counts, per action family, the distinct identities of the current window that
// took it. An identity appears under every family it used, so these do not sum to the totals.
func (r *ParticipantActivityRepository) measureActions(ctx context.Context, m *growth.ParticipantMeasures) error {
	rows, err := r.pool.Query(ctx, `
		WITH `+qualifyingActivityCTE+`
		SELECT action,
		       COUNT(DISTINCT actor_id) FILTER (WHERE actor_type = 'human'),
		       COUNT(DISTINCT actor_id) FILTER (WHERE actor_type = 'agent')
		  FROM eligible
		 GROUP BY action
	`, m.WindowStart, m.WindowEnd, KnownMonitoringAgents)
	if err != nil {
		LogQueryError(ctx, "ParticipantActions", "participant_activity", err)
		return fmt.Errorf("measure participant actions: %w", err)
	}
	defer rows.Close()

	byAction := map[string]growth.ActionIdentities{}
	for rows.Next() {
		var a growth.ActionIdentities
		if err := rows.Scan(&a.Action, &a.Humans, &a.Agents); err != nil {
			return fmt.Errorf("scan participant action: %w", err)
		}
		byAction[a.Action] = a
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read participant actions: %w", err)
	}
	m.ByAction = make([]growth.ActionIdentities, 0, len(participantActions))
	for _, action := range participantActions {
		a := byAction[action]
		a.Action = action
		m.ByAction = append(m.ByAction, a)
	}
	return nil
}

// measureAnonymousAndTraffic fills the anonymous events and flows and the traffic figures of the
// current window [$1, $2).
func (r *ParticipantActivityRepository) measureAnonymousAndTraffic(ctx context.Context, m *growth.ParticipantMeasures) error {
	a := &m.Anonymous
	t := &m.Traffic
	err := r.pool.QueryRow(ctx, `
		WITH `+monitoredUserAgentsCTE+`,
		anon_flows AS (
			SELECT flow_id, MIN(occurred_at) AS first_at
			  FROM funnel_events
			 WHERE source_channel = 'browser' AND actor_type = 'anonymous' AND flow_id IS NOT NULL
			   AND occurred_at >= $1 AND occurred_at < $2
			 GROUP BY flow_id
		)
		SELECT
			(SELECT COUNT(*) FROM search_queries
			  WHERE searcher_type = 'anonymous' AND searched_at >= $1 AND searched_at < $2
			    AND COALESCE(user_agent, '') NOT IN (SELECT ua FROM monitored)),
			(SELECT COUNT(*) FROM post_views
			  WHERE viewer_type = 'anonymous' AND viewed_at >= $1 AND viewed_at < $2),
			(SELECT COUNT(*) FROM funnel_events
			  WHERE source_channel = 'browser' AND actor_type = 'anonymous'
			    AND occurred_at >= $1 AND occurred_at < $2),
			(SELECT COUNT(*) FROM anon_flows),
			(SELECT COUNT(*) FROM anon_flows af
			  WHERE EXISTS (SELECT 1 FROM funnel_events s
			                 WHERE s.flow_id = af.flow_id AND s.source_channel = 'server'
			                   AND s.actor_type IN ('agent', 'human') AND s.actor_ref IS NOT NULL
			                   AND s.occurred_at >= af.first_at AND s.occurred_at < $2)),
			(SELECT COUNT(*) FROM api_request_events WHERE occurred_at >= $1 AND occurred_at < $2),
			(SELECT COALESCE(json_object_agg(actor_type, n), '{}'::json) FROM (
				SELECT actor_type, COUNT(*) AS n FROM api_request_events
				 WHERE occurred_at >= $1 AND occurred_at < $2 GROUP BY actor_type) q),
			(SELECT COALESCE(json_object_agg(operation_kind, n), '{}'::json) FROM (
				SELECT operation_kind, COUNT(*) AS n FROM api_request_events
				 WHERE occurred_at >= $1 AND occurred_at < $2 GROUP BY operation_kind) q),
			(SELECT COALESCE(json_object_agg(searcher_type, n), '{}'::json) FROM (
				SELECT searcher_type, COUNT(*) AS n FROM search_queries
				 WHERE searched_at >= $1 AND searched_at < $2
				   AND COALESCE(user_agent, '') NOT IN (SELECT ua FROM monitored)
				 GROUP BY searcher_type) q),
			(SELECT COUNT(*) FROM search_queries
			  WHERE searched_at >= $1 AND searched_at < $2
			    AND COALESCE(user_agent, '') IN (SELECT ua FROM monitored)),
			(SELECT COUNT(*) FROM post_views WHERE viewed_at >= $1 AND viewed_at < $2),
			(SELECT COUNT(DISTINCT room_id) FROM room_entries
			  WHERE kind = 'message' AND author_type IN ('human', 'agent') AND deleted_at IS NULL
			    AND created_at >= $1 AND created_at < $2),
			(SELECT COUNT(*) FROM funnel_events
			  WHERE event_name = 'first_two_way_exchange' AND occurred_at >= $1 AND occurred_at < $2),
			(SELECT COUNT(*) FROM users
			  WHERE deleted_at IS NULL AND created_at >= $1 AND created_at < $2),
			(SELECT COUNT(*) FROM agents
			  WHERE deleted_at IS NULL AND created_at >= $1 AND created_at < $2)
	`, m.WindowStart, m.WindowEnd, KnownMonitoringAgents).Scan(
		&a.Searches, &a.PostViews, &a.BrowserSteps, &a.Flows, &a.FlowsAttributedToIdentity,
		&t.APIRequests, &t.APIRequestsByActor, &t.APIRequestsByKind, &t.SearchesBySearcher,
		&t.MonitoringSearches, &t.PostViewsRecorded, &t.ActiveRooms, &t.Activations,
		&t.HumanRegistrations, &t.AgentRegistrations)
	if err != nil {
		LogQueryError(ctx, "ParticipantAnonymousTraffic", "participant_activity", err)
		return fmt.Errorf("measure anonymous activity and traffic: %w", err)
	}
	return nil
}
