package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// idx 75 step 5: "copied public links cannot expose account or room secrets". A room
// route accepted the per-agent room token as ?token= on every route, so a link a person
// copied out of a client could carry a live credential. The room routes are now
// header-only; a credential in the URL is refused with a 400 that names the header.

func TestURLCredentials_AreRefusedOnEveryRoomRoute(t *testing.T) {
	a := startRoomInstance(t, RoomRelayOptions{SweepInterval: -1})
	priv := newAccessRoom(t, a, true)
	pubSlug := entriesTestRoom(t, a.ts.URL, priv.ownerJWT, false)
	base := a.ts.URL

	tok, jwt := priv.executorTok, priv.ownerJWT
	for name, url := range map[string]string{
		"canonical entries, room token":          base + "/v1/rooms/" + priv.slug + "/entries?token=" + tok,
		"canonical entries, JWT as access_token": base + "/v1/rooms/" + priv.slug + "/entries?access_token=" + jwt,
		"canonical messages":                     base + "/v1/rooms/" + priv.slug + "/messages?token=" + tok,
		"room detail":                            base + "/v1/rooms/" + priv.slug + "?token=" + tok,
		"connect prompt":                         base + "/v1/rooms/" + priv.slug + "/connect?token=" + tok,
		"participants":                           base + "/v1/rooms/" + priv.slug + "/agents?token=" + tok,
		"a PUBLIC room too":                      base + "/v1/rooms/" + pubSlug + "/entries?token=" + tok,
		"adapter messages":                       base + "/r/" + priv.slug + "/messages?token=" + tok,
		"adapter participants":                   base + "/r/" + priv.slug + "/agents?token=" + tok,
		"adapter events":                         base + "/r/" + priv.slug + "/events?token=" + tok,
		"adapter stream":                         base + "/r/" + priv.slug + "/stream?token=" + tok,
	} {
		status, out := doJSON(t, "GET", url, "", "")
		require.Equal(t, http.StatusBadRequest, status, "%s: %v", name, out)
		code, msg, reqID := sessionError(t, out)
		require.Equal(t, "VALIDATION_ERROR", code, name)
		require.Contains(t, msg, "Authorization", "%s: the answer names the header", name)
		require.NotContains(t, msg, tok, "%s: the refusal never echoes the credential", name)
		require.NotContains(t, msg, jwt, "%s: the refusal never echoes the credential", name)
		require.NotEmpty(t, reqID, name)
	}

	// A write with a credential in the URL is refused as well, before anything is read.
	for name, url := range map[string]string{
		"canonical write": base + "/v1/rooms/" + priv.slug + "/entries?token=" + tok,
		"adapter write":   base + "/r/" + priv.slug + "/message?token=" + tok,
	} {
		status, out := doJSON(t, "POST", url, "", `{"content":"hello","kind":"message"}`)
		require.Equal(t, http.StatusBadRequest, status, "%s: %v", name, out)
	}

	// The header path is untouched, for every credential kind.
	for name, bearer := range map[string]string{"owner JWT": jwt, "member room token": tok} {
		require.Equal(t, http.StatusOK, getStatus(t, base+"/v1/rooms/"+priv.slug+"/entries", bearer), name)
	}
	require.Equal(t, http.StatusOK, getStatus(t, base+"/r/"+priv.slug+"/messages", tok), "adapter, header")
	// A public room stays readable with no credential at all.
	require.Equal(t, http.StatusOK, getStatus(t, base+"/v1/rooms/"+pubSlug+"/entries", ""))
}
