package api

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/hub"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// idx 78 step 2 (watch): the room stream publishes the JSON its frames carry and a recorded
// frame, so a client parses the stream from the same contract it reads the other operations
// from.

// parseSSE reads the frames (sseFrame: id, event, data lines joined) of an event stream;
// comments and retry directives are skipped.
func parseSSE(text string) []sseFrame {
	var frames []sseFrame
	var cur sseFrame
	var data []string
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		switch {
		case line == "":
			if cur.ID != 0 || cur.Event != "" || len(data) > 0 {
				cur.Data = strings.Join(data, "\n")
				frames = append(frames, cur)
			}
			cur, data = sseFrame{}, nil
		case strings.HasPrefix(line, "id: "):
			cur.ID, _ = strconv.ParseInt(strings.TrimPrefix(line, "id: "), 10, 64)
		case strings.HasPrefix(line, "event: "):
			cur.Event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			data = append(data, strings.TrimPrefix(line, "data: "))
		}
	}
	return frames
}

func TestOpenAPIStream_FrameSchemasDescribeTheJSONTheStreamWrites(t *testing.T) {
	spec := servedSpec(t)
	for schema, typ := range map[string]reflect.Type{
		"RoomStreamFrame":   reflect.TypeOf(hub.RoomEvent{}),
		"RoomStreamMessage": reflect.TypeOf(models.Message{}),
	} {
		want := jsonFields(typ)
		sort.Strings(want)
		sameSet(t, schema+" properties", propertyNames(t, spec, schema), want)
	}
	media := at(t, operation(t, spec, "get", "/rooms/{slug}/stream"), "responses", "200", "content", "text/event-stream")
	assert.Equal(t, "#/components/schemas/RoomStreamFrame", at(t, media, "x-solvr-frame-schema", "$ref"),
		"the stream names the schema of its frames' data")
}

// The published stream example is a reconnect: Last-Event-ID is the entry createRoomEntry's
// example stored, and the stream replays the room's next entry as one message frame whose
// data is a RoomStreamFrame carrying a RoomStreamMessage.
func TestOpenAPIStream_TheExampleIsAReplayedFrameOfItsSchema(t *testing.T) {
	spec := servedSpec(t)
	examples := publishedExamples(t, spec)
	ex, ok := examples["streamRoom"]
	require.True(t, ok, "streamRoom publishes no example")
	assert.Equal(t, "text/event-stream", ex.MediaType)
	assert.Equal(t, "room_token", ex.Credential)
	assert.Equal(t, "200", ex.Status)

	stored := examples["createRoomEntry"].Response.(map[string]interface{})["data"].(map[string]interface{})
	assert.Equal(t, fmt.Sprint(stored["id"]), fmt.Sprint(ex.Headers["Last-Event-ID"]),
		"the reconnect resumes after the entry createRoomEntry stored")

	text, ok := ex.Response.(string)
	require.True(t, ok, "the stream example is the event-stream text, got %T", ex.Response)
	assert.True(t, strings.HasSuffix(text, "\n\n"), "a frame ends with a blank line")
	frames := parseSSE(text)
	require.Len(t, frames, 1)
	frame := frames[0]
	assert.Equal(t, "message", frame.Event)

	var data map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(frame.Data), &data))
	assert.Empty(t, schemaProblems(spec, data, ref("schemas", "RoomStreamFrame"), "frame"))
	assert.Empty(t, schemaProblems(spec, data["payload"], ref("schemas", "RoomStreamMessage"), "frame.payload"))
	payload := data["payload"].(map[string]interface{})
	assert.Equal(t, float64(frame.ID), data["id"], "the SSE id is the entry id")
	assert.Equal(t, data["id"], payload["id"])
	assert.Equal(t, "message", data["type"])
	assert.Equal(t, stored["sequence"].(float64)+1, data["sequence"], "the replay starts at the next entry")
	assert.Equal(t, data["sequence"], payload["sequence_num"])
	assert.Equal(t, stored["room_id"], data["room_id"])
	assert.Equal(t, stored["room_id"], payload["room_id"])
}

func TestParseSSE_ReadsFramesAndSkipsCommentsAndRetries(t *testing.T) {
	frames := parseSSE(": heartbeat\n\nid: 7\nevent: message\ndata: {\"a\":1}\n\nretry: 1000\n\nevent: access_revoked\ndata: x\ndata: y\n\n")
	assert.Equal(t, []sseFrame{{ID: 7, Event: "message", Data: `{"a":1}`}, {Event: "access_revoked", Data: "x\ny"}}, frames)
}
