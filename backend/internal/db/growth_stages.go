package db

import (
	"context"
	"fmt"
	"time"

	"github.com/fcavalcantirj/solvr/internal/growth"
)

// Measuring the staged growth gates (spec.json idx 89).
//
// Rooms and their owners come from the connection funnel. A room's creator is the agent whose
// pseudonymous actor_ref the room_created step carries (actorRefSQL maps it back); its OWNER is
// the human who claimed that agent, or the agent itself when nobody has, so two agents of one
// person are one owner and a room whose creator cannot be resolved is reported as unknown rather
// than assigned to anybody. Ownership is read as it stands now (agents.human_id), not as it stood
// when the room was created.
//
// Reliability comes from service_checks and moderation load from flags, reports and posts. None
// of these rows carries a message body, a task or a private title, and the report is served only
// behind the operator gate.

// stageRoomsCTE yields created(room_id, created_at, flow_id, owner) and activated(room_id,
// activated_at) for every room created before $1.
const stageRoomsCTE = `
	created AS (
		SELECT DISTINCT ON (f.room_id) f.room_id, f.occurred_at AS created_at, f.flow_id,
		       CASE WHEN a.human_id IS NOT NULL THEN 'h:' || a.human_id::text
		            WHEN a.id IS NOT NULL THEN 'a:' || a.id END AS owner
		  FROM funnel_events f
		  LEFT JOIN agents a ON f.actor_ref = ` + actorRefSQL + `
		 WHERE f.event_name = 'room_created' AND f.room_id IS NOT NULL AND f.occurred_at < $1
		 ORDER BY f.room_id, f.occurred_at
	),
	activated AS (
		SELECT room_id, MIN(occurred_at) AS activated_at
		  FROM funnel_events
		 WHERE event_name = 'first_two_way_exchange' AND room_id IS NOT NULL AND occurred_at < $1
		 GROUP BY room_id
	)`

// coreServices are the services the reliability gate judges; IPFS is reported beside them.
var coreServices = []string{"api", "database"}

// GrowthStageRepository measures the stage gates from real tables.
type GrowthStageRepository struct {
	pool *Pool
}

// NewGrowthStageRepository creates the stage gate reader.
func NewGrowthStageRepository(pool *Pool) *GrowthStageRepository {
	return &GrowthStageRepository{pool: pool}
}

// Measure reads every stage gate for the window ending at end.
func (r *GrowthStageRepository) Measure(ctx context.Context, end time.Time) (growth.StageMeasures, error) {
	end = end.UTC()
	m := growth.StageMeasures{End: end}
	if err := r.measureWeekly(ctx, end, &m); err != nil {
		return growth.StageMeasures{}, err
	}
	if err := r.measureWorkflows(ctx, end, &m); err != nil {
		return growth.StageMeasures{}, err
	}
	if err := r.measureGateA(ctx, end, &m); err != nil {
		return growth.StageMeasures{}, err
	}
	eligible, returned, err := r.CreatorReturns(ctx, end, 7)
	if err != nil {
		return growth.StageMeasures{}, err
	}
	m.GateBEligible, m.GateBReturned = eligible, returned
	if err := r.measureReliability(ctx, end, &m); err != nil {
		return growth.StageMeasures{}, err
	}
	if err := r.measureModeration(ctx, end, &m); err != nil {
		return growth.StageMeasures{}, err
	}
	return m, nil
}

// measureWeekly counts rooms activated in [end-7d, end), their distinct owners, the rooms whose
// creator cannot be resolved, and the most rooms any single owner has.
func (r *GrowthStageRepository) measureWeekly(ctx context.Context, end time.Time, m *growth.StageMeasures) error {
	err := r.pool.QueryRow(ctx, `
		WITH `+stageRoomsCTE+`,
		weekly AS (
			SELECT ac.room_id, c.owner
			  FROM activated ac LEFT JOIN created c ON c.room_id = ac.room_id
			 WHERE ac.activated_at >= $2
		)
		SELECT
			(SELECT COUNT(*) FROM weekly),
			(SELECT COUNT(DISTINCT owner) FROM weekly),
			(SELECT COUNT(*) FROM weekly WHERE owner IS NULL),
			(SELECT COALESCE(MAX(n), 0) FROM (
				SELECT COUNT(*) AS n FROM weekly WHERE owner IS NOT NULL GROUP BY owner) q)
	`, end, end.Add(-7*24*time.Hour)).Scan(
		&m.WeeklyActivatedRooms, &m.WeeklyOwners, &m.WeeklyUnknownOwnerRooms, &m.LargestOwnerRooms)
	if err != nil {
		LogQueryError(ctx, "GrowthStagesWeekly", "funnel_events", err)
		return fmt.Errorf("measure weekly activated rooms: %w", err)
	}
	return nil
}

// measureWorkflows groups this week's activated rooms by the connect preset their flow chose,
// most first; a room with no flow or no preset is "unknown".
func (r *GrowthStageRepository) measureWorkflows(ctx context.Context, end time.Time, m *growth.StageMeasures) error {
	rows, err := r.pool.Query(ctx, `
		WITH `+stageRoomsCTE+`,
		weekly AS (
			SELECT ac.room_id, c.flow_id
			  FROM activated ac LEFT JOIN created c ON c.room_id = ac.room_id
			 WHERE ac.activated_at >= $2
		)
		SELECT preset, COUNT(*) AS rooms FROM (
			SELECT COALESCE((SELECT p.preset FROM funnel_events p
			                  WHERE w.flow_id IS NOT NULL AND p.flow_id = w.flow_id AND p.preset IS NOT NULL
			                  ORDER BY p.occurred_at LIMIT 1), 'unknown') AS preset
			  FROM weekly w
		) q
		GROUP BY preset
		ORDER BY rooms DESC, preset
	`, end, end.Add(-7*24*time.Hour))
	if err != nil {
		LogQueryError(ctx, "GrowthStagesWorkflows", "funnel_events", err)
		return fmt.Errorf("measure workflows: %w", err)
	}
	defer rows.Close()
	m.Workflows = []growth.WorkflowCount{}
	for rows.Next() {
		var w growth.WorkflowCount
		if err := rows.Scan(&w.Preset, &w.ActivatedRooms); err != nil {
			return fmt.Errorf("scan workflow: %w", err)
		}
		m.Workflows = append(m.Workflows, w)
	}
	return rows.Err()
}

