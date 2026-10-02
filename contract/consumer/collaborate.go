package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	solvr "github.com/fcavalcantirj/solvr/packages/sdk-go"
)

// missingPostID names no post.
const missingPostID = "00000000-0000-4000-8000-000000000000"

// maxPages ends a pagination that never ends.
const maxPages = 50

// collaborate is the planner/executor journey: a room both agents join with their own room
// tokens, a plan and its answer, the timeline read one entry per page, the stream replayed
// and then live; a post the planner writes and both reply to, threaded; its replies read one
// per page; a vote; a read of a post that does not exist. Then, when named, the executor
// continues content that existed before the run: a post and a room.
func (c *consumer) collaborate(ctx context.Context, rep *report) error {
	if err := c.workInRoom(ctx, rep); err != nil {
		return err
	}
	if err := c.postAndReply(ctx, rep); err != nil {
		return err
	}
	return c.continueExisting(ctx, rep)
}

func (c *consumer) workInRoom(ctx context.Context, rep *report) error {
	created, err := c.planner.CreateRoom(ctx, solvr.CreateRoomRequest{
		DisplayName: "Handoff " + c.run,
		Description: "The planner and the executor of " + c.run + " work here.",
	})
	if err := step("createRoom", err); err != nil {
		return err
	}
	slug := created.Data.Slug
	room := &roomReport{Slug: slug}
	rep.Room = room

	var inRoom []*solvr.Client
	for _, agent := range []*solvr.Client{c.planner, c.executor} {
		hs, err := agent.HandshakeRoom(ctx, slug, solvr.HandshakeRoomRequest{})
		if err := step("handshakeRoom", err); err != nil {
			return err
		}
		room.Handshakes = append(room.Handshakes, handshakeReport{
			AgentID: hs.Data.AgentID, RoomSlug: hs.Data.RoomSlug, TokenPrefix: tokenPrefix(hs.Data.RoomToken),
		})
		inRoom = append(inRoom, agent.WithRoomToken(hs.Data.RoomToken))
	}
	planner, executor := inRoom[0], inRoom[1]

	plan, err := planner.CreateRoomEntry(ctx, slug, solvr.CreateRoomEntryRequest{
		Body: "Plan for " + c.run + ": drain the pool, then fail over.", ClientEntryID: c.run + "-plan",
	})
	if err := step("createRoomEntry plan", err); err != nil {
		return err
	}
	done, err := executor.CreateRoomEntry(ctx, slug, solvr.CreateRoomEntryRequest{
		Body: "Done for " + c.run + ": drained and failed over.", ReplyToEntryID: &plan.Data.ID, ClientEntryID: c.run + "-done",
	})
	if err := step("createRoomEntry done", err); err != nil {
		return err
	}
	room.Entries = []entryReport{entryOf(plan.Data), entryOf(done.Data)}

	if room.Timeline, room.Pages, err = readTimeline(ctx, executor, slug, 1); err != nil {
		return err
	}
	return c.watch(ctx, planner, executor, slug, plan.Data.ID, room)
}

// watch has the planner replay the stream after its plan, then receive an entry the
// executor sends while it watches.
func (c *consumer) watch(ctx context.Context, planner, executor *solvr.Client, slug string, after int64, room *roomReport) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	stream, err := planner.StreamRoom(ctx, slug, &solvr.StreamRoomOptions{LastEventID: strconv.FormatInt(after, 10), Type: "message"})
	if err := step("streamRoom", err); err != nil {
		return err
	}
	defer stream.Close()
	for len(room.Stream) < 2 {
		ev, err := stream.Next()
		if err := step("streamRoom next", err); err != nil {
			return err
		}
		if ev.Frame == nil || ev.Frame.Type != "message" {
			continue
		}
		msg, err := ev.Frame.Message()
		if err := step("streamRoom message", err); err != nil {
			return err
		}
		room.Stream = append(room.Stream, entryReport{ID: msg.ID, AuthorID: msg.AuthorID, Body: msg.Content, ReplyToEntryID: msg.ReplyToEntryID})
		if len(room.Stream) == 1 {
			live, err := executor.CreateRoomEntry(ctx, slug, solvr.CreateRoomEntryRequest{
				Body: "Live from " + c.run + ": the replica caught up.", ClientEntryID: c.run + "-live",
			})
			if err := step("createRoomEntry live", err); err != nil {
				return err
			}
			room.Live = entryOf(live.Data)
		}
	}
	return nil
}

// readTimeline reads a room's message timeline page by page.
func readTimeline(ctx context.Context, client *solvr.Client, slug string, limit int) ([]entryReport, int, error) {
	entries := []entryReport{}
	cursor := ""
	for pages := 1; pages <= maxPages; pages++ {
		page, err := client.ListRoomEntries(ctx, slug, &solvr.ListRoomEntriesOptions{Limit: limit, Kind: "message", Cursor: cursor})
		if err := step("listRoomEntries", err); err != nil {
			return nil, pages, err
		}
		for _, e := range page.Data {
			entries = append(entries, entryOf(e))
		}
		if !page.Meta.HasMore {
			return entries, pages, nil
		}
		cursor = page.Meta.NextCursor
	}
	return nil, maxPages, step("listRoomEntries", fmt.Errorf("the timeline of %s did not end after %d pages", slug, maxPages))
}

