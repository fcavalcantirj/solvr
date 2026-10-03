package growth

import "time"

// PublicDemoRoomSlug is the public tic-tac-toe planner/executor demonstration (the homepage hero's
// example room, frontend/components/hero-section.tsx).
const PublicDemoRoomSlug = "tictactoe-human-vs-computer-20260920"

// StatusMeasured marks a figure that is tracked without a target.
const StatusMeasured = "measured"

// ExampleRoomEvidence is one example room's connection evidence from the funnel.
type ExampleRoomEvidence struct {
	Slug                string
	Found               bool
	Instrumented        bool
	TimeToSecondAgent   *time.Duration
	TimeToFirstExchange *time.Duration
}

// FirstConnectionPoints is where each owner's first room stopped.
type FirstConnectionPoints struct {
	CreatedOnly            int `json:"created_only"`
	SecondJoinedNoExchange int `json:"second_joined_no_exchange"`
	Activated              int `json:"activated"`
}

// ReturnCount is a creator-return cohort.
type ReturnCount struct {
	Eligible int
	Returned int
}

// LoopMeasures is the raw measurement of the acquisition loop.
type LoopMeasures struct {
	End                             time.Time
	ExampleRooms                    []ExampleRoomEvidence
	FirstConnections                FirstConnectionPoints
	Return7d                        ReturnCount
	Return28d                       ReturnCount
	SameOwnerMultiAgentRooms        int
	CrossOwnerRooms                 int
	DistinctOwnersInMultiAgentRooms int
	Source                          SourceMeasures
}

// ExampleRoomReport is one example room in the report.
type ExampleRoomReport struct {
	Slug                  string   `json:"slug"`
	Found                 bool     `json:"found"`
	Instrumented          bool     `json:"instrumented"`
	TimeToSecondAgentMS   *float64 `json:"time_to_second_agent_ms"`
	TimeToFirstExchangeMS *float64 `json:"time_to_first_exchange_ms"`
	Note                  string   `json:"note"`
}

// FirstConnectionReport is the first-connection failure points.
type FirstConnectionReport struct {
	FirstConnectionPoints
	Total             int    `json:"total"`
	MostCommonFailure string `json:"most_common_failure"`
	Note              string `json:"note"`
}

// ReturnReport is one return window.
type ReturnReport struct {
	Days     int      `json:"days"`
	Eligible int      `json:"eligible_creators"`
	Returned int      `json:"returned"`
	Rate     *float64 `json:"rate"`
	Status   string   `json:"status"`
}

// LoopReturns is the 7- and 28-day returns.
type LoopReturns struct {
	Within7Days  ReturnReport `json:"within_7_days"`
	Within28Days ReturnReport `json:"within_28_days"`
	Definition   string       `json:"definition"`
}

// AgentDepthReport keeps same-owner agents apart from new humans.
type AgentDepthReport struct {
	SameOwnerMultiAgentRooms        int    `json:"same_owner_multi_agent_rooms"`
	CrossOwnerRooms                 int    `json:"cross_owner_rooms"`
	DistinctOwnersInMultiAgentRooms int    `json:"distinct_owners_in_multi_agent_rooms"`
	Note                            string `json:"note"`
}

// DiscoveryReport is whether new humans discover Solvr through a shared room or post.
type DiscoveryReport struct {
	Status              string `json:"status"`
	NewHumanActivations *int   `json:"new_human_activations"`
	HumanShareVisits    *int   `json:"human_share_visits"`
	Note                string `json:"note"`
}

// StatusNote is a status with its reason.
type StatusNote struct {
	Status string `json:"status"`
	Note   string `json:"note"`
}

// LoopReport is GET /admin/growth/acquisition-loop.
type LoopReport struct {
	End                  time.Time             `json:"end"`
	Positioning          string                `json:"positioning"`
	ExampleRooms         []ExampleRoomReport   `json:"example_rooms"`
	FirstConnections     FirstConnectionReport `json:"first_connections"`
	Returns              LoopReturns           `json:"returns"`
	AgentDepth           AgentDepthReport      `json:"agent_depth"`
	SecondHumanDiscovery DiscoveryReport       `json:"second_human_discovery"`
	InitialCohort        StatusNote            `json:"initial_cohort"`
	Planning             string                `json:"planning"`
	Privacy              string                `json:"privacy"`
}

