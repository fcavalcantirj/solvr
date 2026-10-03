package growth

import (
	"fmt"
	"strings"
	"time"
)

// Stage targets (spec.json idx 89).
const (
	StageOneWeeklyActivatedRooms = 100
	StageTwoParticipants         = 10_000
	StageThreeParticipants       = 100_000
)

// WorkflowCount is how many activated rooms one connect preset produced.
type WorkflowCount struct {
	Preset         string `json:"preset"`
	ActivatedRooms int    `json:"activated_rooms"`
}

// ServiceUptime is one service's health checks over the window.
type ServiceUptime struct {
	Service     string `json:"service"`
	Checks      int    `json:"checks"`
	Operational int    `json:"operational"`
}

// StageMeasures is the raw measurement the stage gates read.
type StageMeasures struct {
	End                     time.Time
	WeeklyActivatedRooms    int
	WeeklyOwners            int
	WeeklyUnknownOwnerRooms int
	LargestOwnerRooms       int
	Workflows               []WorkflowCount
	GateAEligible           int
	GateAConverted          int
	GateBEligible           int
	GateBReturned           int
	CoreChecks              int
	CoreOperational         int
	Services                []ServiceUptime
	ModerationVolume        int
	ModerationBacklog       int
	Source                  SourceMeasures
}

// TargetInputs is the participant counts a participant gate reads.
type TargetInputs struct {
	Current   int
	Previous  int
	Returning int
}

// Gate is one separately verifiable check.
type Gate struct {
	Key               string   `json:"key"`
	Description       string   `json:"description"`
	Threshold         string   `json:"threshold"`
	ProposedThreshold bool     `json:"proposed_threshold"`
	Measured          *float64 `json:"measured"`
	Sample            *int     `json:"sample"`
	MinSample         *int     `json:"min_sample"`
	ProposedMinSample bool     `json:"proposed_min_sample"`
	Status            string   `json:"status"`
	Evidence          string   `json:"evidence"`
}

// Stage is one growth stage and its gates.
type Stage struct {
	Number   int        `json:"number"`
	Name     string     `json:"name"`
	Target   string     `json:"target"`
	Status   string     `json:"status"`
	Gates    []Gate     `json:"gates"`
	Deadline *time.Time `json:"deadline"`
	Budget   *float64   `json:"budget"`
}

// StageReport is GET /admin/growth/stages.
type StageReport struct {
	End       time.Time       `json:"end"`
	Stages    []Stage         `json:"stages"`
	Workflows []WorkflowCount `json:"workflows"`
	Services  []ServiceUptime `json:"services"`
	Note      string          `json:"note"`
	Privacy   string          `json:"privacy"`
}

// Proposed thresholds. spec.json idx 89 names the 60% and 25% rates and the 100-creator sample;
// the values below go beyond it and are labeled proposed in every gate that uses them.
const (
	proposedMinIndependentOwners = 2
	proposedGateAMinRooms        = 100
	proposedCoreUptime           = 0.995
	gateARate                    = 0.60
	gateBRate                    = 0.25
	gateBMinCreators             = 100
)

const (
	stagesNote = "Stage targets and thresholds are proposed planning targets, not current measured performance. " +
		"A stage is met only when every gate is met and the stage before it is met. Shipping the website never " +
		"meets a gate. No deadline or budget is set here: dates and budgets come from observed growth and live in " +
		"the private operator plan, which also records every unmet target."
	ownerObservation = "An owner observation (UAT), never auto-passed by the system."
)

func intPtr(v int) *int           { return &v }
func floatPtr(v float64) *float64 { return &v }

// RateGate judges num/den against threshold once den reaches minSample. Below the minimum it is
// not yet measurable — the rate is still shown when den > 0, but it is not a verdict.
func RateGate(key, description string, num, den int, threshold float64, minSample int, proposedMin bool) Gate {
	g := Gate{
		Key:               key,
		Description:       description,
		Threshold:         fmt.Sprintf(">= %.0f%%", threshold*100),
		Sample:            intPtr(den),
		MinSample:         intPtr(minSample),
		ProposedMinSample: proposedMin,
		Evidence:          fmt.Sprintf("%d of %d", num, den),
	}
	if den > 0 {
		g.Measured = floatPtr(float64(num) / float64(den))
	}
	switch {
	case den < minSample || den == 0:
		g.Status = StatusNotYetMeasurable
	case float64(num)/float64(den) >= threshold:
		g.Status = StatusMet
	default:
		g.Status = StatusUnmet
	}
	return g
}

