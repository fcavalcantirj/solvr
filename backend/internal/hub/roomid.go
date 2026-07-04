package hub

import "github.com/google/uuid"

// RoomID is a type-safe wrapper around uuid.UUID to prevent cross-room contamination bugs.
// Using a named type forces callers to explicitly convert, making accidental room-ID mix-ups
// a compile-time error rather than a runtime bug.
type RoomID uuid.UUID

// NewRoomID wraps a uuid.UUID as a RoomID.
func NewRoomID(id uuid.UUID) RoomID { return RoomID(id) }

// UUID returns the underlying uuid.UUID.
func (r RoomID) UUID() uuid.UUID { return uuid.UUID(r) }

// String returns the canonical UUID string representation.
func (r RoomID) String() string { return uuid.UUID(r).String() }

// MarshalText encodes the RoomID as its canonical UUID string.
//
// Without this, RoomID — a named type over uuid.UUID (which is [16]byte) — does NOT
// inherit uuid.UUID's MarshalText, so encoding/json falls back to serializing it as a
// raw 16-element byte array. That was the bug that made SSE room_id fields appear as
// "[144,42,...]" instead of "902a...". encoding/json uses TextMarshaler when no
// MarshalJSON is present, so defining MarshalText fixes every place a RoomEvent (or any
// struct embedding a RoomID) is marshaled.
func (r RoomID) MarshalText() ([]byte, error) {
	return uuid.UUID(r).MarshalText()
}

// UnmarshalText decodes a canonical UUID string into a RoomID, keeping the type
// round-trippable through JSON and other text encodings.
func (r *RoomID) UnmarshalText(data []byte) error {
	var id uuid.UUID
	if err := id.UnmarshalText(data); err != nil {
		return err
	}
	*r = RoomID(id)
	return nil
}

// ParseRoomID parses a UUID string into a RoomID.
// Returns an error if the string is not a valid UUID.
func ParseRoomID(s string) (RoomID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return RoomID{}, err
	}
	return RoomID(id), nil
}
