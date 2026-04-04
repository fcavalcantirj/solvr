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

// ParseRoomID parses a UUID string into a RoomID.
// Returns an error if the string is not a valid UUID.
func ParseRoomID(s string) (RoomID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return RoomID{}, err
	}
	return RoomID(id), nil
}
