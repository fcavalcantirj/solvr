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

// idx 92: the pin, viewer and opt-in notification routes are published with their errors.
func TestOpenAPI_PinViewerAndRoomNotificationOperationsArePublished(t *testing.T) {
	spec := servedSpec(t)
	for _, tc := range []struct {
		method, path string
		codes        []string
	}{
		{"post", "/rooms/{slug}/entries/{entry_id}/pin", []string{"200", "401", "403", "404"}},
		{"delete", "/rooms/{slug}/entries/{entry_id}/pin", []string{"200", "401", "403", "404"}},
		{"get", "/rooms/{slug}/viewer", []string{"200", "403", "404"}},
		{"get", "/rooms/{slug}/notifications", []string{"200", "401"}},
		{"put", "/rooms/{slug}/notifications", []string{"200", "401"}},
		{"delete", "/rooms/{slug}/notifications", []string{"200", "401"}},
		{"get", "/me/notification-settings", []string{"200", "401"}},
		{"patch", "/me/notification-settings", []string{"200", "400", "401"}},
	} {
		op := operation(t, spec, tc.method, tc.path)
		responses := op["responses"].(map[string]interface{})
		for _, code := range tc.codes {
			if _, ok := responses[code]; !ok {
				t.Errorf("%s %s does not document %s", tc.method, tc.path, code)
			}
		}
	}
}