// countGate judges value >= target.
func countGate(key, description string, value, target int, proposed bool, evidence string) Gate {
	status := StatusUnmet
	if value >= target {
		status = StatusMet
	}
	return Gate{
		Key: key, Description: description, Threshold: fmt.Sprintf(">= %d", target),
		ProposedThreshold: proposed, Measured: floatPtr(float64(value)), Status: status, Evidence: evidence,
	}
}

// fixedGate is a gate the system cannot judge: an owner observation, a missing data source, or a
// metric pending lane G1.
func fixedGate(key, description, status, evidence string) Gate {
	return Gate{Key: key, Description: description, Status: status, Evidence: evidence}
}

// participantGate reads the participant target at goal, sustained across two windows.
func participantGate(goal int, p TargetInputs) Gate {
	ev := EvaluateTarget(goal, p.Current, p.Previous, p.Returning)
	return Gate{
		Key:         "monthly_active_participants",
		Description: "Participant identities over a rolling 30-day window, sustained across two consecutive windows (idx 86 counter).",
		Threshold:   fmt.Sprintf(">= %d in two consecutive 30-day windows", goal),
		Measured:    floatPtr(float64(p.Current)),
		Status:      ev.Status,
		Evidence:    fmt.Sprintf("current %d, previous %d, returning %d", p.Current, p.Previous, p.Returning),
	}
}

// sourceAnalysisGate is stage 2's activation and retention by acquisition source: the public-source
// figures are measured from lane G1's share attribution; repeating the analysis is an owner review.
func sourceAnalysisGate(src SourceMeasures) Gate {
	g := fixedGate("activation_retention_by_source", "Activation and retention repeated by acquisition source.",
		StatusNotYetMeasurable, shareNotRead)
	if src.Available {
		g.Evidence = sourceEvidence(src) + ". Other channels have no recorded source. " + ownerObservation
	}
	return g
}

// channelsGate is stage 3's "at least two acquisition channels measured": only channels with a
// recorded source count, and public sharing is the only one Solvr records.
func channelsGate(src SourceMeasures) Gate {
	if !src.Available {
		return fixedGate("acquisition_channels", "At least two acquisition channels measured and sustainable.",
			StatusNotYetMeasurable, shareNotRead)
	}
	return countGate("acquisition_channels", "At least two acquisition channels measured and sustainable.", 1, 2, false,
		"measured: public_room_sharing; not recorded: "+strings.Join(unrecordedChannels, ", ")+
			". Whether a measured channel is sustainable is an owner judgment.")
}

// retainedCohortsGate is stage 4's "channels with proven retained cohorts": the public-sharing
// cohort's returns are measured; proving a channel's cohorts is an owner judgment across channels.
func retainedCohortsGate(src SourceMeasures) Gate {
	g := fixedGate("retained_channel_cohorts", "Channels with proven retained cohorts.", StatusNotYetMeasurable, shareNotRead)
	if src.Available {
		g.Evidence = "public_room_sharing — " + sourceEvidence(src) + ". " + ownerObservation
	}
	return g
}

// CombineStageStatus folds a stage's gates into its status. A stage whose predecessor is not met
// is blocked whatever its own gates say; otherwise any unmet gate makes it unmet, then a gate
// pending lane G1, then a gate not yet measurable; only all-met is met.
func CombineStageStatus(previousMet bool, gates []Gate) string {
	if !previousMet {
		return StatusBlockedByPreviousStage
	}
	has := map[string]bool{}
	for _, g := range gates {
		has[g.Status] = true
	}
	switch {
	case has[StatusUnmet]:
		return StatusUnmet
	case has[StatusPendingG1Merge]:
		return StatusPendingG1Merge
	case has[StatusNotYetMeasurable]:
		return StatusNotYetMeasurable
	case len(gates) == 0:
		return StatusNotYetMeasurable
	default:
		return StatusMet
	}
}

