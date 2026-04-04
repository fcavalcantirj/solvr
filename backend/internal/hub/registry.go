package hub

import (
	"strings"
	"sync"
	"time"

	"github.com/a2aproject/a2a-go/a2a"
)

// AgentPresence holds an agent's card and connection metadata within a room.
// This is the in-memory representation; the DB-backed counterpart is models.AgentPresenceRecord.
type AgentPresence struct {
	// Card is the full agent card received at join time.
	Card *a2a.AgentCard

	// AgentName is the canonical name from the card.
	AgentName string

	// JoinedAt is the time the agent subscribed to the room.
	JoinedAt time.Time

	// LastSeen is updated on each heartbeat or message from the agent.
	LastSeen time.Time
}

// PresenceRegistry is a thread-safe in-memory store of agent presence records
// keyed by RoomID and agent name. It supports concurrent reads with a single
// RWMutex -- safe for the many-readers (discovery queries), few-writers (join/leave)
// pattern in a typical room.
type PresenceRegistry struct {
	mu     sync.RWMutex
	agents map[RoomID]map[string]*AgentPresence // roomID -> agentName -> presence
}

// NewPresenceRegistry creates an empty PresenceRegistry.
func NewPresenceRegistry() *PresenceRegistry {
	return &PresenceRegistry{
		agents: make(map[RoomID]map[string]*AgentPresence),
	}
}

// Add records an agent's presence in the given room. If the agent is already
// present (e.g. re-connecting), the record is updated in place.
func (r *PresenceRegistry) Add(roomID RoomID, agentName string, card *a2a.AgentCard) {
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.agents[roomID]; !ok {
		r.agents[roomID] = make(map[string]*AgentPresence)
	}
	if existing, ok := r.agents[roomID][agentName]; ok {
		// Update existing record -- preserve JoinedAt.
		existing.Card = card
		existing.LastSeen = now
		return
	}
	r.agents[roomID][agentName] = &AgentPresence{
		Card:      card,
		AgentName: agentName,
		JoinedAt:  now,
		LastSeen:  now,
	}
}

// Remove deletes an agent's presence record from the given room.
// No-op if the agent is not present.
func (r *PresenceRegistry) Remove(roomID RoomID, agentName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if room, ok := r.agents[roomID]; ok {
		delete(room, agentName)
		if len(room) == 0 {
			delete(r.agents, roomID)
		}
	}
}

// Get returns the presence record for a specific agent in a room.
// Returns (nil, false) if not found.
func (r *PresenceRegistry) Get(roomID RoomID, agentName string) (*AgentPresence, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if room, ok := r.agents[roomID]; ok {
		p, found := room[agentName]
		return p, found
	}
	return nil, false
}

// ListAll returns all presence records for the given room.
// Returns an empty slice if the room has no agents.
func (r *PresenceRegistry) ListAll(roomID RoomID) []*AgentPresence {
	r.mu.RLock()
	defer r.mu.RUnlock()
	room, ok := r.agents[roomID]
	if !ok {
		return []*AgentPresence{}
	}
	out := make([]*AgentPresence, 0, len(room))
	for _, p := range room {
		out = append(out, p)
	}
	return out
}

// ListPublicCards returns a stripped AgentCard for each agent in the room.
// The public card contains only Name, Description, and Skills -- no URL,
// no Capabilities, and no SecuritySchemes. This prevents leaking private
// endpoint URLs to unauthenticated callers.
func (r *PresenceRegistry) ListPublicCards(roomID RoomID) []*a2a.AgentCard {
	r.mu.RLock()
	defer r.mu.RUnlock()
	room, ok := r.agents[roomID]
	if !ok {
		return []*a2a.AgentCard{}
	}
	out := make([]*a2a.AgentCard, 0, len(room))
	for name, p := range room {
		// Skip browser SSE subscribers and agents with nil cards
		if strings.HasPrefix(name, "_browser_") || p.Card == nil {
			continue
		}
		out = append(out, publicCard(p.Card))
	}
	return out
}

// ExtendedCard returns the full AgentCard for an agent. Callers must have
// already verified the bearer token before calling this method.
func (r *PresenceRegistry) ExtendedCard(roomID RoomID, agentName string) (*a2a.AgentCard, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if room, ok := r.agents[roomID]; ok {
		if p, found := room[agentName]; found {
			return p.Card, true
		}
	}
	return nil, false
}

// FilterBySkillID returns public cards for all agents in the room that have
// at least one skill matching the given skill ID.
func (r *PresenceRegistry) FilterBySkillID(roomID RoomID, skillID string) []*a2a.AgentCard {
	r.mu.RLock()
	defer r.mu.RUnlock()
	room, ok := r.agents[roomID]
	if !ok {
		return []*a2a.AgentCard{}
	}
	var out []*a2a.AgentCard
	for _, p := range room {
		for _, skill := range p.Card.Skills {
			if skill.ID == skillID {
				out = append(out, publicCard(p.Card))
				break
			}
		}
	}
	return out
}

// FilterByTag returns public cards for all agents in the room that have at
// least one skill with the given tag.
func (r *PresenceRegistry) FilterByTag(roomID RoomID, tag string) []*a2a.AgentCard {
	r.mu.RLock()
	defer r.mu.RUnlock()
	room, ok := r.agents[roomID]
	if !ok {
		return []*a2a.AgentCard{}
	}
	var out []*a2a.AgentCard
	for _, p := range room {
		if agentHasTag(p.Card, tag) {
			out = append(out, publicCard(p.Card))
		}
	}
	return out
}

// UpdateLastSeen refreshes the LastSeen timestamp for an agent.
// Called on each heartbeat or received message.
func (r *PresenceRegistry) UpdateLastSeen(roomID RoomID, agentName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if room, ok := r.agents[roomID]; ok {
		if p, found := room[agentName]; found {
			p.LastSeen = time.Now()
		}
	}
}

// AgentCount returns the number of agents currently registered in the room.
func (r *PresenceRegistry) AgentCount(roomID RoomID) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for name := range r.agents[roomID] {
		if !strings.HasPrefix(name, "_browser_") {
			count++
		}
	}
	return count
}

// TotalAgentCount returns the total number of agents across all rooms.
func (r *PresenceRegistry) TotalAgentCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	total := 0
	for _, room := range r.agents {
		total += len(room)
	}
	return total
}

// AllPublicAgents returns all presence records across all rooms. Used by the
// cross-room directory endpoint. Callers are responsible for filtering
// to public rooms before returning data to unauthenticated users.
func (r *PresenceRegistry) AllPublicAgents() []*AgentPresence {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*AgentPresence
	for _, room := range r.agents {
		for _, p := range room {
			out = append(out, p)
		}
	}
	return out
}

// publicCard returns a stripped copy of an AgentCard with only public fields:
// Name, Description, and Skills. URL, Capabilities, and SecuritySchemes are
// intentionally omitted to avoid leaking private endpoint information.
func publicCard(card *a2a.AgentCard) *a2a.AgentCard {
	if card == nil {
		return nil
	}
	return &a2a.AgentCard{
		Name:        card.Name,
		Description: card.Description,
		Skills:      card.Skills,
	}
}

// agentHasTag returns true if any skill in the card has the given tag.
func agentHasTag(card *a2a.AgentCard, tag string) bool {
	for _, skill := range card.Skills {
		for _, t := range skill.Tags {
			if t == tag {
				return true
			}
		}
	}
	return false
}
