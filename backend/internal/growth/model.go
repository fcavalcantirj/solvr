package growth

import (
	"math"
	"time"
)

// Input sources.
const (
	SourceObserved     = "observed"
	SourceHypothetical = "hypothetical"
)

// ScenarioMonths is how many months a scenario projects.
const ScenarioMonths = 12

// Bottlenecks.
const (
	BottleneckCapacity    = "capacity"
	BottleneckConnection  = "connection_success"
	BottleneckRepeatUsage = "repeat_usage"
	BottleneckReach       = "reach"
)

// Input is one model input and where it came from.
type Input struct {
	Value  float64 `json:"value"`
	Source string  `json:"source"`
}

// CohortObservation is one monthly cohort's active identities by age in months.
type CohortObservation struct {
	CohortMonth string `json:"cohort_month"`
	ActiveByAge []int  `json:"active_by_age"`
}

// PopulationFlows is one population's observed month.
type PopulationFlows struct {
	Active            int                 `json:"active"`
	Retained          int                 `json:"retained"`
	New               int                 `json:"new"`
	Reactivated       int                 `json:"reactivated"`
	PreviousActive    int                 `json:"previous_active"`
	MeasuredRetention *float64            `json:"measured_retention"`
	Cohorts           []CohortObservation `json:"cohorts"`
	Survival          []float64           `json:"survival"`
}

// MonthlyFlows is the observed month for both populations.
type MonthlyFlows struct {
	Month        string          `json:"month"`
	Start        time.Time       `json:"start"`
	End          time.Time       `json:"end"`
	Humans       PopulationFlows `json:"humans"`
	Agents       PopulationFlows `json:"agents"`
	KnownOverlap int             `json:"known_overlap"`
}

// Cohort is a group of identities that started in one month.
type Cohort struct {
	StartMonth int
	Size       float64
}

// WorkedArithmetic is the spec's worked example.
type WorkedArithmetic struct {
	Active                    float64 `json:"active"`
	MonthlyRetention          float64 `json:"monthly_retention"`
	InflowToStayFlat          float64 `json:"inflow_to_stay_flat"`
	ActivationRate            float64 `json:"activation_rate"`
	QualifiedVisitsToStayFlat float64 `json:"qualified_visits_to_stay_flat"`
	Hypothetical              bool    `json:"hypothetical"`
	Note                      string  `json:"note"`
}

// Scenario is one projection for one population.
type Scenario struct {
	Name                      string    `json:"name"`
	Population                string    `json:"population"`
	StartActive               Input     `json:"start_active"`
	MonthlyRetention          Input     `json:"monthly_retention"`
	NewActivated              Input     `json:"new_activated"`
	Reactivated               Input     `json:"reactivated"`
	Duplicates                Input     `json:"duplicates"`
	ActivationRate            Input     `json:"activation_rate"`
	ImpliedQualifiedVisits    *float64  `json:"implied_qualified_visits"`
	QualifiedVisitsToStayFlat *float64  `json:"qualified_visits_to_stay_flat"`
	SteadyState               *float64  `json:"steady_state"`
	Projection                []float64 `json:"projection"`
}

// ChannelRow is one acquisition channel in the comparison.
type ChannelRow struct {
	Channel                   string   `json:"channel"`
	RetainedActivations       *int     `json:"retained_activations"`
	CostPerRetainedActivation *float64 `json:"cost_per_retained_activation"`
	Status                    string   `json:"status"`
	Note                      string   `json:"note"`
}

// Bottleneck is the model's reading of what limits growth now.
type Bottleneck struct {
	Bottleneck string   `json:"bottleneck"`
	Reason     string   `json:"reason"`
	Missing    []string `json:"missing"`
}

// PaidAcquisition says whether paid acquisition may be scaled.
type PaidAcquisition struct {
	Ready bool   `json:"ready"`
	Note  string `json:"note"`
}

// Review is the monthly review the model is used in.
type Review struct {
	Cadence   string   `json:"cadence"`
	Checklist []string `json:"checklist"`
}

