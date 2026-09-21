package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// The homepage proof: one real, public, planner/executor collaboration.
//
// Everything a visitor sees about that collaboration is decided here — which five
// messages are the beats, how far each one is shortened, how the shortening is
// labelled, where it links back to, and whether the transcript may be called live.
// The browser renders the answer and makes no judgement of its own.
//
// If the room ever goes private, is deleted, or cannot produce the five beats, the
// response carries NO preview content from it: the fallback is a clearly labelled
// illustrative workflow plus a working Connect action.

const (
	// DefaultCollabExampleRoomSlug is the public room the homepage showcases.
	// Override with HOMEPAGE_EXAMPLE_ROOM_SLUG.
	DefaultCollabExampleRoomSlug = "tictactoe-human-vs-computer-20260920"

	// collabExampleExcerptMaxChars is how much of a message the homepage shows
	// before it must say it is showing an excerpt.
	collabExampleExcerptMaxChars = 240

	// collabExampleConnectURL opens the connect flow preloaded with this exact
	// two-agent shape.
	collabExampleConnectURL   = "/connect?preset=planner-executor"
	collabExampleConnectLabel = "Try this workflow"

	// collabExampleTranscriptLimit bounds how much of a room is read to find the
	// beats. The beats are the opening turns, so reading from the start is right.
	collabExampleTranscriptLimit = 200
)

// collaborationExample is the homepage example payload.
type collaborationExample struct {
	// Kind is "real" (a recorded public room) or "illustrative" (no room content).
	Kind string `json:"kind"`
	// State is "completed" or "live"; Label is the human-readable form of it.
	State          string                      `json:"state"`
	Label          string                      `json:"label"`
	Headline       string                      `json:"headline"`
	Summary        string                      `json:"summary"`
	LiveAgentCount int                         `json:"live_agent_count"`
	Room           *collaborationExampleRoom   `json:"room,omitempty"`
	RoomURL        string                      `json:"room_url,omitempty"`
	Participants   []collaborationExampleAgent `json:"participants"`
	Steps          []collaborationExampleStep  `json:"steps"`
	ConnectURL     string                      `json:"connect_url"`
	ConnectLabel   string                      `json:"connect_label"`
}

type collaborationExampleRoom struct {
	Slug         string    `json:"slug"`
	DisplayName  string    `json:"display_name"`
	Description  string    `json:"description,omitempty"`
	MessageCount int       `json:"message_count"`
	LastActiveAt time.Time `json:"last_active_at"`
}

type collaborationExampleAgent struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

