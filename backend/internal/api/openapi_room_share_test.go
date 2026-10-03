package api

import "testing"

// idx 88: the share contract is published with the room operations, and the room
// schemas carry the source and try-this fields the handlers return.
func TestOpenAPI_RoomShareOperationAndSourceFieldsArePublished(t *testing.T) {
	spec := servedSpec(t)
	op := operation(t, spec, "get", "/rooms/{slug}/share")
	responses := op["responses"].(map[string]interface{})
	for _, code := range []string{"200", "403", "404", "409"} {
		if _, ok := responses[code]; !ok {
			t.Errorf("GET /rooms/{slug}/share does not document %s", code)
		}
	}
	share := map[string]bool{}
	for _, p := range propertyNames(t, spec, "RoomShare") {
		share[p] = true
	}
	for _, f := range []string{"room_url", "share_url", "try_url", "excerpt", "copy_text", "note"} {
		if !share[f] {
			t.Errorf("RoomShare lacks %s", f)
		}
	}
	create := map[string]bool{}
	for _, p := range propertyNames(t, spec, "CreateRoomRequest") {
		create[p] = true
	}
	if !create["source_room"] {
		t.Error("CreateRoomRequest lacks source_room")
	}
}