// ModelReport is GET /admin/growth/model.
type ModelReport struct {
	Flows           MonthlyFlows     `json:"flows"`
	Formula         string           `json:"formula"`
	Arithmetic      WorkedArithmetic `json:"worked_arithmetic"`
	Scenarios       []Scenario       `json:"scenarios"`
	Channels        []ChannelRow     `json:"channels"`
	PaidAcquisition PaidAcquisition  `json:"paid_acquisition"`
	Bottleneck      Bottleneck       `json:"bottleneck"`
	Review          Review           `json:"review"`
	Privacy         string           `json:"privacy"`
}

// Hypothetical sensitivity inputs (spec.json idx 90 step 4). They are planning assumptions, never
// measurements, and every scenario labels them so.
var (
	scenarioNames      = []string{"conservative", "base", "optimistic"}
	scenarioRetention  = []float64{0.50, 0.65, 0.80}
	scenarioActivation = []float64{0.05, 0.10, 0.15}
)

// minRetentionSample is how many previously active identities a measured retention needs before
// the base scenario uses it instead of the hypothetical rate.
const minRetentionSample = 30

const (
	modelFormula = "A_next = A_current × monthly_retention + new_activated + reactivated − duplicates. " +
		"Illustrative: the single retention term is replaced by measured cohort survival as cohorts accrue."
	arithmeticNote = "Worked example from spec.json idx 90 with hypothetical percentages: retaining 80% of " +
		"1,000,000 participants needs 200,000 new or reactivated participants a month just to stay flat; at a " +
		"10% qualified-visit activation rate that is 2,000,000 qualified visits a month."
	paidNotReady = "Do not scale paid acquisition: it waits until retention is measured from cohorts and the " +
		"cost per retained activation is known for the channel."
	paidReady   = "Retention is measured and channel cost is known; paid acquisition can be tested against them."
	channelNote = "Source attribution for this channel is recorded by lane G1 (spec.json idx 88); no cost source " +
		"is connected. Compared by retained activations and cost, never by visits or downloads."
)

// NextActive is one month of the illustrative model.
func NextActive(active, retention, newActivated, reactivated, duplicates float64) float64 {
	return active*retention + newActivated + reactivated - duplicates
}

// InflowToStayFlat is the new plus reactivated participants a month that hold active flat.
func InflowToStayFlat(active, retention float64) float64 {
	return active * (1 - retention)
}

// QualifiedVisitsNeeded is the qualified visits an inflow implies at an activation rate; a
// non-positive rate implies no finite number.
func QualifiedVisitsNeeded(inflow, activationRate float64) (float64, bool) {
	if activationRate <= 0 {
		return 0, false
	}
	return inflow / activationRate, true
}

// SteadyState is the active level a constant inflow converges to.
func SteadyState(inflow, retention float64) float64 {
	v, _ := SteadyStateOK(inflow, retention)
	return v
}

// SteadyStateOK is SteadyState, reporting false when retention leaves no finite steady state.
func SteadyStateOK(inflow, retention float64) (float64, bool) {
	if retention >= 1 {
		return 0, false
	}
	return inflow / (1 - retention), true
}

// GoalArithmetic is the spec's worked example, computed rather than typed in.
func GoalArithmetic() WorkedArithmetic {
	// Rounded to whole participants and visits: 1e6 × (1 − 0.8) is 199999.99999999997 in floating point.
	inflow := math.Round(InflowToStayFlat(MonthlyActiveParticipantGoal, 0.80))
	visits, _ := QualifiedVisitsNeeded(inflow, 0.10)
	visits = math.Round(visits)
	return WorkedArithmetic{
		Active: MonthlyActiveParticipantGoal, MonthlyRetention: 0.80, InflowToStayFlat: inflow,
		ActivationRate: 0.10, QualifiedVisitsToStayFlat: visits, Hypothetical: true, Note: arithmeticNote,
	}
}

