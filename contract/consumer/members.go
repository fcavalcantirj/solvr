package main

import (
	"context"
	"errors"
	"fmt"

	solvr "github.com/fcavalcantirj/solvr/packages/sdk-go"
)

// membersReport is what the members phase saw.
type membersReport struct {
	Slug       string            `json:"slug"`
	IsPrivate  bool              `json:"is_private"`
	Refused    errorReport       `json:"refused"`   // the executor's handshake before anyone admitted it
	Admitted   []memberReport    `json:"admitted"`  // what AddRoomMember surfaced for each later agent
	Members    []memberReport    `json:"members"`   // the participants the planner listed
	NotOwner   errorReport       `json:"not_owner"` // the last agent listing the participants
	Handshakes []handshakeReport `json:"handshakes"`
	Directive  entryReport       `json:"directive"`
	Addressed  []string          `json:"addressed"` // the directive's recipients as the SDK surfaced them
	Entries    []entryReport     `json:"entries"`   // each later agent's answer
	Timeline   []entryReport     `json:"timeline"`  // the messages the last agent read
}

type memberReport struct {
	RoomID  string `json:"room_id"`
	AgentID string `json:"agent_id"`
	Role    string `json:"role"`
	AddedBy string `json:"added_by"`
}

// members is a room that grows past a pair: the planner opens a closed room, which refuses
// an agent nobody admitted; the planner admits every later agent to that same room; each
// joins with its own room token; the planner lists the participants and addresses its
// directive to them; each answers; the last agent reads the exchange.
func (c *consumer) members(ctx context.Context, rep *report) error {
	if len(c.agents) < 3 || len(c.agentIDs) != len(c.agents) {
		return step("configure", errors.New("members needs at least three SOLVR_API_KEYS and their SOLVR_AGENT_IDS"))
	}
	created, err := c.planner.CreateRoom(ctx, solvr.CreateRoomRequest{
		DisplayName: "Review " + c.run, Description: "The participants of " + c.run + " work here.", IsPrivate: true,
	})
	if err := step("createRoom", err); err != nil {
		return err
	}
	slug := created.Data.Slug
	m := &membersReport{Slug: slug, IsPrivate: created.Data.IsPrivate}
	rep.Members = m

	if m.Refused, err = refusal(c.executor.HandshakeRoom(ctx, slug, solvr.HandshakeRoomRequest{})); err != nil {
		return step("handshakeRoom before admission", err)
	}
	for _, id := range c.agentIDs[1:] {
		added, err := c.planner.AddRoomMember(ctx, slug, solvr.AddRoomMemberRequest{AgentID: id})
		if err := step("addRoomMember", err); err != nil {
			return err
		}
		m.Admitted = append(m.Admitted, memberOf(added.Data))
	}

	var inRoom []*solvr.Client
	for _, agent := range c.agents {
		hs, err := agent.HandshakeRoom(ctx, slug, solvr.HandshakeRoomRequest{})
		if err := step("handshakeRoom", err); err != nil {
			return err
		}
		m.Handshakes = append(m.Handshakes, handshakeReport{
			AgentID: hs.Data.AgentID, RoomSlug: hs.Data.RoomSlug, TokenPrefix: tokenPrefix(hs.Data.RoomToken),
		})
		inRoom = append(inRoom, agent.WithRoomToken(hs.Data.RoomToken))
	}

	listed, err := c.planner.ListRoomMembers(ctx, slug)
	if err := step("listRoomMembers", err); err != nil {
		return err
	}
	var recipients []string
	for _, p := range listed.Data {
		m.Members = append(m.Members, memberOf(p))
		if p.Role != solvr.RoomRoleOwner {
			recipients = append(recipients, p.AgentID)
		}
	}
	directive, err := inRoom[0].CreateRoomEntry(ctx, slug, solvr.CreateRoomEntryRequest{
		Body: "Directive for " + c.run + ": each of you review one module.", AddressedMemberIDs: recipients,
		ClientEntryID: c.run + "-directive",
	})
	if err := step("createRoomEntry directive", err); err != nil {
		return err
	}
	m.Directive, m.Addressed = entryOf(directive.Data), directive.Data.AddressedMemberIDs
	for i, agent := range inRoom[1:] {
		answer, err := agent.CreateRoomEntry(ctx, slug, solvr.CreateRoomEntryRequest{
			Body: fmt.Sprintf("Participant %d of %s: module %d reviewed.", i+2, c.run, i+1), ReplyToEntryID: &directive.Data.ID,
			ClientEntryID: fmt.Sprintf("%s-answer-%d", c.run, i+2),
		})
		if err := step("createRoomEntry answer", err); err != nil {
			return err
		}
		m.Entries = append(m.Entries, entryOf(answer.Data))
	}

	last := len(c.agents) - 1
	if m.Timeline, _, err = readTimeline(ctx, inRoom[last], slug, 0); err != nil {
		return err
	}
	if m.NotOwner, err = refusal(c.agents[last].ListRoomMembers(ctx, slug)); err != nil {
		return step("listRoomMembers by a participant", err)
	}
	return nil
}

// refusal is the API error a call that must be refused surfaced.
func refusal[T any](_ T, err error) (errorReport, error) {
	var apiErr *solvr.APIError
	if !errors.As(err, &apiErr) {
		return errorReport{}, fmt.Errorf("answered %v, not an API refusal", err)
	}
	return errorReport{Status: apiErr.Status, Code: apiErr.Code}, nil
}

func memberOf(p solvr.RoomMember) memberReport {
	return memberReport{RoomID: p.RoomID, AgentID: p.AgentID, Role: p.Role, AddedBy: p.AddedBy}
}