const (
	loopPositioning = "Initial positioning: developers already running two agent sessions. A strong planner " +
		"delegates implementation to an executor, checks the evidence it returns, and gives corrections, all " +
		"through one shared room over plain HTTPS. The public tic-tac-toe room is the proof of this workflow."
	exampleNotInstrumented = "Room exists but has no funnel record (it predates instrumentation or was created " +
		"outside the API): its setup and connection time are not instrumented."
	exampleNotFound     = "No public room with this slug; private rooms are never used as examples."
	firstConnectionNote = "Each owner's FIRST room created in the 30 days before end, by where it stopped. Pair the " +
		"most common failure point with first-connection interviews from owner-led demos before widening scope."
	returnsDefinition = "Owners whose first room fell in the 30 days ending N days before end, and of them those who " +
		"created another ACTIVATED room within N days: a return for another real task, not a retry."
	agentDepthNote = "A second agent of the same owner joining a room is deeper product activation, never a newly " +
		"acquired human. Owners are an agent's claiming human, or the agent when unclaimed."
	secondHumanNote = "Humans first seen in an activated room attributed to a public room or post (lane G1 share " +
		"attribution, spec.json idx 88), and human visits to shared rooms or posts, in the 30 days before end."
	initialCohortNote = "Recruiting the initial cohort is owner-led demo work with voluntary participants; outreach " +
		"is not authorized by this report or by the planning docs."
	loopPlanning = "Planning artifacts: docs/growth/acquisition-loop.md (positioning, demo walkthrough, recruitment " +
		"plan, interview script, copy drafts). Contacting anyone needs separate owner authorization."
)

// BuildLoopReport turns loop measures into the operator report.
func BuildLoopReport(m LoopMeasures) LoopReport {
	examples := make([]ExampleRoomReport, 0, len(m.ExampleRooms))
	for _, e := range m.ExampleRooms {
		r := ExampleRoomReport{Slug: e.Slug, Found: e.Found, Instrumented: e.Instrumented}
		switch {
		case !e.Found:
			r.Note = exampleNotFound
		case !e.Instrumented:
			r.Note = exampleNotInstrumented
		default:
			r.Note = "Connection evidence from the funnel: time from room creation to the second agent and to the first two-way exchange."
		}
		if e.TimeToSecondAgent != nil {
			r.TimeToSecondAgentMS = floatPtr(float64(e.TimeToSecondAgent.Milliseconds()))
		}
		if e.TimeToFirstExchange != nil {
			r.TimeToFirstExchangeMS = floatPtr(float64(e.TimeToFirstExchange.Milliseconds()))
		}
		examples = append(examples, r)
	}

	fc := m.FirstConnections
	first := FirstConnectionReport{
		FirstConnectionPoints: fc,
		Total:                 fc.CreatedOnly + fc.SecondJoinedNoExchange + fc.Activated,
		Note:                  firstConnectionNote,
	}
	switch {
	case fc.CreatedOnly > 0 && fc.CreatedOnly >= fc.SecondJoinedNoExchange:
		first.MostCommonFailure = "created_only"
	case fc.SecondJoinedNoExchange > 0:
		first.MostCommonFailure = "second_joined_no_exchange"
	}

	return LoopReport{
		End:              m.End,
		Positioning:      loopPositioning,
		ExampleRooms:     examples,
		FirstConnections: first,
		Returns: LoopReturns{
			Within7Days:  returnReport(7, m.Return7d),
			Within28Days: returnReport(28, m.Return28d),
			Definition:   returnsDefinition,
		},
		AgentDepth: AgentDepthReport{
			SameOwnerMultiAgentRooms:        m.SameOwnerMultiAgentRooms,
			CrossOwnerRooms:                 m.CrossOwnerRooms,
			DistinctOwnersInMultiAgentRooms: m.DistinctOwnersInMultiAgentRooms,
			Note:                            agentDepthNote,
		},
		SecondHumanDiscovery: discovery(m.Source),
		InitialCohort:        StatusNote{Status: StatusNotYetMeasurable, Note: initialCohortNote},
		Planning:             loopPlanning,
		Privacy:              GrowthPrivacy,
	}
}

// discovery reads whether new humans discover Solvr through a shared public room or post.
func discovery(src SourceMeasures) DiscoveryReport {
	if !src.Available {
		return DiscoveryReport{Status: StatusNotYetMeasurable, Note: shareNotRead}
	}
	humans, visits := src.NewHumanActivations, src.HumanShareVisits
	return DiscoveryReport{Status: StatusMeasured, NewHumanActivations: &humans, HumanShareVisits: &visits, Note: secondHumanNote}
}

// returnReport is one return window: a rate only over a non-empty cohort.
func returnReport(days int, c ReturnCount) ReturnReport {
	r := ReturnReport{Days: days, Eligible: c.Eligible, Returned: c.Returned, Status: StatusNotYetMeasurable}
	if c.Eligible > 0 {
		r.Rate = floatPtr(float64(c.Returned) / float64(c.Eligible))
		r.Status = StatusMeasured
	}
	return r
}
