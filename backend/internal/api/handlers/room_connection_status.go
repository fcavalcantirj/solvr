package handlers

// Connection status values describe a room's collaboration progress. They are
// derived on the server (the client renders them, it never computes them) from
// two real signals only: how many agents are currently present with
// server-confirmed, unexpired heartbeats, and whether the room has recorded the
// persisted two-way activation milestone. Copying a prompt, registering an
// agent, or opening a browser stream touch neither signal, so none of them can
// advance a room's status.
const (
	// ConnectionStatusWaitingForAgents: no agent is present yet.
	ConnectionStatusWaitingForAgents = "waiting_for_agents"
	// ConnectionStatusWaitingForAnotherAgent: at least one agent is present but
	// the room has not yet carried a two-way exchange.
	ConnectionStatusWaitingForAnotherAgent = "waiting_for_another_agent"
	// ConnectionStatusConversationStarted: the room has recorded a two-way
	// exchange between at least two distinct agent identities. This milestone is
	// sticky — it survives every agent's presence expiring.
	ConnectionStatusConversationStarted = "conversation_started"
)

// ComputeConnectionStatus derives a room's connection progress from whether it
// has recorded the two-way activation milestone and how many agents are
// currently present (server-confirmed, unexpired).
//
// The milestone wins: once a room has carried a two-way exchange it reports
// "conversation started" even after everyone leaves, so a completed
// collaboration never regresses to "waiting" just because presence expired.
func ComputeConnectionStatus(activated bool, onlineCount int) string {
	if activated {
		return ConnectionStatusConversationStarted
	}
	if onlineCount <= 0 {
		return ConnectionStatusWaitingForAgents
	}
	return ConnectionStatusWaitingForAnotherAgent
}
