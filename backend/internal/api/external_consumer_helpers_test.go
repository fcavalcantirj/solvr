package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/hub"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

// The external consumer (contract/consumer) is a program of its own Go module that knows
// Solvr only through the published Go SDK: it never imports the backend and reaches the API
// over HTTP alone. These helpers give it a real API on a real schema: a scratch database
// built from the migration chain, the real router, and agents registered the way any client
// registers them.

// productionSchemaVersion is the schema production runs before the redesign: the restored
// production dumps of 2026-09-29 (cutover rehearsal, idx 93) and of the post-purge copy
// (idx 77) were both at migration 84. An upgraded schema starts there.
const productionSchemaVersion = 84

// consumerSchemaFiles lists every up migration in order.
func consumerSchemaFiles(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("../../migrations/*.up.sql")
	require.NoError(t, err)
	require.NotEmpty(t, files, "no up migrations found")
	sort.Strings(files)
	return files
}

// newConsumerScratchSchema creates an empty database next to DATABASE_URL's and builds its
// schema from the migration chain: the first `before` migrations, then seed (the rows an
// older release wrote, in that release's schema), then every remaining migration. The
// database is dropped when the test ends.
func newConsumerScratchSchema(t *testing.T, before int, seed func(ctx context.Context, conn *pgx.Conn)) string {
	t.Helper()
	base := os.Getenv("DATABASE_URL")
	if base == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	admin, err := pgx.Connect(ctx, base)
	require.NoError(t, err, "connect admin")
	name := fmt.Sprintf("solvr_scratch_consumer_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		admin.Close(ctx)
		t.Fatalf("create scratch database: %v", err)
	}
	t.Cleanup(func() {
		c, cc := context.WithTimeout(context.Background(), 30*time.Second)
		defer cc()
		if _, err := admin.Exec(c, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("drop scratch database %s: %v", name, err)
		}
		admin.Close(c)
	})

	u, err := url.Parse(base)
	require.NoError(t, err)
	u.Path = "/" + name
	conn, err := pgx.Connect(ctx, u.String())
	require.NoError(t, err, "connect scratch")
	defer conn.Close(ctx)

	files := consumerSchemaFiles(t)
	require.LessOrEqual(t, before, len(files))
	apply := func(files []string) {
		for _, f := range files {
			sql, err := os.ReadFile(f)
			require.NoError(t, err)
			if _, err := conn.Exec(ctx, string(sql)); err != nil {
				t.Fatalf("apply %s: %v", filepath.Base(f), err)
			}
		}
	}
	apply(files[:before])
	if seed != nil {
		seed(ctx, conn)
	}
	apply(files[before:])
	return u.String()
}

// serveConsumerSchema serves the real router over the scratch database.
func serveConsumerSchema(t *testing.T, dbURL string) (*httptest.Server, *db.Pool) {
	t.Helper()
	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	require.NoError(t, err, "pool on the scratch database")
	t.Cleanup(pool.Close) // registered after the drop, so it runs before it
	registry := hub.NewPresenceRegistry()
	hubMgr := hub.NewHubManager(ctx, registry, slog.Default(), 0)
	ts := httptest.NewServer(NewRouter(pool, hubMgr, registry))
	t.Cleanup(ts.Close)
	return ts, pool
}

// registerConsumerAgent registers an agent through the API and returns its id and API key.
func registerConsumerAgent(t *testing.T, baseURL, name string) (id, key string) {
	t.Helper()
	resp, err := http.Post(baseURL+"/v1/agents/register", "application/json",
		strings.NewReader(fmt.Sprintf(`{"name":%q}`, name)))
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "register %s: %s", name, raw)
	var out struct {
		Agent struct {
			ID string `json:"id"`
		} `json:"agent"`
		APIKey string `json:"api_key"`
	}
	require.NoError(t, json.Unmarshal(raw, &out), string(raw))
	require.NotEmpty(t, out.Agent.ID, string(raw))
	require.NotEmpty(t, out.APIKey, string(raw))
	return out.Agent.ID, out.APIKey
}

// buildExternalConsumer compiles contract/consumer in its own module, against the SDK
// source in this tree, and returns the binary.
func buildExternalConsumer(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "..", "contract", "consumer"))
	require.NoError(t, err)
	bin := filepath.Join(t.TempDir(), "solvr-consumer")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", bin, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "build the external consumer in %s: %s", dir, out)
	return bin
}

