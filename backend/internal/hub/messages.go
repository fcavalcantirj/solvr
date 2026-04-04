package hub

import (
	"sync"
	"time"
)

// StoredMessage represents a message stored in the room's message buffer.
type StoredMessage struct {
	ID        int       `json:"id"`
	AgentName string    `json:"agent_name"`
	Content   any       `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// MessageStore is a per-room ring buffer of recent messages for polling-based agents.
// Thread-safe via RWMutex.
type MessageStore struct {
	mu         sync.RWMutex
	rooms      map[RoomID][]StoredMessage
	counters   map[RoomID]int
	maxPerRoom int
}

// NewMessageStore creates a store that keeps the last maxPerRoom messages per room.
func NewMessageStore(maxPerRoom int) *MessageStore {
	return &MessageStore{
		rooms:      make(map[RoomID][]StoredMessage),
		counters:   make(map[RoomID]int),
		maxPerRoom: maxPerRoom,
	}
}

// Append adds a message to the room's buffer. Returns the message ID.
func (s *MessageStore) Append(roomID RoomID, agentName string, content any) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.counters[roomID]++
	id := s.counters[roomID]

	msg := StoredMessage{
		ID:        id,
		AgentName: agentName,
		Content:   content,
		Timestamp: time.Now(),
	}

	msgs := s.rooms[roomID]
	msgs = append(msgs, msg)
	if len(msgs) > s.maxPerRoom {
		msgs = msgs[len(msgs)-s.maxPerRoom:]
	}
	s.rooms[roomID] = msgs

	return id
}

// Since returns all messages with ID > afterID for the given room.
func (s *MessageStore) Since(roomID RoomID, afterID int) []StoredMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	msgs := s.rooms[roomID]
	var result []StoredMessage
	for _, m := range msgs {
		if m.ID > afterID {
			result = append(result, m)
		}
	}
	if result == nil {
		result = []StoredMessage{}
	}
	return result
}
