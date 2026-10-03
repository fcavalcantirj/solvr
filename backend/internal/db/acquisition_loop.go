package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/fcavalcantirj/solvr/internal/growth"
)

// Measuring the planner-to-executor acquisition loop (spec.json idx 87).
//
// Everything is read from the connection funnel and the rooms table: example rooms' connection
// evidence (only PUBLIC rooms are ever examples), where each owner's FIRST room stopped (created
// only, a second agent joined but no exchange, activated), creator returns at 7 and 28 days, and
// whether a multi-agent room joined agents of one owner or of several. Owners are resolved the
// same way as the stage gates (stageRoomsCTE): an agent's claiming human, or the agent itself.

// AcquisitionLoopRepository measures the acquisition loop from real tables.
type AcquisitionLoopRepository struct {
	pool *Pool
}

// NewAcquisitionLoopRepository creates the acquisition loop reader.
func NewAcquisitionLoopRepository(pool *Pool) *AcquisitionLoopRepository {
	return &AcquisitionLoopRepository{pool: pool}
}

// Measure reads the loop for the 30 days ending at end, with evidence for each example slug.
func (r *AcquisitionLoopRepository) Measure(ctx context.Context, end time.Time, exampleSlugs []string) (growth.LoopMeasures, error) {
	end = end.UTC()
	m := growth.LoopMeasures{End: end, ExampleRooms: []growth.ExampleRoomEvidence{}}
	for _, slug := range exampleSlugs {
		ev, err := r.exampleRoom(ctx, slug, end)
		if err != nil {
			return growth.LoopMeasures{}, err
		}
		m.ExampleRooms = append(m.ExampleRooms, ev)
	}
	if err := r.measureFirstConnections(ctx, end, &m); err != nil {
		return growth.LoopMeasures{}, err
	}
	if err := r.measureAgentDepth(ctx, end, &m); err != nil {
		return growth.LoopMeasures{}, err
	}
	stages := NewGrowthStageRepository(r.pool)
	for _, w := range []struct {
		days int
		into *growth.ReturnCount
	}{{7, &m.Return7d}, {28, &m.Return28d}} {
		eligible, returned, err := stages.CreatorReturns(ctx, end, w.days)
		if err != nil {
			return growth.LoopMeasures{}, err
		}
		*w.into = growth.ReturnCount{Eligible: eligible, Returned: returned}
	}
	return m, nil
}

