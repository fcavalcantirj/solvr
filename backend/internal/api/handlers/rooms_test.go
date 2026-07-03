package handlers

import (
	"testing"

	"github.com/google/uuid"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

func TestAgentOwnsRoom_ClaimedAgentOwnsRoom(t *testing.T) {
	ownerID := uuid.New()
	agent := &models.Agent{ID: "agent_test", HumanID: strPtr(ownerID.String())}
	room := &models.Room{OwnerID: &ownerID}
	if !agentOwnsRoom(agent, room) {
		t.Error("expected claimed agent whose human owns the room to own it")
	}
}

func TestAgentOwnsRoom_ClaimedAgentDifferentOwner(t *testing.T) {
	ownerID := uuid.New()
	agent := &models.Agent{ID: "agent_test", HumanID: strPtr(uuid.New().String())}
	room := &models.Room{OwnerID: &ownerID}
	if agentOwnsRoom(agent, room) {
		t.Error("expected agent linked to a different human to not own the room")
	}
}

func TestAgentOwnsRoom_UnclaimedAgent(t *testing.T) {
	ownerID := uuid.New()
	agent := &models.Agent{ID: "agent_test", HumanID: nil}
	room := &models.Room{OwnerID: &ownerID}
	if agentOwnsRoom(agent, room) {
		t.Error("expected unclaimed agent to not own any room")
	}
}

func TestAgentOwnsRoom_OwnerlessRoom(t *testing.T) {
	agent := &models.Agent{ID: "agent_test", HumanID: strPtr(uuid.New().String())}
	room := &models.Room{OwnerID: nil}
	if agentOwnsRoom(agent, room) {
		t.Error("expected no agent to own an ownerless room")
	}
}

func TestAgentOwnsRoom_InvalidHumanID(t *testing.T) {
	ownerID := uuid.New()
	agent := &models.Agent{ID: "agent_test", HumanID: strPtr("not-a-uuid")}
	room := &models.Room{OwnerID: &ownerID}
	if agentOwnsRoom(agent, room) {
		t.Error("expected agent with unparseable human ID to not own the room")
	}
}

func TestAgentOwnsRoom_NilAgent(t *testing.T) {
	ownerID := uuid.New()
	room := &models.Room{OwnerID: &ownerID}
	if agentOwnsRoom(nil, room) {
		t.Error("expected nil agent to not own the room")
	}
}

func TestCanManageRoom_HumanOwner(t *testing.T) {
	ownerID := uuid.New()
	claims := &auth.Claims{UserID: ownerID.String(), Role: "user"}
	room := &models.Room{OwnerID: &ownerID}
	if !canManageRoom(claims, nil, room) {
		t.Error("expected human owner to manage the room")
	}
}

func TestCanManageRoom_Admin(t *testing.T) {
	ownerID := uuid.New()
	claims := &auth.Claims{UserID: uuid.New().String(), Role: "admin"}
	room := &models.Room{OwnerID: &ownerID}
	if !canManageRoom(claims, nil, room) {
		t.Error("expected admin to manage any room")
	}
}

func TestCanManageRoom_NonOwnerHuman(t *testing.T) {
	ownerID := uuid.New()
	claims := &auth.Claims{UserID: uuid.New().String(), Role: "user"}
	room := &models.Room{OwnerID: &ownerID}
	if canManageRoom(claims, nil, room) {
		t.Error("expected non-owner human to not manage the room")
	}
}

func TestCanManageRoom_AgentOwner(t *testing.T) {
	ownerID := uuid.New()
	agent := &models.Agent{ID: "agent_test", HumanID: strPtr(ownerID.String())}
	room := &models.Room{OwnerID: &ownerID}
	if !canManageRoom(nil, agent, room) {
		t.Error("expected claimed agent whose human owns the room to manage it")
	}
}

func TestCanManageRoom_NonOwnerAgent(t *testing.T) {
	ownerID := uuid.New()
	agent := &models.Agent{ID: "agent_test", HumanID: strPtr(uuid.New().String())}
	room := &models.Room{OwnerID: &ownerID}
	if canManageRoom(nil, agent, room) {
		t.Error("expected non-owner agent to not manage the room")
	}
}

func TestCanManageRoom_NilBoth(t *testing.T) {
	ownerID := uuid.New()
	room := &models.Room{OwnerID: &ownerID}
	if canManageRoom(nil, nil, room) {
		t.Error("expected unauthenticated caller to not manage the room")
	}
}

func TestCanRotateRoomToken_HumanOwner(t *testing.T) {
	ownerID := uuid.New()
	claims := &auth.Claims{UserID: ownerID.String(), Role: "user"}
	room := &models.Room{OwnerID: &ownerID}
	if !canRotateRoomToken(claims, nil, room) {
		t.Error("expected human owner to rotate the token")
	}
}

func TestCanRotateRoomToken_AgentOwner(t *testing.T) {
	ownerID := uuid.New()
	agent := &models.Agent{ID: "agent_test", HumanID: strPtr(ownerID.String())}
	room := &models.Room{OwnerID: &ownerID}
	if !canRotateRoomToken(nil, agent, room) {
		t.Error("expected claimed agent whose human owns the room to rotate the token")
	}
}

func TestCanRotateRoomToken_Admin(t *testing.T) {
	// D-25 amendment: admin may rotate — otherwise ownerless rooms have
	// tokens that nobody can rotate.
	claims := &auth.Claims{UserID: uuid.New().String(), Role: "admin"}
	room := &models.Room{OwnerID: nil}
	if !canRotateRoomToken(claims, nil, room) {
		t.Error("expected admin to rotate any room token")
	}
}

func TestCanRotateRoomToken_NonOwner(t *testing.T) {
	ownerID := uuid.New()
	claims := &auth.Claims{UserID: uuid.New().String(), Role: "user"}
	room := &models.Room{OwnerID: &ownerID}
	if canRotateRoomToken(claims, nil, room) {
		t.Error("expected non-owner to not rotate the token")
	}
}
