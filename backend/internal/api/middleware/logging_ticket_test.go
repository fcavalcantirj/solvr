package middleware

import (
	"strings"
	"testing"
)

// A stream ticket rides in the stream URL; it expires within a minute, but the access log
// must still never carry it.
func TestRedactURLPath_RedactsTheStreamTicket(t *testing.T) {
	got := RedactURLPath("/v1/rooms/x/stream?ticket=solvr_st_abcdef.signature&lastEventId=42")
	if strings.Contains(got, "solvr_st_abcdef") {
		t.Errorf("the ticket reached the log line: %s", got)
	}
	if !strings.Contains(got, "lastEventId=42") {
		t.Errorf("other parameters must survive: %s", got)
	}
}
