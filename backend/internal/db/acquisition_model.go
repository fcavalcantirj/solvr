package db

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/fcavalcantirj/solvr/internal/growth"
)

// Observed monthly flows for the acquisition model (spec.json idx 90).
//
// Activity is the participant counter's qualifying-action definition (qualifyingActivityCTE), read
// over all history up to the end of the month so "new" means the first qualifying action EVER.
// For one calendar month (UTC) every active identity lands in exactly one bucket:
//
//	retained     active this month and last month
//	new          first qualifying action ever is this month
//	reactivated  active this month, not last month, active some month before
//
// so active = retained + new + reactivated per population, and no identity is counted twice. The
// monthly cohorts (identities grouped by their first month) give the survival curve that replaces
// the single retention term. Every read is a full-history scan of the activity sources; it runs
// only for the operator's monthly review.

// modelCohortMonths is how many monthly cohorts, ending with the month itself, the model reads.
const modelCohortMonths = 6

// activityHistoryStart predates every record, so the activity CTE reads all history.
var activityHistoryStart = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// monthlyIdentitiesCTE extends qualifyingActivityCTE with each identity's first month and whether
// it was active in the month $4 and the month before, $5.
const monthlyIdentitiesCTE = qualifyingActivityCTE + `,
	months AS (
		SELECT DISTINCT actor_type, actor_id, date_trunc('month', occurred_at, 'UTC') AS m FROM eligible
	),
	per_identity AS (
		SELECT actor_type, actor_id, MIN(m) AS first_m, bool_or(m = $4) AS in_m, bool_or(m = $5) AS in_prev
		  FROM months GROUP BY actor_type, actor_id
	)`

// AcquisitionModelRepository reads observed monthly flows from real tables.
type AcquisitionModelRepository struct {
	pool *Pool
}

// NewAcquisitionModelRepository creates the monthly flow reader.
func NewAcquisitionModelRepository(pool *Pool) *AcquisitionModelRepository {
	return &AcquisitionModelRepository{pool: pool}
}

// MonthlyFlows reads the calendar month (UTC) that contains monthStart.
func (r *AcquisitionModelRepository) MonthlyFlows(ctx context.Context, monthStart time.Time) (growth.MonthlyFlows, error) {
	u := monthStart.UTC()
	start := time.Date(u.Year(), u.Month(), 1, 0, 0, 0, 0, time.UTC)
	f := growth.MonthlyFlows{Month: start.Format("2006-01"), Start: start, End: start.AddDate(0, 1, 0)}
	args := []any{activityHistoryStart, f.End, KnownMonitoringAgents, start, start.AddDate(0, -1, 0)}

	if err := r.readBuckets(ctx, args, &f); err != nil {
		return growth.MonthlyFlows{}, err
	}
	if err := r.readCohorts(ctx, args, start, &f); err != nil {
		return growth.MonthlyFlows{}, err
	}
	err := r.pool.QueryRow(ctx, `
		WITH `+monthlyIdentitiesCTE+`
		SELECT COUNT(*)
		  FROM per_identity ag JOIN agents a ON a.id = ag.actor_id
		 WHERE ag.actor_type = 'agent' AND ag.in_m AND a.human_id IS NOT NULL
		   AND EXISTS (SELECT 1 FROM per_identity h
		                WHERE h.actor_type = 'human' AND h.in_m AND h.actor_id = a.human_id::text)
	`, args...).Scan(&f.KnownOverlap)
	if err != nil {
		LogQueryError(ctx, "AcquisitionModelOverlap", "participant_activity", err)
		return growth.MonthlyFlows{}, fmt.Errorf("measure monthly overlap: %w", err)
	}
	return f, nil
}

