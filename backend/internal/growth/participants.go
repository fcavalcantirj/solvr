// Package growth holds Solvr's growth definitions, targets and planning arithmetic.
//
// Everything here is pure: no database, no clock, no I/O. The db package measures raw figures
// from real tables and hands them over as plain structs; this package turns them into the
// operator reports, with every definition carried in the response so a reader never has to
// guess what a number means. Nothing in this package is ever served on a public surface: the
// reports sit behind the operator gate (handlers.RequireOperatorAccess).
package growth

import "time"

// Report statuses. A growth OUTCOME is never marked met by shipping software: it is met only
// when the measured figure clears its threshold, and a figure that cannot be measured yet says
// so rather than reading as zero.
const (
	StatusMet              = "met"
	StatusUnmet            = "unmet"
	StatusNotYetMeasurable = "not_yet_measurable"
	// StatusPendingG1Merge marks a metric defined against attribution, activation or return
	// recording that lane G1 (spec.json idx 88/92) is adding and that has not merged yet.
	StatusPendingG1Merge = "pending_g1_merge"
	// StatusBlockedByPreviousStage marks a stage whose predecessor is not met.
	StatusBlockedByPreviousStage = "blocked_by_previous_stage"
)

// MonthlyActiveParticipantGoal is the one-million target (spec.json idx 86 step 1).
const MonthlyActiveParticipantGoal = 1_000_000

// ParticipantWindowDays is the rolling window participants are counted over.
const ParticipantWindowDays = 30

// ActionIdentities is how many distinct identities one family of qualifying actions drew.
type ActionIdentities struct {
	Action string `json:"action"`
	Humans int    `json:"humans"`
	Agents int    `json:"agent_identities"`
}

// AnonymousMeasures is anonymous activity the server recorded, as EVENTS, never people.
type AnonymousMeasures struct {
	Searches     int
	PostViews    int
	BrowserSteps int
	// Flows is distinct anonymous connection flows (a server-issued flow_id from GET
	// /v1/connect); FlowsAttributedToIdentity is the subset an authenticated server step later
	// carried, deterministically, by the same flow_id.
	Flows                     int
	FlowsAttributedToIdentity int
}

// TrafficMeasures is traffic, reported apart from participants.
type TrafficMeasures struct {
	APIRequests        int
	APIRequestsByActor map[string]int
	APIRequestsByKind  map[string]int
	SearchesBySearcher map[string]int
	MonitoringSearches int
	PostViewsRecorded  int
	ActiveRooms        int
	Activations        int
	HumanRegistrations int
	AgentRegistrations int
}

// ParticipantMeasures is the raw measurement of one rolling window and the window before it.
type ParticipantMeasures struct {
	WindowStart         time.Time
	WindowEnd           time.Time
	PreviousWindowStart time.Time

	Humans          int
	Agents          int
	PreviousHumans  int
	PreviousAgents  int
	ReturningHumans int
	ReturningAgents int

	// KnownOverlap counts active agent identities whose claiming human is also an active
	// human; UnresolvedOverlap counts active agents no human has claimed, who may belong to
	// anyone, including a counted human.
	KnownOverlap      int
	UnresolvedOverlap int

	ByAction  []ActionIdentities
	Anonymous AnonymousMeasures
	Traffic   TrafficMeasures
}

// TargetEvaluation is the one-million goal read against measured identities.
type TargetEvaluation struct {
	Goal           int    `json:"goal"`
	WindowDays     int    `json:"window_days"`
	Measured       int    `json:"measured_identities"`
	PreviousWindow int    `json:"previous_window_identities"`
	Returning      int    `json:"returning_identities"`
	Status         string `json:"status"`
	Note           string `json:"note"`
}

// EvaluateTarget reads the goal against the current window, the window before it and the
// identities active in both.
//
// The goal is met only when the CURRENT window and the window BEFORE it both reach it:
// sustained 30-day adoption, never a spike that fills one window. Registered accounts are not
// an input at all; the figures are identities that did something useful.
func EvaluateTarget(goal, current, previous, returning int) TargetEvaluation {
	status := StatusUnmet
	if current >= goal && previous >= goal {
		status = StatusMet
	}
	return TargetEvaluation{
		Goal:           goal,
		WindowDays:     ParticipantWindowDays,
		Measured:       current,
		PreviousWindow: previous,
		Returning:      returning,
		Status:         status,
		Note:           targetNote,
	}
}

