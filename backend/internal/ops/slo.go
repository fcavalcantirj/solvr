// Package ops holds Solvr's reliability, capacity and cost gates (spec.json idx 79):
// the service-level targets, the computations that judge them from what the service
// recorded, the capacity planning model and the cost calculator. Everything here is
// pure; the database layer supplies observations and the operator report renders them.
package ops

import (
	"fmt"
	"sort"
	"time"

	"github.com/fcavalcantirj/solvr/internal/growth"
)

// The initial targets spec.json idx 79 step 1 adopts.
const (
	// TargetAvailabilityPercent is monthly core-API availability.
	TargetAvailabilityPercent = 99.9
	// TargetReadP95Ms is the p95 of an ordinary read: p95 must be strictly below it.
	TargetReadP95Ms = 500.0
	// TargetTimelineWriteP95Ms is the p95 of an accepted timeline write.
	TargetTimelineWriteP95Ms = 1000.0
	// TargetDeliveryP95Ms is the p95 from an accepted write to a connected client.
	TargetDeliveryP95Ms = 2000.0
)

const (
	// AvailabilityWindow is the "monthly" window availability is judged over.
	AvailabilityWindow = 30 * 24 * time.Hour
	// CheckInterval is how often HealthCheckJob records a check per service
	// (jobs.DefaultHealthCheckInterval; the jobs package test pins the two equal).
	CheckInterval = 5 * time.Minute
	// GapTolerance is how many intervals may separate two checks of one service
	// before the silence counts as downtime. Below it is scheduling jitter.
	GapTolerance = 1.5
	// MinLatencySamples is the fewest measured requests a p95 is judged on.
	MinLatencySamples = 100
	// QueueLagAlarmThreshold is how long a due delivery may wait before it alarms.
	QueueLagAlarmThreshold = 5 * time.Minute
)

// CoreServices are the service_checks services core-API availability is made of,
// and the only services the HealthCheckJob checks (there is no IPFS node any more).
var CoreServices = []string{"api", "database"}

// Queue statuses.
const (
	QueueOK    = "ok"
	QueueAlarm = "alarm"
)

// CoreCheck is one service_checks row.
type CoreCheck struct {
	Service   string    `json:"service"`
	Status    string    `json:"status"`
	CheckedAt time.Time `json:"checked_at"`
}

// AvailabilityResult is core-API availability over one window.
type AvailabilityResult struct {
	Percent         *float64   `json:"percent"`
	DowntimeSeconds float64    `json:"downtime_seconds"`
	Gaps            int        `json:"gaps"`
	OutageIntervals int        `json:"outage_intervals"`
	Checks          int        `json:"checks"`
	HistoryStart    *time.Time `json:"history_start"`
	Status          string     `json:"status"`
	Note            string     `json:"note"`
}

type span struct{ from, to time.Time }

// Availability judges core-API availability over [start, end] from service checks.
//
// Downtime is the union of: every interval a core service checked as outage (a
// check covers the interval until the next one), and every silence between two
// checks of one service longer than GapTolerance intervals (nothing answered the
// checker, or the checker itself was down). Degraded counts as available. The
// window must be fully covered by history to judge a monthly target; a shorter
// history reports its observed span as not_yet_measurable.
func Availability(checks []CoreCheck, start, end time.Time, interval time.Duration) AvailabilityResult {
	core := map[string]bool{}
	for _, s := range CoreServices {
		core[s] = true
	}
	byService := map[string][]CoreCheck{}
	for _, c := range checks {
		if core[c.Service] && !c.CheckedAt.After(end) {
			byService[c.Service] = append(byService[c.Service], c)
		}
	}

	res := AvailabilityResult{}
	var historyStart time.Time
	for _, s := range CoreServices {
		cs := byService[s]
		if len(cs) == 0 {
			res.Status = growth.StatusNotYetMeasurable
			res.Note = fmt.Sprintf("no %s checks recorded in the window", s)
			return res
		}
		sort.Slice(cs, func(i, j int) bool { return cs[i].CheckedAt.Before(cs[j].CheckedAt) })
		byService[s] = cs
		res.Checks += len(cs)
		if historyStart.IsZero() || cs[0].CheckedAt.After(historyStart) {
			historyStart = cs[0].CheckedAt // the LATEST first check: every core service observed from here
		}
	}

	tolerance := time.Duration(GapTolerance * float64(interval))
	observedFrom := start
	full := !historyStart.After(start.Add(tolerance))
	if !full {
		observedFrom = historyStart
	}
	hs := historyStart
	res.HistoryStart = &hs

	var down, gaps []span
	outageInstants := map[int64]bool{}
	for _, s := range CoreServices {
		cs := byService[s]
		for i, c := range cs {
			next := end
			if i+1 < len(cs) {
				next = cs[i+1].CheckedAt
			}
			covered := c.CheckedAt.Add(interval)
			if covered.After(next) {
				covered = next
			}
			if c.Status == "outage" {
				down = append(down, span{c.CheckedAt, covered})
				outageInstants[c.CheckedAt.Truncate(interval).Unix()] = true
			}
			if next.Sub(c.CheckedAt) > tolerance {
				gaps = append(gaps, span{c.CheckedAt.Add(interval), next})
			}
		}
	}
	// Services silent at the same time are one gap.
	res.Gaps = len(mergeSpans(clipSpans(gaps, observedFrom, end)))
	down = append(down, gaps...)
	res.OutageIntervals = len(outageInstants)
	res.DowntimeSeconds = unionSeconds(down, observedFrom, end)

	observed := end.Sub(observedFrom).Seconds()
	if observed <= 0 {
		res.Status = growth.StatusNotYetMeasurable
		res.Note = "the window has no observed span"
		return res
	}
	pct := 100 * (1 - res.DowntimeSeconds/observed)
	res.Percent = &pct

	switch {
	case !full:
		res.Status = growth.StatusNotYetMeasurable
		res.Note = fmt.Sprintf("checks begin %s, inside the %d-day window: the observed span is reported, the monthly target is not judged",
			historyStart.UTC().Format(time.RFC3339), int(AvailabilityWindow.Hours()/24))
	case pct >= TargetAvailabilityPercent:
		res.Status = growth.StatusMet
	default:
		res.Status = growth.StatusUnmet
	}
	return res
}