// readBuckets fills each population's active, retained, new, reactivated and previous-month counts.
func (r *AcquisitionModelRepository) readBuckets(ctx context.Context, args []any, f *growth.MonthlyFlows) error {
	rows, err := r.pool.Query(ctx, `
		WITH `+monthlyIdentitiesCTE+`
		SELECT actor_type,
		       COUNT(*) FILTER (WHERE in_m),
		       COUNT(*) FILTER (WHERE in_m AND in_prev),
		       COUNT(*) FILTER (WHERE in_m AND first_m = $4),
		       COUNT(*) FILTER (WHERE in_m AND NOT in_prev AND first_m < $4),
		       COUNT(*) FILTER (WHERE in_prev)
		  FROM per_identity
		 GROUP BY actor_type
	`, args...)
	if err != nil {
		LogQueryError(ctx, "AcquisitionModelBuckets", "participant_activity", err)
		return fmt.Errorf("measure monthly flows: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var actorType string
		var p growth.PopulationFlows
		if err := rows.Scan(&actorType, &p.Active, &p.Retained, &p.New, &p.Reactivated, &p.PreviousActive); err != nil {
			return fmt.Errorf("scan monthly flows: %w", err)
		}
		if p.PreviousActive > 0 {
			rate := float64(p.Retained) / float64(p.PreviousActive)
			p.MeasuredRetention = &rate
		}
		switch actorType {
		case "human":
			f.Humans = p
		case "agent":
			f.Agents = p
		}
	}
	return rows.Err()
}

// readCohorts fills each population's monthly cohorts: for the cohorts whose first month is among
// the last modelCohortMonths months, the identities active at each age up to the month itself.
func (r *AcquisitionModelRepository) readCohorts(ctx context.Context, args []any, month time.Time, f *growth.MonthlyFlows) error {
	from := month.AddDate(0, -(modelCohortMonths - 1), 0)
	rows, err := r.pool.Query(ctx, `
		WITH `+monthlyIdentitiesCTE+`
		SELECT mo.actor_type, p.first_m, mo.m, COUNT(*)
		  FROM months mo
		  JOIN per_identity p ON p.actor_type = mo.actor_type AND p.actor_id = mo.actor_id
		 WHERE p.first_m >= $6 AND mo.m <= $4
		 GROUP BY mo.actor_type, p.first_m, mo.m
	`, append(args, from)...)
	if err != nil {
		LogQueryError(ctx, "AcquisitionModelCohorts", "participant_activity", err)
		return fmt.Errorf("measure monthly cohorts: %w", err)
	}
	defer rows.Close()

	byType := map[string]map[time.Time][]int{"human": {}, "agent": {}}
	for rows.Next() {
		var actorType string
		var first, m time.Time
		var n int
		if err := rows.Scan(&actorType, &first, &m, &n); err != nil {
			return fmt.Errorf("scan monthly cohort: %w", err)
		}
		first, m = first.UTC(), m.UTC()
		cohorts, ok := byType[actorType]
		if !ok {
			continue
		}
		if cohorts[first] == nil {
			cohorts[first] = make([]int, monthsBetween(first, month)+1)
		}
		cohorts[first][monthsBetween(first, m)] = n
	}
	if err := rows.Err(); err != nil {
		return err
	}
	f.Humans.Cohorts = cohortObservations(byType["human"])
	f.Agents.Cohorts = cohortObservations(byType["agent"])
	return nil
}

// monthsBetween is the whole calendar months from a to b.
func monthsBetween(a, b time.Time) int {
	return (b.Year()-a.Year())*12 + int(b.Month()) - int(a.Month())
}

// cohortObservations orders cohorts by their first month.
func cohortObservations(cohorts map[time.Time][]int) []growth.CohortObservation {
	months := make([]time.Time, 0, len(cohorts))
	for m := range cohorts {
		months = append(months, m)
	}
	sort.Slice(months, func(i, j int) bool { return months[i].Before(months[j]) })
	out := make([]growth.CohortObservation, 0, len(months))
	for _, m := range months {
		out = append(out, growth.CohortObservation{CohortMonth: m.Format("2006-01"), ActiveByAge: cohorts[m]})
	}
	return out
}
