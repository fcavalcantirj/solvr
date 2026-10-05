package handlers

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// POST /v1/rooms keeps a flow_id only when it is a well-formed flow code that an earlier
// funnel step already carries (SPEC.md 25.7). Everything else is dropped without a word:
// the room is created exactly as without it.

func TestAttributableFlowCode_KeepsOnlyAWellFormedKnownCode(t *testing.T) {
	var asked []string
	lookup := func(known bool, err error) func(string) (bool, error) {
		return func(code string) (bool, error) {
			asked = append(asked, code)
			return known, err
		}
	}

	// Known: kept.
	code, err := attributableFlowCode("k7m2p9xq", lookup(true, nil))
	require.NoError(t, err)
	require.Equal(t, "k7m2p9xq", code)
	require.Equal(t, []string{"k7m2p9xq"}, asked)

	// Well-formed but no earlier step carries it: dropped.
	code, err = attributableFlowCode("k7m2p9xq", lookup(false, nil))
	require.NoError(t, err)
	require.Equal(t, "", code)

	// The lookup failed: dropped too, and the failure is handed back to be logged.
	boom := errors.New("database is away")
	code, err = attributableFlowCode("k7m2p9xq", lookup(true, boom))
	require.ErrorIs(t, err, boom)
	require.Equal(t, "", code, "a failed lookup never keeps the code, whatever it answered")
}

// A value that is not exactly a code is dropped before anything is looked up.
func TestAttributableFlowCode_NeverLooksUpAMalformedValue(t *testing.T) {
	for _, sent := range []string{
		"", "f_0123456789abcdef01234567", "K7M2P9XQ", "k7m2p9x", "k7m2p9xqq", "k7m2p9xi", " k7m2p9xq", "k7m2p9xq\n",
		"null", `{"a":1}`, "'; DROP TABLE funnel_events; --",
	} {
		code, err := attributableFlowCode(sent, func(string) (bool, error) {
			t.Fatalf("looked up the malformed value %q", sent)
			return true, nil
		})
		require.NoError(t, err, sent)
		require.Equal(t, "", code, sent)
	}
}

// The create body is never refused because of what flow_id holds: any JSON decodes, and
// only a string (or a bare number that spells a code) can become a candidate.
func TestCreateRoomRequest_AnyFlowIDDecodes(t *testing.T) {
	for body, want := range map[string]string{
		`{"display_name":"x"}`:                         "",
		`{"display_name":"x","flow_id":"k7m2p9xq"}`:    "k7m2p9xq",
		`{"display_name":"x","flow_id":""}`:            "",
		`{"display_name":"x","flow_id":null}`:          "",
		`{"display_name":"x","flow_id":23456789}`:      "23456789",
		`{"display_name":"x","flow_id":2.5}`:           "2.5",
		`{"display_name":"x","flow_id":true}`:          "true",
		`{"display_name":"x","flow_id":["k7m2p9xq"]}`:  `["k7m2p9xq"]`,
		`{"display_name":"x","flow_id":{"f":"k7m2"}}`:  `{"f":"k7m2"}`,
		`{"display_name":"x","flow_id":"f_legacy_id"}`: "f_legacy_id",
	} {
		var req createRoomRequest
		require.NoError(t, json.Unmarshal([]byte(body), &req), body)
		require.Equal(t, "x", req.DisplayName, body)
		require.Equal(t, want, string(req.FlowID), body)
	}
}