// exampleRoom reads one public example room's funnel evidence: the time from its room_created step
// to the second agent joining and to its first two-way exchange. A private, deleted or missing
// room is reported as not found; a room with no room_created step is found but not instrumented.
func (r *AcquisitionLoopRepository) exampleRoom(ctx context.Context, slug string, end time.Time) (growth.ExampleRoomEvidence, error) {
	ev := growth.ExampleRoomEvidence{Slug: slug}
	var created, second, exchange *time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT c.occurred_at,
		       (SELECT MIN(j.occurred_at) FROM funnel_events j
		         WHERE j.room_id = ro.id AND j.event_name = 'participant_joined' AND j.ordinal = 2 AND j.occurred_at < $2),
		       (SELECT MIN(x.occurred_at) FROM funnel_events x
		         WHERE x.room_id = ro.id AND x.event_name = 'first_two_way_exchange' AND x.occurred_at < $2)
		  FROM rooms ro
		  LEFT JOIN LATERAL (
			SELECT f.occurred_at FROM funnel_events f
			 WHERE f.room_id = ro.id AND f.event_name = 'room_created' AND f.occurred_at < $2
			 ORDER BY f.occurred_at LIMIT 1
		  ) c ON true
		 WHERE ro.slug = $1 AND ro.is_private = false AND ro.deleted_at IS NULL
	`, slug, end).Scan(&created, &second, &exchange)
	if errors.Is(err, pgx.ErrNoRows) {
		return ev, nil
	}
	if err != nil {
		LogQueryError(ctx, "AcquisitionLoopExample", "rooms", err)
		return ev, fmt.Errorf("read example room %s: %w", slug, err)
	}
	ev.Found = true
	if created == nil {
		return ev, nil
	}
	ev.Instrumented = true
	if second != nil {
		d := second.Sub(*created)
		ev.TimeToSecondAgent = &d
	}
	if exchange != nil {
		d := exchange.Sub(*created)
		ev.TimeToFirstExchange = &d
	}
	return ev, nil
}

// measureFirstConnections classifies each owner's FIRST room, when it was created in the 30 days
// before end, by where it stopped.
func (r *AcquisitionLoopRepository) measureFirstConnections(ctx context.Context, end time.Time, m *growth.LoopMeasures) error {
	fc := &m.FirstConnections
	err := r.pool.QueryRow(ctx, `
		WITH `+stageRoomsCTE+`,
		firsts AS (
			SELECT DISTINCT ON (owner) owner, room_id, created_at
			  FROM created WHERE owner IS NOT NULL
			 ORDER BY owner, created_at, room_id
		),
		win AS (
			SELECT w.room_id,
			       EXISTS (SELECT 1 FROM activated ac WHERE ac.room_id = w.room_id) AS activated,
			       EXISTS (SELECT 1 FROM funnel_events j
			                WHERE j.room_id = w.room_id AND j.event_name = 'participant_joined'
			                  AND j.ordinal >= 2 AND j.occurred_at < $1) AS second_joined
			  FROM firsts w WHERE w.created_at >= $2
		)
		SELECT COUNT(*) FILTER (WHERE NOT activated AND NOT second_joined),
		       COUNT(*) FILTER (WHERE NOT activated AND second_joined),
		       COUNT(*) FILTER (WHERE activated)
		  FROM win
	`, end, end.Add(-ParticipantWindow)).Scan(&fc.CreatedOnly, &fc.SecondJoinedNoExchange, &fc.Activated)
	if err != nil {
		LogQueryError(ctx, "AcquisitionLoopFirstConnections", "funnel_events", err)
		return fmt.Errorf("measure first connections: %w", err)
	}
	return nil
}

// measureAgentDepth counts, over rooms created in the 30 days before end that two or more distinct
// agents joined, those whose agents all belong to one owner (deeper activation) and those that
// joined several owners, and the distinct owners across them.
func (r *AcquisitionLoopRepository) measureAgentDepth(ctx context.Context, end time.Time, m *growth.LoopMeasures) error {
	err := r.pool.QueryRow(ctx, `
		WITH `+stageRoomsCTE+`,
		parts AS (
			SELECT DISTINCT j.room_id, a.id AS agent_id,
			       CASE WHEN a.human_id IS NOT NULL THEN 'h:' || a.human_id::text ELSE 'a:' || a.id END AS owner
			  FROM funnel_events j
			  JOIN agents a ON j.actor_ref = `+actorRefSQL+`
			 WHERE j.event_name = 'participant_joined' AND j.occurred_at < $1
			   AND j.room_id IN (SELECT room_id FROM created WHERE created_at >= $2)
		),
		multi AS (
			SELECT room_id, COUNT(DISTINCT owner) AS owners
			  FROM parts GROUP BY room_id HAVING COUNT(DISTINCT agent_id) >= 2
		)
		SELECT COUNT(*) FILTER (WHERE owners = 1),
		       COUNT(*) FILTER (WHERE owners >= 2),
		       (SELECT COUNT(DISTINCT owner) FROM parts WHERE room_id IN (SELECT room_id FROM multi))
		  FROM multi
	`, end, end.Add(-ParticipantWindow)).Scan(&m.SameOwnerMultiAgentRooms, &m.CrossOwnerRooms, &m.DistinctOwnersInMultiAgentRooms)
	if err != nil {
		LogQueryError(ctx, "AcquisitionLoopAgentDepth", "funnel_events", err)
		return fmt.Errorf("measure agent depth: %w", err)
	}
	return nil
}