// ProjectWithCohorts projects active identities month by month from cohorts and a survival curve
// (survival[age] = share of a cohort still active at that age, survival[0] = 1). Past the measured
// ages a cohort keeps the last measured share rather than an invented decay.
func ProjectWithCohorts(cohorts []Cohort, survival []float64, months int) []float64 {
	out := make([]float64, months)
	if len(survival) == 0 {
		return out
	}
	for m := 0; m < months; m++ {
		for _, c := range cohorts {
			age := m - c.StartMonth
			if age < 0 {
				continue
			}
			if age >= len(survival) {
				age = len(survival) - 1
			}
			out[m] += c.Size * survival[age]
		}
	}
	return out
}

// SurvivalFromCohorts measures survival by age, size-weighted over the cohorts old enough to have
// reached that age.
func SurvivalFromCohorts(obs []CohortObservation) []float64 {
	maxAge := 0
	for _, o := range obs {
		if len(o.ActiveByAge) > maxAge {
			maxAge = len(o.ActiveByAge)
		}
	}
	survival := []float64{}
	for age := 0; age < maxAge; age++ {
		var active, size int
		for _, o := range obs {
			if age < len(o.ActiveByAge) && len(o.ActiveByAge) > 0 {
				active += o.ActiveByAge[age]
				size += o.ActiveByAge[0]
			}
		}
		if size == 0 {
			break
		}
		survival = append(survival, float64(active)/float64(size))
	}
	return survival
}

// BuildScenarios projects one population under the conservative, base and optimistic inputs. The
// starting level, new activations and reactivations are the observed month; retention and the
// activation rate are hypothetical, except that the base case uses the measured retention once
// enough identities were active the month before.
func BuildScenarios(population string, f PopulationFlows, duplicates int) []Scenario {
	out := make([]Scenario, 0, len(scenarioNames))
	for i, name := range scenarioNames {
		retention := Input{Value: scenarioRetention[i], Source: SourceHypothetical}
		if name == "base" && f.PreviousActive >= minRetentionSample {
			retention = Input{Value: float64(f.Retained) / float64(f.PreviousActive), Source: SourceObserved}
		}
		s := Scenario{
			Name: name, Population: population,
			StartActive:      Input{Value: float64(f.Active), Source: SourceObserved},
			MonthlyRetention: retention,
			NewActivated:     Input{Value: float64(f.New), Source: SourceObserved},
			Reactivated:      Input{Value: float64(f.Reactivated), Source: SourceObserved},
			Duplicates:       Input{Value: float64(duplicates), Source: SourceObserved},
			ActivationRate:   Input{Value: scenarioActivation[i], Source: SourceHypothetical},
		}
		inflow := s.NewActivated.Value + s.Reactivated.Value - s.Duplicates.Value
		if v, ok := QualifiedVisitsNeeded(s.NewActivated.Value, s.ActivationRate.Value); ok {
			s.ImpliedQualifiedVisits = floatPtr(v)
		}
		if v, ok := QualifiedVisitsNeeded(InflowToStayFlat(s.StartActive.Value, retention.Value), s.ActivationRate.Value); ok {
			s.QualifiedVisitsToStayFlat = floatPtr(v)
		}
		if v, ok := SteadyStateOK(inflow, retention.Value); ok {
			s.SteadyState = floatPtr(v)
		}
		s.Projection = make([]float64, ScenarioMonths)
		a := s.StartActive.Value
		for m := range s.Projection {
			a = NextActive(a, retention.Value, s.NewActivated.Value, s.Reactivated.Value, s.Duplicates.Value)
			s.Projection[m] = a
		}
		out = append(out, s)
	}
	return out
}

// ChannelComparison is the four channels compared by retained activation and cost. Until source
// attribution is recorded every row is pending, with no figure guessed.
func ChannelComparison() []ChannelRow {
	rows := []ChannelRow{}
	for _, c := range []string{"seo", "public_room_sharing", "agent_ecosystem_referrals", "direct"} {
		rows = append(rows, ChannelRow{Channel: c, Status: StatusPendingG1Merge, Note: channelNote})
	}
	return rows
}

