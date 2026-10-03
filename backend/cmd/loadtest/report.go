package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// LaptopLabel is printed on every report: these are numbers from one development machine
// running the API, PostgreSQL and the harness together, never production capacity.
const LaptopLabel = "LAPTOP"

// The targets the knee is judged against (spec.json idx 79 step 1; internal/ops holds the
// same values for the operator report).
const (
	kneeReadP95Ms     = 500.0
	kneeWriteP95Ms    = 1000.0
	kneeDeliveryP95Ms = 2000.0
	kneeMaxErrorRate  = 0.01
	kneeMinRateShare  = 0.9
)

// Machine describes where the run happened.
type Machine struct {
	Label    string `json:"label"`
	OS       string `json:"os"`
	NumCPU   int    `json:"num_cpu"`
	MemBytes int64  `json:"mem_bytes"`
	Note     string `json:"note"`
}

// RunConfig is what the run was asked to do.
type RunConfig struct {
	BaseURL     string  `json:"base_url"`
	Stages      string  `json:"stages"`
	MaxInFlight int     `json:"max_in_flight"`
	Agents      int     `json:"agents"`
	Rooms       int     `json:"rooms"`
	Streams     int     `json:"streams"`
	Mix         Mix     `json:"mix"`
	MixSource   string  `json:"mix_source"`
	Seed        int64   `json:"seed"`
	Dataset     string  `json:"dataset"`
	WriteShare  float64 `json:"write_share"`
}

// StreamReport is the long-lived streams held open during the run.
type StreamReport struct {
	Requested  int `json:"requested"`
	Opened     int `json:"opened"`
	Failed     int `json:"failed"`
	EndedEarly int `json:"ended_early"`
}

// StageReport is one stage's results.
type StageReport struct {
	TargetRPS     float64        `json:"target_rps"`
	DurationS     float64        `json:"duration_s"`
	AchievedRPS   float64        `json:"achieved_rps"`
	Scheduled     int            `json:"scheduled"`
	Dropped       int            `json:"dropped_by_client"`
	ErrorRate     float64        `json:"error_rate"`
	ReadP95Ms     float64        `json:"read_p95_ms"`
	WriteP95Ms    float64        `json:"write_p95_ms"`
	DeliveryP95Ms float64        `json:"delivery_p95_ms"`
	Classes       []ClassReport  `json:"classes"`
	Delivery      DeliveryReport `json:"delivery"`
	Load          LoadSummary    `json:"load"`
}

// Report is the whole run.
type Report struct {
	StartedAt  time.Time     `json:"started_at"`
	FinishedAt time.Time     `json:"finished_at"`
	Machine    Machine       `json:"machine"`
	Config     RunConfig     `json:"config"`
	Baseline   LoadSummary   `json:"baseline_load"`
	Streams    StreamReport  `json:"streams"`
	Stages     []StageReport `json:"stages"`
	KneeRPS    float64       `json:"knee_rps"`
	StoppedBy  string        `json:"stopped_by,omitempty"`
}

// summariseStage turns a stage's raw results into its report. Read p95 is the worse of the
// two read classes (anonymous overview reads and agent polling).
func summariseStage(st Stage, run StageRun, classes []ClassReport, delivery DeliveryReport, load LoadSummary) StageReport {
	sr := StageReport{TargetRPS: st.TargetRPS, DurationS: st.Duration.Seconds(), Scheduled: run.Scheduled,
		Dropped: run.Dropped, Classes: classes, Delivery: delivery, Load: load,
		ReadP95Ms: -1, WriteP95Ms: -1, DeliveryP95Ms: delivery.P95Ms}
	total, errors := 0, 0
	for _, c := range classes {
		total += c.Count
		errors += c.Errors
		switch c.Class {
		case ClassOverview, ClassPoll:
			sr.ReadP95Ms = math.Max(sr.ReadP95Ms, c.P95Ms)
		case ClassWrite:
			sr.WriteP95Ms = c.P95Ms
		}
	}
	if run.Elapsed > 0 {
		sr.AchievedRPS = float64(total) / run.Elapsed.Seconds()
	}
	if total > 0 {
		sr.ErrorRate = float64(errors+run.Dropped) / float64(total+run.Dropped)
	}
	return sr
}

func meets(v, target float64) bool { return v >= 0 && v < target }