// ParticipantReport is GET /admin/growth/participants.
type ParticipantReport struct {
	Window          ReportWindow           `json:"window"`
	Definitions     ParticipantDefinitions `json:"definitions"`
	Humans          IdentityCount          `json:"humans"`
	AgentIdentities AgentIdentityCount     `json:"agent_identities"`
	Combined        CombinedIdentities     `json:"combined"`
	ByAction        []ActionIdentities     `json:"by_action"`
	Anonymous       AnonymousReport        `json:"anonymous"`
	Traffic         TrafficReport          `json:"traffic"`
	Target          TargetEvaluation       `json:"monthly_active_participant_target"`
}

// ReportWindow is the rolling window a report covers.
type ReportWindow struct {
	Start               time.Time `json:"start"`
	End                 time.Time `json:"end"`
	Days                int       `json:"days"`
	PreviousWindowStart time.Time `json:"previous_window_start"`
}

// ParticipantDefinitions carries the meaning of every figure in the report.
type ParticipantDefinitions struct {
	Humans          string `json:"humans"`
	AgentIdentities string `json:"agent_identities"`
	Exclusions      string `json:"exclusions"`
	Sessions        string `json:"sessions"`
	Anonymous       string `json:"anonymous"`
	MergeBasis      string `json:"merge_basis"`
	SuccessCriteria string `json:"success_condition"`
	Privacy         string `json:"privacy"`
}

// IdentityCount is one population of verified identities.
type IdentityCount struct {
	Active         int `json:"active"`
	PreviousWindow int `json:"previous_window"`
	Returning      int `json:"returning"`
}

// AgentIdentityCount is the agent population, with the ownership disclosure.
type AgentIdentityCount struct {
	IdentityCount
	Disclosure string `json:"disclosure"`
}

// CombinedIdentities is the only combined figure, labeled as identities with its overlap.
type CombinedIdentities struct {
	Identities        int    `json:"identities"`
	Label             string `json:"label"`
	KnownOverlap      int    `json:"known_overlap"`
	UnresolvedOverlap int    `json:"unresolved_overlap"`
	OverlapNote       string `json:"overlap_note"`
}

// AnonymousFlows is anonymous connection flows and the deterministic merge.
type AnonymousFlows struct {
	Total                int `json:"total"`
	AttributedToIdentity int `json:"attributed_to_identity"`
	AnonymousOnly        int `json:"anonymous_only"`
}

// AnonymousEvents is server-recorded anonymous activity, counted as events.
type AnonymousEvents struct {
	Searches     int `json:"searches"`
	PostViews    int `json:"post_views"`
	BrowserSteps int `json:"browser_steps"`
}

// AnonymousReport keeps anonymous visitors apart from verified identities.
type AnonymousReport struct {
	EstimatedEngagedVisitors *int            `json:"estimated_engaged_visitors"`
	Available                bool            `json:"available"`
	Note                     string          `json:"note"`
	Events                   AnonymousEvents `json:"events"`
	Flows                    AnonymousFlows  `json:"flows"`
}

// APIRequestTraffic is aggregate API request volume.
type APIRequestTraffic struct {
	Total           int            `json:"total"`
	ByActorType     map[string]int `json:"by_actor_type"`
	ByOperationKind map[string]int `json:"by_operation_kind"`
}

// SearchTraffic is search volume by searcher type.
type SearchTraffic struct {
	BySearcherType     map[string]int `json:"by_searcher_type"`
	MonitoringExcluded int            `json:"monitoring_excluded"`
}

// Registrations is accounts created in the window: context, never participants.
type Registrations struct {
	Humans int    `json:"humans"`
	Agents int    `json:"agents"`
	Note   string `json:"note"`
}