// PaidAcquisitionReady allows scaling paid acquisition only when retention is measured AND the
// channel's cost is known.
func PaidAcquisitionReady(retentionMeasured, costKnown bool) (bool, string) {
	if retentionMeasured && costKnown {
		return true, paidReady
	}
	return false, paidNotReady
}

// ClassifyBottleneck names what limits growth now, checking in order: capacity (reliability),
// connection success (gate A), repeat usage (gate B), and otherwise reach. A check that cannot be
// judged stops the reading there and is named as missing.
func ClassifyBottleneck(capacity, connection, repeat Gate) Bottleneck {
	checks := []struct {
		name string
		gate Gate
		why  string
	}{
		{BottleneckCapacity, capacity, "Core services are below the reliability gate; fix capacity before adding traffic."},
		{BottleneckConnection, connection, "Too few created rooms reach a two-way exchange within 24 hours; fix connection success first."},
		{BottleneckRepeatUsage, repeat, "Too few creators come back for another real task within 7 days; fix repeat usage before reach."},
	}
	for _, c := range checks {
		switch c.gate.Status {
		case StatusMet:
			continue
		case StatusUnmet:
			return Bottleneck{Bottleneck: c.name, Reason: c.why, Missing: []string{}}
		default:
			return Bottleneck{
				Bottleneck: StatusNotYetMeasurable,
				Reason:     "The " + c.name + " check cannot be judged yet, so the order of bottlenecks after it is unknown.",
				Missing:    []string{c.name},
			}
		}
	}
	return Bottleneck{Bottleneck: BottleneckReach,
		Reason: "Reliability, connection success and repeat usage clear their gates: the limit is reach.", Missing: []string{}}
}

// reviewChecklist is what the monthly review of the model covers.
var reviewChecklist = []string{
	"Replace each hypothetical input that now has an observed value; note its sample.",
	"Compare last month's base projection with this month's observed active identities, per population.",
	"Refresh the measured cohort survival and compare it with the single-rate retention.",
	"Read the bottleneck: capacity, connection success, repeat usage or reach, and pick the next action for it.",
	"Compare channels by retained activations and cost once attribution is recorded; never scale paid acquisition before both are known.",
	"Record the review in the private operator plan, including every unmet target.",
}

// BuildModelReport assembles the monthly model from observed flows and the stage gates at the
// month's end.
func BuildModelReport(flows MonthlyFlows, stages StageMeasures) ModelReport {
	flows.Humans.Survival = SurvivalFromCohorts(flows.Humans.Cohorts)
	flows.Agents.Survival = SurvivalFromCohorts(flows.Agents.Cohorts)
	if flows.Humans.Cohorts == nil {
		flows.Humans.Cohorts = []CohortObservation{}
	}
	if flows.Agents.Cohorts == nil {
		flows.Agents.Cohorts = []CohortObservation{}
	}
	gates := EvaluateStages(stages, TargetInputs{})
	stage1, stage3 := gates.Stages[0], gates.Stages[2]
	pick := func(s Stage, key string) Gate {
		for _, g := range s.Gates {
			if g.Key == key {
				return g
			}
		}
		return Gate{Status: StatusNotYetMeasurable}
	}
	retentionMeasured := flows.Humans.PreviousActive >= minRetentionSample || flows.Agents.PreviousActive >= minRetentionSample
	ready, note := PaidAcquisitionReady(retentionMeasured, false)
	scenarios := append(BuildScenarios("humans", flows.Humans, 0), BuildScenarios("agents", flows.Agents, 0)...)
	return ModelReport{
		Flows:           flows,
		Formula:         modelFormula,
		Arithmetic:      GoalArithmetic(),
		Scenarios:       scenarios,
		Channels:        ChannelComparison(),
		PaidAcquisition: PaidAcquisition{Ready: ready, Note: note},
		Bottleneck: ClassifyBottleneck(pick(stage3, "core_service_reliability"),
			pick(stage1, "gate_a_two_way_within_24h"), pick(stage1, "gate_b_creator_return_7d")),
		Review:  Review{Cadence: "monthly", Checklist: reviewChecklist},
		Privacy: GrowthPrivacy,
	}
}