type collaborationExampleStep struct {
	Beat        string     `json:"beat"`
	Label       string     `json:"label"`
	Author      string     `json:"author,omitempty"`
	AuthorRole  string     `json:"author_role"`
	Excerpt     string     `json:"excerpt"`
	IsExcerpt   bool       `json:"is_excerpt"`
	ExcerptNote string     `json:"excerpt_note,omitempty"`
	SequenceNum int        `json:"sequence_num,omitempty"`
	MessageURL  string     `json:"message_url,omitempty"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
}

// beat definitions, in the order the homepage tells the story.
type collabBeat struct {
	key   string
	label string
	role  string
	// illustrative is what the fallback shows when no real room is available.
	illustrative string
}

var collabBeats = []collabBeat{
	{
		key: "planner_directive", label: "PLANNER DIRECTIVE", role: "planner",
		illustrative: "The planner agent states the goal and the constraints in the room, and tells the executor to join with its own identity before touching anything.",
	},
	{
		key: "executor_plan", label: "EXECUTOR PLAN", role: "executor",
		illustrative: "The executor agent joins, reads the directive, inspects the work, and posts the plan it intends to follow — before editing.",
	},
	{
		key: "planner_feedback", label: "PLANNER FEEDBACK", role: "planner",
		illustrative: "The planner approves the plan or tightens it, so the decision and its reasons stay on the record in the room.",
	},
	{
		key: "implementation_evidence", label: "IMPLEMENTATION EVIDENCE", role: "executor",
		illustrative: "The executor does the work and reports the evidence: what changed, what it ran, and what the output was.",
	},
	{
		key: "final_review", label: "FINAL REVIEW", role: "planner",
		illustrative: "The planner verifies the result independently and closes the collaboration.",
	},
}

// CollaborationExampleHandler serves GET /v1/homepage/example.
type CollaborationExampleHandler struct {
	roomRepo     *db.RoomRepository
	msgRepo      *db.MessageRepository
	presenceRepo *db.AgentPresenceRepository
	slug         string
}

// NewCollaborationExampleHandler wires the handler to the room it showcases.
// The slug comes from HOMEPAGE_EXAMPLE_ROOM_SLUG, falling back to the default.
func NewCollaborationExampleHandler(
	roomRepo *db.RoomRepository,
	msgRepo *db.MessageRepository,
	presenceRepo *db.AgentPresenceRepository,
) *CollaborationExampleHandler {
	slug := os.Getenv("HOMEPAGE_EXAMPLE_ROOM_SLUG")
	if slug == "" {
		slug = DefaultCollabExampleRoomSlug
	}
	return &CollaborationExampleHandler{
		roomRepo:     roomRepo,
		msgRepo:      msgRepo,
		presenceRepo: presenceRepo,
		slug:         slug,
	}
}

// GetExample handles GET /v1/homepage/example (public, no auth).
//
// It always answers 200: an unavailable room degrades to the illustrative
// workflow rather than breaking the homepage.
func (h *CollaborationExampleHandler) GetExample(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	room, err := h.roomRepo.GetBySlug(ctx, h.slug)
	if err != nil {
		// Missing, deleted — or unreadable. Either way there is nothing to preview.
		slog.Info("homepage example room unavailable", "slug", h.slug, "error", err)
		roomWriteJSON(w, http.StatusOK, map[string]any{
			"data": buildCollaborationExample(nil, nil, 0, time.Now()),
		})
		return
	}

	var messages []models.Message
	var live int
	if !room.IsPrivate {
		messages, err = h.msgRepo.ListAfter(ctx, room.ID, 0, collabExampleTranscriptLimit)
		if err != nil {
			slog.Error("homepage example transcript unavailable", "slug", h.slug, "error", err)
			messages = nil
		}
		// Presence is the only thing allowed to call the transcript live.
		if presence, perr := h.presenceRepo.ListByRoom(ctx, room.ID); perr == nil {
			live = len(presence)
		} else {
			slog.Error("homepage example presence unavailable", "slug", h.slug, "error", perr)
		}
	}

	roomWriteJSON(w, http.StatusOK, map[string]any{
		"data": buildCollaborationExample(room, messages, live, time.Now()),
	})
}

// buildCollaborationExample turns a room and its transcript into the homepage
// example. It is pure: same inputs, same payload.
func buildCollaborationExample(room *models.Room, messages []models.Message, liveAgents int, now time.Time) collaborationExample {
	if room == nil || room.IsPrivate {
		return illustrativeCollaborationExample()
	}

	steps, participants, ok := collabStepsFromTranscript(room.Slug, messages)
	if !ok {
		return illustrativeCollaborationExample()
	}

	ex := collaborationExample{
		Kind:           "real",
		LiveAgentCount: liveAgents,
		Room: &collaborationExampleRoom{
			Slug:         room.Slug,
			DisplayName:  room.DisplayName,
			MessageCount: room.MessageCount,
			LastActiveAt: room.LastActiveAt,
		},
		RoomURL:      "/rooms/" + room.Slug,
		Participants: participants,
		Steps:        steps,
		ConnectURL:   collabExampleConnectURL,
		ConnectLabel: collabExampleConnectLabel,
		Headline:     room.DisplayName,
	}
	if room.Description != nil {
		ex.Room.Description = *room.Description
	}

	// Only presence reported by the API right now may call this live.
	if liveAgents > 0 {
		ex.State = "live"
		ex.Label = "LIVE COLLABORATION"
		ex.Summary = fmt.Sprintf(
			"%d agents are in this public room right now. Every message below is theirs.",
			liveAgents,
		)
	} else {
		ex.State = "completed"
		ex.Label = "COMPLETED COLLABORATION"
		ex.Summary = fmt.Sprintf(
			"A finished two-agent collaboration in a public room: %d messages, last one %s, nobody in the room now. Every message below is theirs, on the record.",
			room.MessageCount,
			collabHumanAge(now.Sub(room.LastActiveAt)),
		)
	}

	return ex
}

// illustrativeCollaborationExample is the fallback. It carries NO room, NO author
// names, NO excerpts and NO room link — only the shape of the workflow and a
// working Connect action.
func illustrativeCollaborationExample() collaborationExample {
	steps := make([]collaborationExampleStep, 0, len(collabBeats))
	for _, b := range collabBeats {
		steps = append(steps, collaborationExampleStep{
			Beat:       b.key,
			Label:      b.label,
			AuthorRole: b.role,
			Excerpt:    b.illustrative,
		})
	}
	return collaborationExample{
		Kind:         "illustrative",
		State:        "illustrative",
		Label:        "ILLUSTRATIVE WORKFLOW",
		Headline:     "How a planner and an executor work together",
		Summary:      "No public example is available right now, so this is an illustrative workflow, not a recorded conversation.",
		Participants: []collaborationExampleAgent{},
		Steps:        steps,
		ConnectURL:   collabExampleConnectURL,
		ConnectLabel: collabExampleConnectLabel,
	}
}

// collabStepsFromTranscript finds the five beats in a real transcript.
//
// Roles come from the transcript itself, not from agent names: the agent that
// opens the room is the planner, the next distinct agent to speak is the executor.
// Human and system messages are never beats.
func collabStepsFromTranscript(slug string, messages []models.Message) ([]collaborationExampleStep, []collaborationExampleAgent, bool) {
	turns := make([]models.Message, 0, len(messages))
	for _, m := range messages {
		if m.AuthorType == "agent" && strings.TrimSpace(m.Content) != "" {
			turns = append(turns, m)
		}
	}
	if len(turns) < len(collabBeats) {
		return nil, nil, false
	}

	planner := turns[0].AgentName
	executor := ""
	firstExecutorIdx := -1
	for i, m := range turns {
		if m.AgentName != planner {
			executor = m.AgentName
			firstExecutorIdx = i
			break
		}
	}
	if executor == "" {
		return nil, nil, false
	}

	// planner_directive: the planner's last word before the executor arrives.
	directiveIdx := firstExecutorIdx - 1

	// executor_plan: the executor's last message of its first turn.
	planIdx := firstExecutorIdx
	for i := firstExecutorIdx + 1; i < len(turns) && turns[i].AgentName == executor; i++ {
		planIdx = i
	}

	// planner_feedback: the planner's answer to that plan.
	feedbackIdx := nextIdxByAuthor(turns, planIdx+1, planner)
	if feedbackIdx < 0 {
		return nil, nil, false
	}

	// implementation_evidence: what the executor reports back afterwards.
	evidenceIdx := nextIdxByAuthor(turns, feedbackIdx+1, executor)
	if evidenceIdx < 0 {
		return nil, nil, false
	}

	// final_review: the planner's last word in the room.
	reviewIdx := -1
	for i := len(turns) - 1; i > evidenceIdx; i-- {
		if turns[i].AgentName == planner {
			reviewIdx = i
			break
		}
	}
	if reviewIdx < 0 {
		return nil, nil, false
	}

	picked := []int{directiveIdx, planIdx, feedbackIdx, evidenceIdx, reviewIdx}
	steps := make([]collaborationExampleStep, 0, len(collabBeats))
	for i, b := range collabBeats {
		m := turns[picked[i]]
		excerpt, truncated := collabExcerpt(m.Content)
		step := collaborationExampleStep{
			Beat:       b.key,
			Label:      b.label,
			Author:     m.AgentName,
			AuthorRole: b.role,
			Excerpt:    excerpt,
			IsExcerpt:  truncated,
			MessageURL: "/rooms/" + slug,
		}
		created := m.CreatedAt
		step.CreatedAt = &created
		if m.SequenceNum != nil {
			step.SequenceNum = *m.SequenceNum
			step.MessageURL = "/rooms/" + slug + "#message-" + strconv.Itoa(*m.SequenceNum)
		}
		if truncated {
			step.ExcerptNote = "Excerpt — the original message is " +
				strconv.Itoa(len([]rune(m.Content))) + " characters"
		}
		steps = append(steps, step)
	}

	participants := []collaborationExampleAgent{
		{Name: planner, Role: "planner"},
		{Name: executor, Role: "executor"},
	}
	return steps, participants, true
}

// nextIdxByAuthor returns the first index >= from authored by name, or -1.
func nextIdxByAuthor(turns []models.Message, from int, name string) int {
	for i := from; i < len(turns); i++ {
		if turns[i].AgentName == name {
			return i
		}
	}
	return -1
}

// collabHumanAge phrases how long ago something happened. The API formats it so
// the homepage renders a string instead of deciding what "recent" means.
func collabHumanAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "moments ago"
	case d < time.Hour:
		return pluralAgo(int(d.Minutes()), "minute")
	case d < 24*time.Hour:
		return pluralAgo(int(d.Hours()), "hour")
	default:
		return pluralAgo(int(d.Hours()/24), "day")
	}
}

func pluralAgo(n int, unit string) string {
	if n == 1 {
		return "1 " + unit + " ago"
	}
	return strconv.Itoa(n) + " " + unit + "s ago"
}

// collabExcerpt shortens a message at a word boundary and says whether it cut.
func collabExcerpt(content string) (string, bool) {
	trimmed := strings.TrimSpace(content)
	runes := []rune(trimmed)
	if len(runes) <= collabExampleExcerptMaxChars {
		return trimmed, false
	}

	cut := string(runes[:collabExampleExcerptMaxChars])
	if idx := strings.LastIndexAny(cut, " \n\t"); idx > collabExampleExcerptMaxChars/2 {
		cut = cut[:idx]
	}
	return strings.TrimRight(cut, " \n\t.,;:—-") + "…", true
}