// consumerReport is what the consumer printed: what the SDK surfaced at each step.
type consumerReport struct {
	Phase      string `json:"phase"`
	FailedStep string `json:"failed_step"`
	Error      string `json:"error"`
	ErrorCode  string `json:"error_code"`

	Room struct {
		Slug       string              `json:"slug"`
		Handshakes []consumerHandshake `json:"handshakes"`
		Entries    []consumerEntry     `json:"entries"`
		Timeline   []consumerEntry     `json:"timeline"`
		Pages      int                 `json:"pages"`
		Stream     []consumerEntry     `json:"stream"`
		Live       consumerEntry       `json:"live"`
	} `json:"room"`
	Post struct {
		ID               string `json:"id"`
		Type             string `json:"type"`
		Status           string `json:"status"`
		PublicationState string `json:"publication_state"`
		ModerationState  string `json:"moderation_state"`
		AuthorID         string `json:"author_id"`
	} `json:"post"`
	Replies    []consumerReply `json:"replies"`
	ReplyPages []struct {
		IDs        []string `json:"ids"`
		Total      int      `json:"total"`
		HasMore    bool     `json:"has_more"`
		NextCursor bool     `json:"next_cursor"`
	} `json:"reply_pages"`
	Vote struct {
		Voted     bool   `json:"voted"`
		Direction string `json:"direction"`
	} `json:"vote"`
	MissingPost struct {
		Status int    `json:"status"`
		Code   string `json:"code"`
	} `json:"missing_post"`

	Existing *struct {
		Post struct {
			ID    string `json:"id"`
			Type  string `json:"type"`
			Title string `json:"title"`
		} `json:"post"`
		Replies  []consumerReply `json:"replies"`
		FollowUp consumerReply   `json:"follow_up"`
		Room     *struct {
			AgentID  string          `json:"agent_id"`
			Timeline []consumerEntry `json:"timeline"`
			Sent     consumerEntry   `json:"sent"`
		} `json:"room"`
	} `json:"existing"`

	Searches []struct {
		Query   string `json:"query"`
		Total   int    `json:"total"`
		Results []struct {
			ID             string   `json:"id"`
			Type           string   `json:"type"`
			Title          string   `json:"title"`
			MatchedReplies []string `json:"matched_replies"`
		} `json:"results"`
	} `json:"searches"`

	Members *consumerMembers `json:"members"`
}

type consumerHandshake struct {
	AgentID     string `json:"agent_id"`
	RoomSlug    string `json:"room_slug"`
	TokenPrefix string `json:"token_prefix"`
}

type consumerEntry struct {
	ID             int64  `json:"id"`
	AuthorID       string `json:"author_id"`
	Body           string `json:"body"`
	ReplyToEntryID *int64 `json:"reply_to_entry_id"`
}

type consumerReply struct {
	ID            string  `json:"id"`
	ParentReplyID *string `json:"parent_reply_id"`
	AuthorID      string  `json:"author_id"`
	Body          string  `json:"body"`
	LegacyType    *string `json:"legacy_type"`
	LegacyID      *string `json:"legacy_id"`
}

// runExternalConsumer runs one phase of the consumer with only the environment it is
// given (no ambient credential reaches it) and returns its report. A phase that fails
// fails the test with the step and the API error code it reported.
func runExternalConsumer(t *testing.T, bin, phase string, env map[string]string) consumerReport {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, phase)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	var rep consumerReport
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &rep),
		"the consumer's %s report is not JSON (exit: %v): %s; stderr: %s", phase, runErr, stdout.String(), stderr.String())
	require.NoError(t, runErr, "consumer %s failed at %q: %s (code %q); stderr: %s",
		phase, rep.FailedStep, rep.Error, rep.ErrorCode, stderr.String())
	require.Equal(t, phase, rep.Phase)
	return rep
}

// approveConsumerPost plays the moderator: UpdateStatus(open) is the only path that
// publishes a post and approves it (BART-583); without a moderation service configured a
// new post waits in pending_review, where search does not show it.
func approveConsumerPost(t *testing.T, pool *db.Pool, postID string) {
	t.Helper()
	require.NoError(t, db.NewPostRepository(pool).UpdateStatus(context.Background(), postID, models.PostStatusOpen))
}
