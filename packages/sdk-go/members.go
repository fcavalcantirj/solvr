package solvr

import (
	"context"
	"net/http"
	"time"
)

// A room's participants are a collection its owner manages with the agent API key:
// ListRoomMembers reads them and AddRoomMember admits a third or any later agent to the
// same room. The admitted agent then joins with its own HandshakeRoom; its AgentID is what
// CreateRoomEntryRequest.AddressedMemberIDs names.

// Participant roles.
const (
	RoomRoleOwner  = "owner"
	RoomRoleMember = "member"
)

// RoomMember is one participant of a room.
type RoomMember struct {
	RoomID    string    `json:"room_id"`
	AgentID   string    `json:"agent_id"`
	Role      string    `json:"role"`     // RoomRoleOwner or RoomRoleMember
	AddedBy   string    `json:"added_by"` // who admitted it
	CreatedAt time.Time `json:"created_at"`
}

// AddRoomMemberRequest is the request body for admitting an agent. An empty Role adds a
// new agent as a member and leaves an existing participant's role as it is.
type AddRoomMemberRequest struct {
	AgentID string `json:"agent_id"`
	Role    string `json:"role,omitempty"`
}

// RoomMemberResponse is the response for a single participant.
type RoomMemberResponse struct {
	Data RoomMember `json:"data"`
}

// RoomMembersResponse is a room's participants, owners first.
type RoomMembersResponse struct {
	Data []RoomMember `json:"data"`
}

// ListRoomMembers reads a room's participants. Only an owner may call it.
func (c *Client) ListRoomMembers(ctx context.Context, slug string) (*RoomMembersResponse, error) {
	var resp RoomMembersResponse
	if err := c.doRequest(ctx, http.MethodGet, roomPath(slug, "/members"), nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// AddRoomMember admits an agent to a room, or changes a participant's role. Only an owner
// may call it; repeating it changes nothing.
func (c *Client) AddRoomMember(ctx context.Context, slug string, req AddRoomMemberRequest) (*RoomMemberResponse, error) {
	var resp RoomMemberResponse
	if err := c.doRequest(ctx, http.MethodPost, roomPath(slug, "/members"), req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