func (c *consumer) postAndReply(ctx context.Context, rep *report) error {
	post, err := c.planner.CreatePost(ctx, solvr.CreatePostRequest{
		Title:       "Draining connection pools before a failover in " + c.run,
		Description: "What the " + c.run + " handoff learned: pause the pool, wait for active clients to reach zero, then fail over.",
		Tags:        []string{"postgres", "failover"},
	})
	if err := step("createPost", err); err != nil {
		return err
	}
	p := postOf(post.Data)
	rep.Post = &p

	first, err := c.executor.CreateReply(ctx, p.ID, solvr.CreateReplyRequest{
		Body: "Executor of " + c.run + ": PAUSE held every client and the failover took four seconds.",
	})
	if err := step("createReply", err); err != nil {
		return err
	}
	threaded, err := c.planner.CreateReply(ctx, p.ID, solvr.CreateReplyRequest{
		Body: "Planner of " + c.run + ": confirmed, closing the handoff.", ParentReplyID: &first.Data.ID,
	})
	if err := step("createReply threaded", err); err != nil {
		return err
	}
	rep.Replies = []replyReport{replyOf(first.Data), replyOf(threaded.Data)}

	cursor := ""
	for len(rep.ReplyPages) < maxPages {
		page, err := c.anonymous.ListReplies(ctx, p.ID, &solvr.ListRepliesOptions{Limit: 1, Cursor: cursor})
		if err := step("listReplies", err); err != nil {
			return err
		}
		ids := []string{}
		for _, r := range page.Data {
			ids = append(ids, r.ID)
		}
		rep.ReplyPages = append(rep.ReplyPages, pageReport{IDs: ids, Total: page.Meta.Total, HasMore: page.Meta.HasMore, NextCursor: page.Meta.NextCursor != ""})
		if !page.Meta.HasMore {
			break
		}
		cursor = page.Meta.NextCursor
	}

	vote, err := c.executor.VoteReply(ctx, threaded.Data.ID, solvr.VoteUp)
	if err := step("voteReply", err); err != nil {
		return err
	}
	rep.Vote = &vote.Data

	_, err = c.anonymous.GetPost(ctx, missingPostID)
	var apiErr *solvr.APIError
	if !errors.As(err, &apiErr) {
		return step("getPost missing", fmt.Errorf("a post that does not exist answered %v, not an API error", err))
	}
	rep.MissingPost = &errorReport{Status: apiErr.Status, Code: apiErr.Code}
	return nil
}

// continueExisting has the executor continue SOLVR_EXISTING_POST (a reply threaded under its
// first reply) and join SOLVR_EXISTING_ROOM (read it, then send).
func (c *consumer) continueExisting(ctx context.Context, rep *report) error {
	if c.existingPost == "" && c.room == "" {
		return nil
	}
	ex := &existingReport{}
	rep.Existing = ex
	if c.existingPost != "" {
		got, err := c.executor.GetPost(ctx, c.existingPost)
		if err := step("getPost existing", err); err != nil {
			return err
		}
		ex.Post = postOf(got.Data)
		replies, err := c.executor.ListReplies(ctx, c.existingPost, nil)
		if err := step("listReplies existing", err); err != nil {
			return err
		}
		for _, r := range replies.Data {
			ex.Replies = append(ex.Replies, replyOf(r))
		}
		if len(ex.Replies) == 0 {
			return step("createReply existing", fmt.Errorf("post %s has no reply to continue", c.existingPost))
		}
		follow, err := c.executor.CreateReply(ctx, c.existingPost, solvr.CreateReplyRequest{
			Body: "Executor of " + c.run + ": still true after the upgrade.", ParentReplyID: &ex.Replies[0].ID,
		})
		if err := step("createReply existing", err); err != nil {
			return err
		}
		ex.FollowUp = replyOf(follow.Data)
	}
	if c.room != "" {
		hs, err := c.executor.HandshakeRoom(ctx, c.room, solvr.HandshakeRoomRequest{})
		if err := step("handshakeRoom existing", err); err != nil {
			return err
		}
		inRoom := c.executor.WithRoomToken(hs.Data.RoomToken)
		timeline, _, err := readTimeline(ctx, inRoom, c.room, 0)
		if err != nil {
			return err
		}
		sent, err := inRoom.CreateRoomEntry(ctx, c.room, solvr.CreateRoomEntryRequest{
			Body: "Executor of " + c.run + " joined after the upgrade.", ClientEntryID: c.run + "-joined",
		})
		if err := step("createRoomEntry existing", err); err != nil {
			return err
		}
		ex.Room = &existingRoomReport{AgentID: hs.Data.AgentID, Timeline: timeline, Sent: entryOf(sent.Data)}
	}
	return nil
}

// tokenPrefix is the kind of a credential (solvr_rt_ for a room token), never the secret.
func tokenPrefix(token string) string {
	const prefix = "solvr_rt_"
	if len(token) > len(prefix) && token[:len(prefix)] == prefix {
		return prefix
	}
	return "unexpected"
}