// measureGateA counts rooms created in [end-24h-30d, end-24h) and those that reached a two-way
// exchange within 24 hours of creation. A room younger than 24 hours is not yet judgeable.
func (r *GrowthStageRepository) measureGateA(ctx context.Context, end time.Time, m *growth.StageMeasures) error {
	cutoff := end.Add(-24 * time.Hour)
	err := r.pool.QueryRow(ctx, `
		WITH `+stageRoomsCTE+`
		SELECT COUNT(*),
		       COUNT(*) FILTER (WHERE ac.activated_at <= c.created_at + interval '24 hours')
		  FROM created c LEFT JOIN activated ac ON ac.room_id = c.room_id
		 WHERE c.created_at >= $2 AND c.created_at < $3
	`, end, cutoff.Add(-ParticipantWindow), cutoff).Scan(&m.GateAEligible, &m.GateAConverted)
	if err != nil {
		LogQueryError(ctx, "GrowthStagesGateA", "funnel_events", err)
		return fmt.Errorf("measure gate A: %w", err)
	}
	return nil
}

// CreatorReturns counts the owners whose FIRST room was created in the 30 days ending `days` days
// before end, and those of them who created another room within `days` days of that first room
// which reached a two-way exchange — a new real task, not a retry or an abandoned room.
func (r *GrowthStageRepository) CreatorReturns(ctx context.Context, end time.Time, days int) (int, int, error) {
	end = end.UTC()
	horizon := time.Duration(days) * 24 * time.Hour
	var eligible, returned int
	err := r.pool.QueryRow(ctx, `
		WITH `+stageRoomsCTE+`,
		firsts AS (
			SELECT owner, MIN(created_at) AS first_at
			  FROM created WHERE owner IS NOT NULL GROUP BY owner
		),
		cohort AS (
			SELECT owner, first_at FROM firsts WHERE first_at >= $2 AND first_at < $3
		)
		SELECT COUNT(*),
		       COUNT(*) FILTER (WHERE EXISTS (
		           SELECT 1 FROM created c JOIN activated ac ON ac.room_id = c.room_id
		            WHERE c.owner = h.owner
		              AND c.created_at > h.first_at
		              AND c.created_at <= h.first_at + make_interval(days => $4)))
		  FROM cohort h
	`, end, end.Add(-horizon-ParticipantWindow), end.Add(-horizon), days).Scan(&eligible, &returned)
	if err != nil {
		LogQueryError(ctx, "GrowthStagesCreatorReturns", "funnel_events", err)
		return 0, 0, fmt.Errorf("measure creator returns (%d days): %w", days, err)
	}
	return eligible, returned, nil
}

// measureReliability reads every service's checks in [end-30d, end), and the operational share
// of the core services.
func (r *GrowthStageRepository) measureReliability(ctx context.Context, end time.Time, m *growth.StageMeasures) error {
	rows, err := r.pool.Query(ctx, `
		SELECT service_name, COUNT(*), COUNT(*) FILTER (WHERE status = 'operational')
		  FROM service_checks
		 WHERE checked_at >= $1 AND checked_at < $2
		 GROUP BY service_name
		 ORDER BY service_name
	`, end.Add(-ParticipantWindow), end)
	if err != nil {
		LogQueryError(ctx, "GrowthStagesReliability", "service_checks", err)
		return fmt.Errorf("measure reliability: %w", err)
	}
	defer rows.Close()
	core := map[string]bool{}
	for _, s := range coreServices {
		core[s] = true
	}
	m.Services = []growth.ServiceUptime{}
	for rows.Next() {
		var s growth.ServiceUptime
		if err := rows.Scan(&s.Service, &s.Checks, &s.Operational); err != nil {
			return fmt.Errorf("scan service uptime: %w", err)
		}
		m.Services = append(m.Services, s)
		if core[s.Service] {
			m.CoreChecks += s.Checks
			m.CoreOperational += s.Operational
		}
	}
	return rows.Err()
}

// measureModeration counts flags and reports created in [end-30d, end) and the current backlog:
// pending flags, reports and posts awaiting moderation created more than 7 days before end.
func (r *GrowthStageRepository) measureModeration(ctx context.Context, end time.Time, m *growth.StageMeasures) error {
	from, stale := end.Add(-ParticipantWindow), end.Add(-7*24*time.Hour)
	err := r.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM flags WHERE created_at >= $1 AND created_at < $2)
			+ (SELECT COUNT(*) FROM reports WHERE created_at >= $1 AND created_at < $2),
			(SELECT COUNT(*) FROM flags WHERE status = 'pending' AND created_at < $3)
			+ (SELECT COUNT(*) FROM reports WHERE status = 'pending' AND created_at < $3)
			+ (SELECT COUNT(*) FROM posts
			    WHERE moderation_state = 'pending' AND deleted_at IS NULL AND created_at < $3)
	`, from, end, stale).Scan(&m.ModerationVolume, &m.ModerationBacklog)
	if err != nil {
		LogQueryError(ctx, "GrowthStagesModeration", "flags", err)
		return fmt.Errorf("measure moderation load: %w", err)
	}
	return nil
}
