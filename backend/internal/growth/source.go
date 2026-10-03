package growth

import "fmt"

// SourceMeasures is lane G1's share attribution (spec.json idx 88) read for one window: the one
// acquisition source Solvr records is a public room or post that somebody shared or reused
// ("Try this workflow"). Available is false when the attribution was not read.
type SourceMeasures struct {
	Available                bool
	ShareVisits              int
	HumanShareVisits         int
	AttributedRoomsActivated int
	NewHumanActivations      int
	NewAgentActivations      int
	HumanReturns7d           ReturnCount
	HumanReturns28d          ReturnCount
	AgentReturns7d           ReturnCount
	AgentReturns28d          ReturnCount
}

// unrecordedChannels are the acquisition channels no recorded source distinguishes yet.
var unrecordedChannels = []string{"seo", "agent_ecosystem_referrals", "direct"}

const (
	shareNotRead = "The share attribution was not read for this window (lane G1, spec.json idx 88), so this figure " +
		"is not yet measurable here."
	noRecordedSource = "There is no recorded source for this channel: Solvr records one acquisition source, a public room or " +
		"post that was shared or reused (spec.json idx 88). Rooms from this channel count as unattributed."
)

// sourceEvidence summarizes the public-source activations and their returns.
func sourceEvidence(src SourceMeasures) string {
	return fmt.Sprintf("public room or post: %d activated rooms, %d new humans and %d new agent identities activated; "+
		"28-day returns humans %d/%d, agents %d/%d",
		src.AttributedRoomsActivated, src.NewHumanActivations, src.NewAgentActivations,
		src.HumanReturns28d.Returned, src.HumanReturns28d.Eligible,
		src.AgentReturns28d.Returned, src.AgentReturns28d.Eligible)
}
