// Command consumer is an external consumer of the Solvr API: a program of its own module
// that knows Solvr only through the published Go SDK (packages/sdk-go) and reaches the API
// over HTTP alone. The backend's external consumer tests run it against the real API on a
// clean schema and on a schema upgraded from production's, then hold what it reports to
// what the API stored.
//
//	consumer collaborate   a planner and an executor work in a new room, the planner posts,
//	                       both reply; with SOLVR_EXISTING_POST / SOLVR_EXISTING_ROOM the
//	                       executor also continues that post and joins that room
//	consumer members       the planner opens a closed room and admits every later agent to
//	                       it; all join, the planner addresses the participants it listed,
//	                       and each answers
//	consumer find          search each query in SOLVR_FIND ("|"-separated)
//
// Environment: SOLVR_API_URL (the API's base URL), SOLVR_API_KEYS (the planner's, the
// executor's and any later agent's API keys, comma-separated), SOLVR_AGENT_IDS (the agent
// ids of those keys, in order; members needs it and at least three agents), SOLVR_RUN (one
// word naming this run; it is in every text the consumer writes).
//
// It prints one JSON report on stdout: what the SDK surfaced at each step. A step that
// fails ends the run with exit status 1 and the report names the step and the error.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	solvr "github.com/fcavalcantirj/solvr/packages/sdk-go"
)

// report is the consumer's output.
type report struct {
	Phase      string `json:"phase"`
	FailedStep string `json:"failed_step,omitempty"`
	Error      string `json:"error,omitempty"`
	ErrorCode  string `json:"error_code,omitempty"`

	Room        *roomReport      `json:"room,omitempty"`
	Post        *postReport      `json:"post,omitempty"`
	Replies     []replyReport    `json:"replies,omitempty"`
	ReplyPages  []pageReport     `json:"reply_pages,omitempty"`
	Vote        *solvr.ReplyVote `json:"vote,omitempty"`
	MissingPost *errorReport     `json:"missing_post,omitempty"`
	Existing    *existingReport  `json:"existing,omitempty"`
	Searches    []searchReport   `json:"searches,omitempty"`
	Members     *membersReport   `json:"members,omitempty"`
}

type roomReport struct {
	Slug       string            `json:"slug"`
	Handshakes []handshakeReport `json:"handshakes"`
	Entries    []entryReport     `json:"entries"`  // what each agent sent
	Timeline   []entryReport     `json:"timeline"` // the message timeline, read one entry per page
	Pages      int               `json:"pages"`
	Stream     []entryReport     `json:"stream"` // the message frames the planner watched
	Live       entryReport       `json:"live"`   // the entry sent while the planner watched
}

type handshakeReport struct {
	AgentID     string `json:"agent_id"`
	RoomSlug    string `json:"room_slug"`
	TokenPrefix string `json:"token_prefix"` // the token itself stays with the consumer
}

type entryReport struct {
	ID             int64  `json:"id"`
	AuthorID       string `json:"author_id"`
	Body           string `json:"body"`
	ReplyToEntryID *int64 `json:"reply_to_entry_id"`
}

type postReport struct {
	ID               string `json:"id"`
	Type             string `json:"type"`
	Title            string `json:"title"`
	Status           string `json:"status"`
	PublicationState string `json:"publication_state"`
	ModerationState  string `json:"moderation_state"`
	AuthorID         string `json:"author_id"`
}

type replyReport struct {
	ID            string  `json:"id"`
	ParentReplyID *string `json:"parent_reply_id"`
	AuthorID      string  `json:"author_id"`
	Body          string  `json:"body"`
	LegacyType    *string `json:"legacy_type"`
	LegacyID      *string `json:"legacy_id"`
}

type pageReport struct {
	IDs        []string `json:"ids"`
	Total      int      `json:"total"`
	HasMore    bool     `json:"has_more"`
	NextCursor bool     `json:"next_cursor"`
}

type errorReport struct {
	Status int    `json:"status"`
	Code   string `json:"code"`
}

type existingReport struct {
	Post     postReport          `json:"post"`
	Replies  []replyReport       `json:"replies"`
	FollowUp replyReport         `json:"follow_up"`
	Room     *existingRoomReport `json:"room,omitempty"`
}

type existingRoomReport struct {
	AgentID  string        `json:"agent_id"`
	Timeline []entryReport `json:"timeline"`
	Sent     entryReport   `json:"sent"`
}

type searchReport struct {
	Query   string         `json:"query"`
	Total   int            `json:"total"`
	Results []resultReport `json:"results"`
}

type resultReport struct {
	ID             string   `json:"id"`
	Type           string   `json:"type"`
	Title          string   `json:"title"`
	MatchedReplies []string `json:"matched_replies"`
}

// failure is a step that did not succeed.
type failure struct {
	step string
	err  error
}