// Knee is the highest target rate whose stage achieved at least 90% of it with under 1%
// errors and every p95 under its target. 0 when no stage did.
func Knee(stages []StageReport) float64 {
	knee := 0.0
	for _, s := range stages {
		if s.AchievedRPS >= kneeMinRateShare*s.TargetRPS && s.ErrorRate < kneeMaxErrorRate &&
			meets(s.ReadP95Ms, kneeReadP95Ms) && meets(s.WriteP95Ms, kneeWriteP95Ms) &&
			meets(s.DeliveryP95Ms, kneeDeliveryP95Ms) && s.TargetRPS > knee {
			knee = s.TargetRPS
		}
	}
	return knee
}

func ms(v float64) string {
	if v < 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f", v)
}

func rate(v float64) string {
	return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%.1f", v), "0"), ".")
}

// Markdown renders the report, labelled LAPTOP.
func (r Report) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Load test — %s numbers, not production capacity\n\n", r.Machine.Label)
	fmt.Fprintf(&b, "- Run: %s → %s\n", r.StartedAt.UTC().Format(time.RFC3339), r.FinishedAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "- Machine: %s, %d CPUs, %.0f GiB RAM. %s\n", r.Machine.OS, r.Machine.NumCPU,
		float64(r.Machine.MemBytes)/(1<<30), r.Machine.Note)
	fmt.Fprintf(&b, "- Dataset: %s\n", r.Config.Dataset)
	fmt.Fprintf(&b, "- Config: stages `%s`, max in flight %d, %d agents in %d public rooms, %d streams, seed %d\n",
		r.Config.Stages, r.Config.MaxInFlight, r.Config.Agents, r.Config.Rooms, r.Config.Streams, r.Config.Seed)
	classes := make([]string, 0, len(r.Config.Mix))
	for c, w := range r.Config.Mix {
		classes = append(classes, fmt.Sprintf("%s %.1f%%", c, 100*w))
	}
	sort.Strings(classes)
	fmt.Fprintf(&b, "- Mix: %s (%s)\n", strings.Join(classes, ", "), r.Config.MixSource)
	fmt.Fprintf(&b, "- Streams: %d requested, %d opened, %d failed, %d ended early\n",
		r.Streams.Requested, r.Streams.Opened, r.Streams.Failed, r.Streams.EndedEarly)
	fmt.Fprintf(&b, "- Baseline load average (1m) before the run: min %.2f, mean %.2f, max %.2f\n",
		r.Baseline.Min, r.Baseline.Mean, r.Baseline.Max)
	fmt.Fprintf(&b, "- Knee (highest stage meeting every target at ≥90%% of its rate, <1%% errors): **%s RPS**\n", rate(r.KneeRPS))
	if r.StoppedBy != "" {
		fmt.Fprintf(&b, "- Stopped early: %s\n", r.StoppedBy)
	}

	b.WriteString("\n| Target RPS | Achieved RPS | Dropped by client | Error rate | Read p95 ms | Write p95 ms | Delivery p95 ms (samples) | Load 1m min/mean/max |\n")
	b.WriteString("|---:|---:|---:|---:|---:|---:|---:|---|\n")
	for _, s := range r.Stages {
		fmt.Fprintf(&b, "| %s | %.1f | %d | %.2f%% | %s | %s | %s (%d) | %.2f / %.2f / %.2f |\n",
			rate(s.TargetRPS), s.AchievedRPS, s.Dropped, 100*s.ErrorRate, ms(s.ReadP95Ms), ms(s.WriteP95Ms),
			ms(s.DeliveryP95Ms), s.Delivery.Samples, s.Load.Min, s.Load.Mean, s.Load.Max)
	}

	b.WriteString("\n| Stage | Class | Count | RPS | p50 ms | p95 ms | p99 ms | max ms | Errors | Statuses |\n")
	b.WriteString("|---:|---|---:|---:|---:|---:|---:|---:|---:|---|\n")
	for _, s := range r.Stages {
		for _, c := range s.Classes {
			var statuses []string
			for code, n := range c.ByStatus {
				statuses = append(statuses, fmt.Sprintf("%d×%d", code, n))
			}
			sort.Strings(statuses)
			if c.TransportErrors > 0 {
				statuses = append(statuses, fmt.Sprintf("transport×%d", c.TransportErrors))
			}
			fmt.Fprintf(&b, "| %s | %s | %d | %.1f | %s | %s | %s | %s | %d | %s |\n", rate(s.TargetRPS), c.Class, c.Count,
				c.RPS, ms(c.P50Ms), ms(c.P95Ms), ms(c.P99Ms), ms(c.MaxMs), c.Errors, strings.Join(statuses, " "))
		}
	}
	return b.String()
}
