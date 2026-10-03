package ops

import "github.com/fcavalcantirj/solvr/internal/growth"

// SourceDerived marks a figure computed from the inputs rather than given.
const SourceDerived = "derived"

const secondsPerDay = 86_400.0

// CapacityInputs are the planning model's inputs (spec.json idx 79 step 3). Each carries
// its source: observed (measured) or hypothetical (an assumption). An input with an empty
// source was not provided and contributes nothing.
type CapacityInputs struct {
	MonthlyActiveParticipants   growth.Input `json:"monthly_active_participants"`
	DailyActiveShare            growth.Input `json:"daily_active_share"`
	RequestsPerDailyParticipant growth.Input `json:"requests_per_daily_participant"`

	// Polling, added separately: the share of daily participants that poll while they
	// wait, how often, and for how many hours a day.
	PollingShare        growth.Input `json:"polling_share"`
	PollIntervalSeconds growth.Input `json:"poll_interval_seconds"`
	PollingHoursPerDay  growth.Input `json:"polling_hours_per_day"`

	// Long-lived streams: the share of daily participants holding one and for how long,
	// and the heartbeat interval each held stream costs.
	StreamShare            growth.Input `json:"stream_share"`
	StreamHoursPerDay      growth.Input `json:"stream_hours_per_day"`
	StreamHeartbeatSeconds growth.Input `json:"stream_heartbeat_seconds"`

	// BurstFactor is peak RPS over mean RPS.
	BurstFactor growth.Input `json:"burst_factor"`
	// MeasuredCapacityRPS is the highest load-test stage that met the targets.
	MeasuredCapacityRPS growth.Input `json:"measured_capacity_rps"`
}

// PlanRow is one line of the plan's table.
type PlanRow struct {
	Label  string  `json:"label"`
	Value  float64 `json:"value"`
	Unit   string  `json:"unit"`
	Source string  `json:"source"`
}

// CapacityPlanResult is the planning model's output.
type CapacityPlanResult struct {
	DailyActiveParticipants        float64   `json:"daily_active_participants"`
	BaseRequestsPerDay             float64   `json:"base_requests_per_day"`
	BaseMeanRPS                    float64   `json:"base_mean_rps"`
	PollingRequestsPerDay          float64   `json:"polling_requests_per_day"`
	TotalRequestsPerDay            float64   `json:"total_requests_per_day"`
	MeanRPS                        float64   `json:"mean_rps"`
	PeakRPS                        float64   `json:"peak_rps"`
	MeanConcurrentStreams          float64   `json:"mean_concurrent_streams"`
	PeakConcurrentStreams          float64   `json:"peak_concurrent_streams"`
	StreamHeartbeatFramesPerSecond float64   `json:"stream_heartbeat_frames_per_second"`
	CapacityMultiple               *float64  `json:"capacity_multiple"`
	Rows                           []PlanRow `json:"rows"`
	Note                           string    `json:"note"`
}

// DefaultCapacityInputs are the spec's planning assumptions plus stated hypothetical
// polling, stream and burst inputs; the heartbeat is read from the code (rooms_sse.go).
func DefaultCapacityInputs() CapacityInputs {
	hyp := func(v float64) growth.Input { return growth.Input{Value: v, Source: growth.SourceHypothetical} }
	return CapacityInputs{
		MonthlyActiveParticipants:   hyp(1_000_000),
		DailyActiveShare:            hyp(0.10),
		RequestsPerDailyParticipant: hyp(20),
		PollingShare:                hyp(0.20),
		PollIntervalSeconds:         hyp(30),
		PollingHoursPerDay:          hyp(1),
		StreamShare:                 hyp(0.05),
		StreamHoursPerDay:           hyp(1),
		StreamHeartbeatSeconds:      growth.Input{Value: 30, Source: growth.SourceObserved},
		BurstFactor:                 hyp(3),
	}
}

func given(in growth.Input) bool { return in.Source != "" }

