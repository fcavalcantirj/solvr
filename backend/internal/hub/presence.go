package hub

import "github.com/a2aproject/a2a-go/a2a"

// PresenceChange is an agent joining or leaving a room, as one instance tells the others
// sharing its database. Presence itself lives in the database; a change notice only lets
// every instance's live streams show the join or leave, once.
type PresenceChange struct {
	RoomID    RoomID `json:"room_id"`
	AgentName string `json:"agent_name"`
	Joined    bool   `json:"joined"`
	// Origin is the InstanceID of the manager that made the change; that instance has
	// already applied it to its own streams.
	Origin string `json:"origin"`
}

// presenceCmd asks a hub to show another instance's presence change (or a departure
// made here for an agent this hub may or may not hold).
type presenceCmd struct {
	joined    bool
	agentName string
	card      *a2a.AgentCard
}

// InstanceID identifies this manager among the instances sharing the database.
func (m *HubManager) InstanceID() string { return m.instanceID }

// SetPresenceNotifier sets how this instance tells the others about presence changes it
// makes (the cross-instance room relay). Without one, changes stay local.
func (m *HubManager) SetPresenceNotifier(fn func(PresenceChange)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifier = fn
}

// Joined tells the other instances that agentName joined the room here. The local join
// was already announced when the agent subscribed.
func (m *HubManager) Joined(id RoomID, agentName string) {
	m.notify(PresenceChange{RoomID: id, AgentName: agentName, Joined: true})
}

// Left applies a departure made here — the agent left, was renamed or expired — and tells
// the other instances. The agent's subscription is closed if this instance holds it
// (announcing the leave); otherwise the leave is announced to this instance's streams.
func (m *HubManager) Left(id RoomID, agentName string) {
	m.depart(id, agentName)
	m.notify(PresenceChange{RoomID: id, AgentName: agentName})
}

// ApplyRemotePresence shows another instance's presence change on this instance's
// streams. card is the joining agent's card (nil when unknown). Changes this instance
// made itself are ignored: they were applied when made.
func (m *HubManager) ApplyRemotePresence(c PresenceChange, card *a2a.AgentCard) {
	if c.Origin == m.instanceID {
		return
	}
	if !c.Joined {
		m.depart(c.RoomID, c.AgentName)
		return
	}
	if h := m.Get(c.RoomID); h != nil {
		h.applyPresence(presenceCmd{joined: true, agentName: c.AgentName, card: card})
	}
}

func (m *HubManager) depart(id RoomID, agentName string) {
	m.registry.Remove(id, agentName)
	if h := m.Get(id); h != nil {
		h.applyPresence(presenceCmd{agentName: agentName})
	}
}

func (m *HubManager) notify(c PresenceChange) {
	m.mu.RLock()
	fn := m.notifier
	m.mu.RUnlock()
	if fn == nil {
		return
	}
	c.Origin = m.instanceID
	fn(c)
}

// applyPresence hands cmd to the hub goroutine; a stopped hub has no streams to tell.
func (h *RoomHub) applyPresence(cmd presenceCmd) {
	select {
	case h.presence <- cmd:
	case <-h.done:
	}
}
