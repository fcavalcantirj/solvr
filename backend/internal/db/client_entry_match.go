package db

import (
	"bytes"
	"encoding/json"
	"reflect"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// A client_entry_id retry replays the stored entry only when it carries the same write.
// These comparisons define "the same write": the content-bearing fields of the entry,
// with defaults applied (content_type "text", extension {}) and JSON compared by value
// so key order and whitespace do not matter. The display label is not compared: the
// author is already fixed by the credential that scopes the key.

// sameMessageWrite reports whether params would store the message that is already stored.
func sameMessageWrite(stored *models.Message, params models.CreateMessageParams) bool {
	return stored.Content == params.Content &&
		orText(stored.ContentType) == orText(params.ContentType) &&
		sameJSON(stored.Metadata, params.Metadata, `{}`) &&
		sameRef(stored.ReplyToEntryID, params.ReplyToEntryID) &&
		sameRef(stored.SupersedesEntryID, params.SupersedesEntryID) &&
		sameJSON(stored.AddressedMemberIDs, params.AddressedMemberIDs, `null`)
}

// sameEntryWrite reports whether params would store the timeline entry that is already stored.
func sameEntryWrite(stored *models.RoomEntry, params models.CreateRoomEntryParams) bool {
	return stored.Kind == params.Kind &&
		deref(stored.Body) == deref(params.Body) &&
		orText(stored.ContentType) == orText(params.ContentType) &&
		deref(stored.EventType) == deref(params.EventType) &&
		stored.Issue == params.Issue &&
		sameJSON(stored.Extension, params.Extension, `{}`) &&
		sameRef(stored.ReplyToEntryID, params.ReplyToEntryID) &&
		sameRef(stored.SupersedesEntryID, params.SupersedesEntryID) &&
		sameJSON(stored.AddressedMemberIDs, params.AddressedMemberIDs, `null`)
}

func orText(contentType string) string {
	if contentType == "" {
		return "text"
	}
	return contentType
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func sameRef(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// sameJSON compares two JSON documents by value; an absent document stands for def.
// Unparseable input never matches.
func sameJSON(a, b json.RawMessage, def string) bool {
	va, okA := decodeJSON(a, def)
	vb, okB := decodeJSON(b, def)
	return okA && okB && reflect.DeepEqual(va, vb)
}

func decodeJSON(raw json.RawMessage, def string) (any, bool) {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = json.RawMessage(def)
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, false
	}
	return v, true
}