// CapacityPlan turns the inputs into requests per day, mean and peak RPS, stream load and,
// when a measured capacity is given, how many of those units the peak needs. It is a
// planning model, not a forecast.
func CapacityPlan(in CapacityInputs) CapacityPlanResult {
	p := CapacityPlanResult{Note: "planning model, not a forecast: every hypothetical input is an assumption"}
	p.DailyActiveParticipants = in.MonthlyActiveParticipants.Value * in.DailyActiveShare.Value
	p.BaseRequestsPerDay = p.DailyActiveParticipants * in.RequestsPerDailyParticipant.Value
	p.BaseMeanRPS = p.BaseRequestsPerDay / secondsPerDay

	if given(in.PollingShare) && in.PollIntervalSeconds.Value > 0 {
		pollsEach := in.PollingHoursPerDay.Value * 3600 / in.PollIntervalSeconds.Value
		p.PollingRequestsPerDay = p.DailyActiveParticipants * in.PollingShare.Value * pollsEach
	}
	p.TotalRequestsPerDay = p.BaseRequestsPerDay + p.PollingRequestsPerDay
	p.MeanRPS = p.TotalRequestsPerDay / secondsPerDay

	burst := 1.0
	if given(in.BurstFactor) && in.BurstFactor.Value > 0 {
		burst = in.BurstFactor.Value
	}
	p.PeakRPS = p.MeanRPS * burst

	if given(in.StreamShare) {
		p.MeanConcurrentStreams = p.DailyActiveParticipants * in.StreamShare.Value * in.StreamHoursPerDay.Value / 24
		p.PeakConcurrentStreams = p.MeanConcurrentStreams * burst
		if in.StreamHeartbeatSeconds.Value > 0 {
			p.StreamHeartbeatFramesPerSecond = p.MeanConcurrentStreams / in.StreamHeartbeatSeconds.Value
		}
	}

	if given(in.MeasuredCapacityRPS) && in.MeasuredCapacityRPS.Value > 0 {
		m := p.PeakRPS / in.MeasuredCapacityRPS.Value
		p.CapacityMultiple = &m
	}

	row := func(label string, in growth.Input, unit string) PlanRow {
		return PlanRow{Label: label, Value: in.Value, Unit: unit, Source: in.Source}
	}
	derived := func(label string, v float64, unit string) PlanRow {
		return PlanRow{Label: label, Value: v, Unit: unit, Source: SourceDerived}
	}
	p.Rows = append(p.Rows,
		row("Monthly active participants", in.MonthlyActiveParticipants, "participants"),
		row("Daily active share", in.DailyActiveShare, "ratio"),
		row("Requests per daily participant", in.RequestsPerDailyParticipant, "requests"),
		derived("Daily active participants", p.DailyActiveParticipants, "participants"),
		derived("Base requests per day", p.BaseRequestsPerDay, "requests/day"),
		derived("Base mean RPS", p.BaseMeanRPS, "req/s"),
	)
	if given(in.PollingShare) {
		p.Rows = append(p.Rows,
			row("Polling share of daily participants", in.PollingShare, "ratio"),
			row("Poll interval", in.PollIntervalSeconds, "s"),
			row("Polling hours per day", in.PollingHoursPerDay, "h"),
			derived("Polling requests per day", p.PollingRequestsPerDay, "requests/day"))
	}
	p.Rows = append(p.Rows,
		derived("Total requests per day", p.TotalRequestsPerDay, "requests/day"),
		derived("Mean RPS", p.MeanRPS, "req/s"))
	if given(in.BurstFactor) {
		p.Rows = append(p.Rows, row("Burst factor (peak / mean)", in.BurstFactor, "x"))
	}
	p.Rows = append(p.Rows, derived("Peak RPS", p.PeakRPS, "req/s"))
	if given(in.StreamShare) {
		p.Rows = append(p.Rows,
			row("Stream share of daily participants", in.StreamShare, "ratio"),
			row("Stream hours per day", in.StreamHoursPerDay, "h"),
			row("Stream heartbeat interval", in.StreamHeartbeatSeconds, "s"),
			derived("Mean concurrent streams", p.MeanConcurrentStreams, "streams"),
			derived("Peak concurrent streams", p.PeakConcurrentStreams, "streams"),
			derived("Heartbeat frames per second", p.StreamHeartbeatFramesPerSecond, "frames/s"))
	}
	if p.CapacityMultiple != nil {
		p.Rows = append(p.Rows,
			row("Measured capacity (load test)", in.MeasuredCapacityRPS, "req/s"),
			derived("Peak / measured capacity", *p.CapacityMultiple, "x"))
	}
	return p
}