// TrafficReport is traffic, kept apart from participants.
type TrafficReport struct {
	Sessions          *int              `json:"sessions"`
	PageViews         *int              `json:"page_views"`
	UnavailableNote   string            `json:"unavailable_note"`
	PostViewsRecorded int               `json:"post_views_recorded"`
	APIRequests       APIRequestTraffic `json:"api_requests"`
	Searches          SearchTraffic     `json:"searches"`
	ActiveRooms       int               `json:"active_rooms"`
	Activations       int               `json:"activations"`
	Registrations     Registrations     `json:"registrations"`
}

// BuildParticipantReport turns raw measures into the operator report.
func BuildParticipantReport(m ParticipantMeasures, goal int) ParticipantReport {
	current := m.Humans + m.Agents
	previous := m.PreviousHumans + m.PreviousAgents
	returning := m.ReturningHumans + m.ReturningAgents
	byAction := m.ByAction
	if byAction == nil {
		byAction = []ActionIdentities{}
	}
	return ParticipantReport{
		Window: ReportWindow{
			Start: m.WindowStart, End: m.WindowEnd, Days: ParticipantWindowDays,
			PreviousWindowStart: m.PreviousWindowStart,
		},
		Definitions: ParticipantDefinitions{
			Humans:          HumanParticipantDefinition,
			AgentIdentities: AgentParticipantDefinition,
			Exclusions:      ParticipantExclusions,
			Sessions:        ParticipantSessionRule,
			Anonymous:       AnonymousDefinition,
			MergeBasis:      AnonymousMergeBasis,
			SuccessCriteria: SuccessCondition,
			Privacy:         GrowthPrivacy,
		},
		Humans: IdentityCount{Active: m.Humans, PreviousWindow: m.PreviousHumans, Returning: m.ReturningHumans},
		AgentIdentities: AgentIdentityCount{
			IdentityCount: IdentityCount{Active: m.Agents, PreviousWindow: m.PreviousAgents, Returning: m.ReturningAgents},
			Disclosure:    AgentOwnershipDisclosure,
		},
		Combined: CombinedIdentities{
			Identities:        current,
			Label:             CombinedLabel,
			KnownOverlap:      m.KnownOverlap,
			UnresolvedOverlap: m.UnresolvedOverlap,
			OverlapNote:       OverlapNote,
		},
		ByAction: byAction,
		Anonymous: AnonymousReport{
			EstimatedEngagedVisitors: nil,
			Available:                false,
			Note:                     AnonymousUnavailableNote,
			Events: AnonymousEvents{
				Searches: m.Anonymous.Searches, PostViews: m.Anonymous.PostViews, BrowserSteps: m.Anonymous.BrowserSteps,
			},
			Flows: AnonymousFlows{
				Total:                m.Anonymous.Flows,
				AttributedToIdentity: m.Anonymous.FlowsAttributedToIdentity,
				AnonymousOnly:        m.Anonymous.Flows - m.Anonymous.FlowsAttributedToIdentity,
			},
		},
		Traffic: TrafficReport{
			Sessions:          nil,
			PageViews:         nil,
			UnavailableNote:   TrafficUnavailableNote,
			PostViewsRecorded: m.Traffic.PostViewsRecorded,
			APIRequests: APIRequestTraffic{
				Total:           m.Traffic.APIRequests,
				ByActorType:     nonNilCounts(m.Traffic.APIRequestsByActor),
				ByOperationKind: nonNilCounts(m.Traffic.APIRequestsByKind),
			},
			Searches: SearchTraffic{
				BySearcherType:     nonNilCounts(m.Traffic.SearchesBySearcher),
				MonitoringExcluded: m.Traffic.MonitoringSearches,
			},
			ActiveRooms: m.Traffic.ActiveRooms,
			Activations: m.Traffic.Activations,
			Registrations: Registrations{
				Humans: m.Traffic.HumanRegistrations,
				Agents: m.Traffic.AgentRegistrations,
				Note:   RegistrationsNote,
			},
		},
		Target: EvaluateTarget(goal, current, previous, returning),
	}
}

// nonNilCounts keeps an empty breakdown an empty object rather than null.
func nonNilCounts(m map[string]int) map[string]int {
	if m == nil {
		return map[string]int{}
	}
	return m
}