// unionSeconds is the total length of the union of spans, clipped to [from, to].
func unionSeconds(spans []span, from, to time.Time) float64 {
	var total time.Duration
	for _, s := range mergeSpans(clipSpans(spans, from, to)) {
		total += s.to.Sub(s.from)
	}
	return total.Seconds()
}

// clipSpans clips spans to [from, to] and drops the empty ones.
func clipSpans(spans []span, from, to time.Time) []span {
	var clipped []span
	for _, s := range spans {
		if s.from.Before(from) {
			s.from = from
		}
		if s.to.After(to) {
			s.to = to
		}
		if s.to.After(s.from) {
			clipped = append(clipped, s)
		}
	}
	return clipped
}

// mergeSpans returns the union of spans as disjoint spans, oldest first.
func mergeSpans(spans []span) []span {
	sorted := append([]span(nil), spans...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].from.Before(sorted[j].from) })
	var merged []span
	for _, s := range sorted {
		if n := len(merged); n > 0 && !s.from.After(merged[n-1].to) {
			if s.to.After(merged[n-1].to) {
				merged[n-1].to = s.to
			}
			continue
		}
		merged = append(merged, s)
	}
	return merged
}

// LatencyObservation is a measured p95 and how many requests it is computed from.
type LatencyObservation struct {
	P95Ms   *float64 `json:"p95_ms"`
	Samples int      `json:"samples"`
}

// Target is one gate in the report.
type Target struct {
	Key       string   `json:"key"`
	Label     string   `json:"label"`
	Objective string   `json:"objective"`
	Threshold float64  `json:"threshold"`
	Unit      string   `json:"unit"`
	Measured  *float64 `json:"measured"`
	Samples   int      `json:"samples"`
	Status    string   `json:"status"`
	Source    string   `json:"source"`
	Missing   string   `json:"missing,omitempty"`
	Note      string   `json:"note,omitempty"`
}

// EvaluateP95 judges a latency target: met only when p95 is strictly below the
// target on at least MinLatencySamples measured requests.
func EvaluateP95(key, label string, targetMs float64, obs LatencyObservation, source string) Target {
	t := Target{
		Key: key, Label: label, Objective: fmt.Sprintf("p95 < %.0f ms", targetMs), Threshold: targetMs,
		Unit: "ms", Measured: obs.P95Ms, Samples: obs.Samples, Source: source,
	}
	switch {
	case obs.P95Ms == nil || obs.Samples < MinLatencySamples:
		t.Status = growth.StatusNotYetMeasurable
		t.Missing = fmt.Sprintf("%d measured requests in the window; at least %d are needed to judge a p95",
			obs.Samples, MinLatencySamples)
	case *obs.P95Ms < targetMs:
		t.Status = growth.StatusMet
	default:
		t.Status = growth.StatusUnmet
	}
	return t
}

// QueueObservation is the backlog of one delivery queue.
type QueueObservation struct {
	Pending       int        `json:"pending"`
	Due           int        `json:"due"`
	OldestDue     *time.Time `json:"oldest_due"`
	FailedLast24h int        `json:"failed_last_24h"`
}

// QueueLag is one queue's lag and its alarm state.
type QueueLag struct {
	Queue               string   `json:"queue"`
	Pending             int      `json:"pending"`
	Due                 int      `json:"due"`
	OldestDueAgeSeconds *float64 `json:"oldest_due_age_seconds"`
	FailedLast24h       int      `json:"failed_last_24h"`
	ThresholdSeconds    float64  `json:"threshold_seconds"`
	Status              string   `json:"status"`
	Definition          string   `json:"definition"`
}

