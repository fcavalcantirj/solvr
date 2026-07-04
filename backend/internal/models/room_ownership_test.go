package models

import (
	"testing"

	"github.com/google/uuid"
)

func strptr(s string) *string { return &s }

func TestSameHumanAsOwner(t *testing.T) {
	ownerHuman := uuid.New()
	otherHuman := uuid.New()

	roomOwnedBy := func(id *uuid.UUID) *Room { return &Room{OwnerID: id} }
	agentWithHuman := func(h *string) *Agent { return &Agent{ID: "agent_x", HumanID: h} }

	tests := []struct {
		name  string
		agent *Agent
		room  *Room
		want  bool
	}{
		{"match: agent human == room owner", agentWithHuman(strptr(ownerHuman.String())), roomOwnedBy(&ownerHuman), true},
		{"mismatch: different human", agentWithHuman(strptr(otherHuman.String())), roomOwnedBy(&ownerHuman), false},
		{"unclaimed agent (nil HumanID)", agentWithHuman(nil), roomOwnedBy(&ownerHuman), false},
		{"ownerless room (nil OwnerID)", agentWithHuman(strptr(ownerHuman.String())), roomOwnedBy(nil), false},
		{"nil agent", nil, roomOwnedBy(&ownerHuman), false},
		{"nil room", agentWithHuman(strptr(ownerHuman.String())), nil, false},
		{"unparseable HumanID", agentWithHuman(strptr("not-a-uuid")), roomOwnedBy(&ownerHuman), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := SameHumanAsOwner(tc.agent, tc.room); got != tc.want {
				t.Errorf("SameHumanAsOwner() = %v, want %v", got, tc.want)
			}
		})
	}
}