// EvaluateStages evaluates the four stages in order. Every gate is measured even when its stage is
// blocked, so the operator sees how far each one is.
func EvaluateStages(m StageMeasures, p TargetInputs) StageReport {
	workflowEvidence := "no activated rooms this week"
	if len(m.Workflows) > 0 {
		workflowEvidence = fmt.Sprintf("most activated rooms this week by connect preset: %s (%d of %d)",
			m.Workflows[0].Preset, m.Workflows[0].ActivatedRooms, m.WeeklyActivatedRooms)
	}
	stage1 := []Gate{
		countGate("weekly_activated_rooms", "Rooms that reached a two-way exchange in the last 7 days.",
			m.WeeklyActivatedRooms, StageOneWeeklyActivatedRooms, false,
			fmt.Sprintf("%d activated rooms; %d with an unresolvable creator", m.WeeklyActivatedRooms, m.WeeklyUnknownOwnerRooms)),
		countGate("independent_owners", "Distinct owners of this week's activated rooms (an agent's claiming human, or the agent when unclaimed).",
			m.WeeklyOwners, proposedMinIndependentOwners, true,
			fmt.Sprintf("%d owners; the largest owner has %d of %d rooms", m.WeeklyOwners, m.LargestOwnerRooms, m.WeeklyActivatedRooms)),
		fixedGate("connect_unaided", "Users connect their agents without help.", StatusNotYetMeasurable, ownerObservation),
		fixedGate("repeated_workflow", "The most repeated useful workflow is identified.", StatusNotYetMeasurable,
			ownerObservation+" Evidence: "+workflowEvidence),
		RateGate("gate_a_two_way_within_24h", "Rooms created in the 30 days ending 24h ago that reached a two-way exchange within 24 hours.",
			m.GateAConverted, m.GateAEligible, gateARate, proposedGateAMinRooms, true),
		RateGate("gate_b_creator_return_7d", "Owners whose first room fell in the 30 days ending 7 days ago and created another activated room within 7 days.",
			m.GateBReturned, m.GateBEligible, gateBRate, gateBMinCreators, false),
	}
	stage2 := []Gate{
		participantGate(StageTwoParticipants, p),
		sourceAnalysisGate(m.Source),
	}
	reliability := RateGate("core_service_reliability",
		"Operational health checks of the core services (api, database) over 30 days; IPFS is reported, not gated.",
		m.CoreOperational, m.CoreChecks, proposedCoreUptime, 1, false)
	reliability.Threshold = ">= 99.5%"
	reliability.ProposedThreshold = true
	backlog := Gate{
		Key:               "moderation_backlog",
		Description:       "Pending flags, reports and posts awaiting moderation for more than 7 days (current queue).",
		Threshold:         "= 0",
		ProposedThreshold: true,
		Measured:          floatPtr(float64(m.ModerationBacklog)),
		Status:            StatusUnmet,
		Evidence:          fmt.Sprintf("backlog %d; %d flags and reports created in 30 days", m.ModerationBacklog, m.ModerationVolume),
	}
	if m.ModerationBacklog == 0 {
		backlog.Status = StatusMet
	}
	stage3 := []Gate{
		participantGate(StageThreeParticipants, p),
		reliability,
		fixedGate("cost_per_activated_room", "Cost per activated room, measured and sustainable.", StatusNotYetMeasurable,
			"No cost data source is connected."),
		backlog,
		channelsGate(m.Source),
	}
	stage4 := []Gate{
		participantGate(MonthlyActiveParticipantGoal, p),
		retainedCohortsGate(m.Source),
		fixedGate("tested_capacity", "Capacity tested at the target scale.", StatusNotYetMeasurable,
			"No load test at the target scale is recorded."),
	}

	defs := []struct {
		name, target string
		gates        []Gate
	}{
		{"Repeatable workflow", "100 weekly activated rooms from multiple independent owners", stage1},
		{"Proven workflow at scale", "10,000 monthly active participants", stage2},
		{"Sustainable channels", "100,000 monthly active participants", stage3},
		{"One million", "1,000,000 monthly active participants", stage4},
	}
	report := StageReport{
		End: m.End, Workflows: nonNilWorkflows(m.Workflows), Services: nonNilServices(m.Services),
		Note: stagesNote, Privacy: GrowthPrivacy,
	}
	previousMet := true
	for i, d := range defs {
		status := CombineStageStatus(previousMet, d.gates)
		report.Stages = append(report.Stages, Stage{Number: i + 1, Name: d.name, Target: d.target, Status: status, Gates: d.gates})
		previousMet = status == StatusMet
	}
	return report
}

func nonNilWorkflows(w []WorkflowCount) []WorkflowCount {
	if w == nil {
		return []WorkflowCount{}
	}
	return w
}

func nonNilServices(s []ServiceUptime) []ServiceUptime {
	if s == nil {
		return []ServiceUptime{}
	}
	return s
}