// EvaluateQueueLag alarms when the oldest due delivery has waited longer than
// QueueLagAlarmThreshold. A delivery scheduled for a later retry is pending but
// not yet due, so a slow subscriber does not alarm the worker.
func EvaluateQueueLag(queue string, obs QueueObservation, now time.Time) QueueLag {
	q := QueueLag{
		Queue: queue, Pending: obs.Pending, Due: obs.Due, FailedLast24h: obs.FailedLast24h,
		ThresholdSeconds: QueueLagAlarmThreshold.Seconds(), Status: QueueOK,
		Definition: "age of the oldest pending delivery whose next attempt is due",
	}
	if obs.OldestDue != nil {
		age := now.Sub(*obs.OldestDue).Seconds()
		if age < 0 {
			age = 0
		}
		q.OldestDueAgeSeconds = &age
		if age > q.ThresholdSeconds {
			q.Status = QueueAlarm
		}
	}
	return q
}

// SLOObservations is everything the report is computed from.
type SLOObservations struct {
	WindowStart   time.Time
	WindowEnd     time.Time
	Now           time.Time
	CoreChecks    []CoreCheck
	Read          LatencyObservation
	TimelineWrite LatencyObservation
	Search        LatencyObservation
	Webhooks      QueueObservation
}

// SLOReport is GET /admin/ops/slo.
type SLOReport struct {
	GeneratedAt   time.Time          `json:"generated_at"`
	WindowStart   time.Time          `json:"window_start"`
	WindowEnd     time.Time          `json:"window_end"`
	Targets       []Target           `json:"targets"`
	Availability  AvailabilityResult `json:"availability"`
	ExternalModel Target             `json:"external_model"`
	Queues        []QueueLag         `json:"queues"`
	Missing       []string           `json:"missing"`
	UAT           []string           `json:"uat"`
}

// Sources, as the report names them.
const (
	sourceAvailability = "service_checks: api and database, one check per service every 5 minutes (HealthCheckJob)"
	sourceRead         = "api_request_events.duration_ms: GET requests (operation_kind poll) answered 2xx; server-side time; search, streams, heartbeats, health, admin and the homepage's own reads are not recorded"
	sourceWrite        = "api_request_events.duration_ms: POST /v1/rooms/{slug}/entries, /v1/rooms/{slug}/messages, /r/{slug}/message and /r/{slug}/events answered 2xx; server-side time"
	sourceSearch       = "search_queries.duration_ms: every search, including the external embedding call when one is configured"
)

// BuildSLOReport computes the four targets, the separately measured external-model
// latency and the queue lag from the observations.
func BuildSLOReport(obs SLOObservations) SLOReport {
	avail := Availability(obs.CoreChecks, obs.WindowStart, obs.WindowEnd, CheckInterval)
	availability := Target{
		Key: "core_api_availability", Label: "Monthly core-API availability",
		Objective: fmt.Sprintf(">= %.1f%% over %d days", TargetAvailabilityPercent, int(AvailabilityWindow.Hours()/24)),
		Threshold: TargetAvailabilityPercent, Unit: "percent", Measured: avail.Percent, Samples: avail.Checks,
		Status: avail.Status, Source: sourceAvailability,
		Note: "self-measured from inside the API process: an outage of the edge, DNS or TLS is invisible here; external measurement is UAT",
	}
	if avail.Note != "" {
		availability.Missing = avail.Note
	}

	delivery := Target{
		Key: "delivery_p95", Label: "p95 connected-client delivery",
		Objective: fmt.Sprintf("p95 < %.0f ms", TargetDeliveryP95Ms), Threshold: TargetDeliveryP95Ms, Unit: "ms",
		Status: growth.StatusNotYetMeasurable,
		Missing: "nothing records when the room relay delivers an entry to a connected client stream; " +
			"delivery latency is measured only by the load harness (cmd/loadtest), on a laptop",
	}

	search := Target{
		Key: "search_p95", Label: "p95 search, including the external embedding call",
		Unit: "ms", Measured: obs.Search.P95Ms, Samples: obs.Search.Samples, Source: sourceSearch,
		Note: "external model latency is measured separately from the targets and carries none of its own",
	}

	return SLOReport{
		GeneratedAt:  obs.Now,
		WindowStart:  obs.WindowStart,
		WindowEnd:    obs.WindowEnd,
		Availability: avail,
		Targets: []Target{
			availability,
			EvaluateP95("read_p95", "p95 ordinary read", TargetReadP95Ms, obs.Read, sourceRead),
			EvaluateP95("timeline_write_p95", "p95 accepted timeline write", TargetTimelineWriteP95Ms, obs.TimelineWrite, sourceWrite),
			delivery,
		},
		ExternalModel: search,
		Queues:        []QueueLag{EvaluateQueueLag("webhook_deliveries", obs.Webhooks, obs.Now)},
		Missing: []string{
			"connected-client delivery time: no record of when an entry reaches a stream",
			"notifications: in-app rows with no delivery tracking, so they have no lag to measure",
			"request latency before migration 000139: never measured, never estimated",
			"external availability: the API cannot observe its own edge",
		},
		UAT: []string{
			"choose and connect an external uptime monitor for the public API",
			"page someone when /admin/ops/slo reports a queue alarm or an unmet target",
		},
	}
}