func (f *failure) Error() string { return f.step + ": " + f.err.Error() }

func (f *failure) Unwrap() error { return f.err }

func step(name string, err error) error {
	if err == nil {
		return nil
	}
	return &failure{step: name, err: err}
}

// consumer holds the clients the run works with.
type consumer struct {
	run                string
	planner, executor  *solvr.Client
	agents             []*solvr.Client // every key's client, the planner first
	agentIDs           []string        // SOLVR_AGENT_IDS
	anonymous          *solvr.Client
	existingPost, room string
	find               []string
}

func main() {
	phase := ""
	if len(os.Args) > 1 {
		phase = os.Args[1]
	}
	rep := &report{Phase: phase}
	err := runPhase(phase, rep)
	if err != nil {
		rep.Error = err.Error()
		var f *failure
		if errors.As(err, &f) {
			rep.FailedStep, rep.Error = f.step, f.err.Error()
		}
		var apiErr *solvr.APIError
		if errors.As(err, &apiErr) {
			rep.ErrorCode = apiErr.Code
		}
	}
	if encErr := json.NewEncoder(os.Stdout).Encode(rep); encErr != nil {
		fmt.Fprintln(os.Stderr, "write the report:", encErr)
		os.Exit(1)
	}
	if err != nil {
		os.Exit(1)
	}
}

func runPhase(phase string, rep *report) error {
	c, err := newConsumer()
	if err != nil {
		return step("configure", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	switch phase {
	case "collaborate":
		return c.collaborate(ctx, rep)
	case "members":
		return c.members(ctx, rep)
	case "find":
		return c.search(ctx, rep)
	default:
		return step("configure", fmt.Errorf("unknown phase %q: use collaborate, members or find", phase))
	}
}

func newConsumer() (*consumer, error) {
	base := os.Getenv("SOLVR_API_URL")
	keys := strings.Split(os.Getenv("SOLVR_API_KEYS"), ",")
	run := os.Getenv("SOLVR_RUN")
	if base == "" || len(keys) < 2 || run == "" {
		return nil, errors.New("SOLVR_API_URL, SOLVR_API_KEYS (planner,executor[,...]) and SOLVR_RUN are required")
	}
	client := func(key string) *solvr.Client {
		return solvr.NewClient(key, solvr.WithBaseURL(base), solvr.WithMaxRetries(0), solvr.WithTimeout(15*time.Second))
	}
	c := &consumer{
		run:          run,
		anonymous:    client(""),
		existingPost: os.Getenv("SOLVR_EXISTING_POST"),
		room:         os.Getenv("SOLVR_EXISTING_ROOM"),
	}
	for _, key := range keys {
		if key == "" {
			return nil, errors.New("SOLVR_API_KEYS has an empty key")
		}
		c.agents = append(c.agents, client(key))
	}
	c.planner, c.executor = c.agents[0], c.agents[1]
	if ids := os.Getenv("SOLVR_AGENT_IDS"); ids != "" {
		c.agentIDs = strings.Split(ids, ",")
	}
	if find := os.Getenv("SOLVR_FIND"); find != "" {
		c.find = strings.Split(find, "|")
	}
	return c, nil
}

// search runs each query of SOLVR_FIND anonymously.
func (c *consumer) search(ctx context.Context, rep *report) error {
	if len(c.find) == 0 {
		return step("configure", errors.New("SOLVR_FIND names no query"))
	}
	for _, q := range c.find {
		res, err := c.anonymous.Search(ctx, q, nil)
		if err := step("search "+q, err); err != nil {
			return err
		}
		s := searchReport{Query: q, Total: res.Meta.Total, Results: []resultReport{}}
		for _, r := range res.Data {
			matched := []string{}
			for _, m := range r.MatchedReplies {
				matched = append(matched, m.ID)
			}
			s.Results = append(s.Results, resultReport{ID: r.ID, Type: r.Type, Title: r.Title, MatchedReplies: matched})
		}
		rep.Searches = append(rep.Searches, s)
	}
	return nil
}

func entryOf(e solvr.RoomEntry) entryReport {
	return entryReport{ID: e.ID, AuthorID: e.AuthorID, Body: e.Body, ReplyToEntryID: e.ReplyToEntryID}
}

func postOf(p solvr.Post) postReport {
	return postReport{ID: p.ID, Type: p.Type, Title: p.Title, Status: p.Status, PublicationState: p.PublicationState,
		ModerationState: p.ModerationState, AuthorID: p.PostedByID}
}

func replyOf(r solvr.Reply) replyReport {
	return replyReport{ID: r.ID, ParentReplyID: r.ParentReplyID, AuthorID: r.AuthorID, Body: r.Body,
		LegacyType: r.LegacyType, LegacyID: r.LegacyID}
}
