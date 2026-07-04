package api

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// postRoomMessage posts an A2A message and returns its DB id.
func postRoomMessage(t *testing.T, baseURL string, slug, roomToken, agent, content string) int64 {
	t.Helper()
	body := `{"agent_name":"` + agent + `","content":"` + content + `"}`
	req, _ := http.NewRequest("POST", baseURL+"/r/"+slug+"/message", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+roomToken)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, "post message: %s", string(raw))
	var out map[string]any
	require.NoError(t, json.Unmarshal(raw, &out))
	data, _ := out["data"].(map[string]any)
	idf, _ := data["id"].(float64)
	return int64(idf)
}

// collectSSE opens an SSE stream and collects all "data:" line payloads until the
// context deadline, returning them. Runs the read in the caller's goroutine.
func collectSSE(ctx context.Context, t *testing.T, url, roomToken string, out *[]string, mu *sync.Mutex, ready chan<- struct{}) {
	t.Helper()
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+roomToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		close(ready)
		return
	}
	defer resp.Body.Close()
	close(ready) // headers received -> subscription is being established
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			mu.Lock()
			*out = append(*out, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			mu.Unlock()
		}
	}
}

// TestRoomSSE_ResumeCursorDeliversGap verifies mission #5(b): reconnecting with
// ?after=<id> replays the messages missed while disconnected.
func TestRoomSSE_ResumeCursorDeliversGap(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, jwt := createRoomTestUser(t, pool)
	slug, roomToken := createTestRoomWithToken(t, ts, jwt)

	// Two messages arrive while the consumer is disconnected.
	idA := postRoomMessage(t, ts.URL, slug, roomToken, "w1", "first")
	idB := postRoomMessage(t, ts.URL, slug, roomToken, "w1", "second")
	require.Greater(t, idB, idA)

	// Reconnect with the cursor at idA -> the gap (message B) must be replayed.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var lines []string
	var mu sync.Mutex
	ready := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		collectSSE(ctx, t, ts.URL+"/r/"+slug+"/stream?after="+strconv.FormatInt(idA, 10), roomToken, &lines, &mu, ready)
	}()
	<-ready
	// Replay is synchronous on connect; give it a brief moment, then stop.
	time.Sleep(600 * time.Millisecond)
	cancel()
	<-done

	mu.Lock()
	joined := strings.Join(lines, "\n")
	mu.Unlock()
	require.Contains(t, joined, "\"content\":\"second\"", "gap message B must be replayed")
	require.NotContains(t, joined, "\"content\":\"first\"", "message A (at/behind cursor) must NOT be replayed")
}

// TestRoomSSE_TypeFilterOnlyMatching verifies mission #5(c): a ?type= filtered stream
// emits only matching events.
func TestRoomSSE_TypeFilterOnlyMatching(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, jwt := createRoomTestUser(t, pool)
	slug, roomToken := createTestRoomWithToken(t, ts, jwt)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	var lines []string
	var mu sync.Mutex
	ready := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		collectSSE(ctx, t, ts.URL+"/r/"+slug+"/stream?type=CLAIM", roomToken, &lines, &mu, ready)
	}()
	<-ready
	time.Sleep(300 * time.Millisecond) // let the subscription register

	// A plain message (type=message) must be filtered out; a CLAIM event must pass.
	postRoomMessage(t, ts.URL, slug, roomToken, "chatter", "ignore me")
	postTypedEvent(t, ts.URL+"/r/"+slug+"/events", roomToken, `{"type":"CLAIM","issue":"APP-185","actor":"w1"}`)

	time.Sleep(700 * time.Millisecond)
	cancel()
	<-done

	mu.Lock()
	joined := strings.Join(lines, "\n")
	mu.Unlock()
	require.Contains(t, joined, "APP-185", "CLAIM event must be delivered on ?type=CLAIM stream")
	require.NotContains(t, joined, "ignore me", "non-matching message must be filtered out")
}

func postTypedEvent(t *testing.T, url, roomToken, body string) {
	t.Helper()
	req, _ := http.NewRequest("POST", url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+roomToken)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)
}
