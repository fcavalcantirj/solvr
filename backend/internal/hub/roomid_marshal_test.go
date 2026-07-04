package hub

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRoomID_MarshalJSON_IsUUIDString(t *testing.T) {
	id := uuid.MustParse("902a1b3c-4d5e-6f70-8192-a3b4c5d6e7f8")
	rid := NewRoomID(id)

	out, err := json.Marshal(rid)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := `"902a1b3c-4d5e-6f70-8192-a3b4c5d6e7f8"`
	if string(out) != want {
		t.Fatalf("RoomID JSON = %s; want %s (not a raw byte array)", out, want)
	}
	// Guard against regression to the [16]byte array encoding.
	if strings.HasPrefix(string(out), "[") {
		t.Fatalf("RoomID marshaled as an array: %s", out)
	}
}

func TestRoomID_JSONRoundTrip(t *testing.T) {
	id := uuid.New()
	rid := NewRoomID(id)
	data, err := json.Marshal(rid)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var back RoomID
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if back.UUID() != id {
		t.Fatalf("round-trip mismatch: %v != %v", back.UUID(), id)
	}
}

func TestRoomEvent_RoomIDIsString(t *testing.T) {
	evt := RoomEvent{
		Type:      EventMessage,
		RoomID:    NewRoomID(uuid.MustParse("11111111-2222-3333-4444-555555555555")),
		Timestamp: time.Unix(0, 0).UTC(),
	}
	out, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("Marshal event: %v", err)
	}
	if !strings.Contains(string(out), `"room_id":"11111111-2222-3333-4444-555555555555"`) {
		t.Fatalf("event room_id not a UUID string: %s", out)
	}
}
