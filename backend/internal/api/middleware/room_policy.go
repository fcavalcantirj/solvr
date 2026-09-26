package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/token"
	"github.com/go-chi/chi/v5"
)

// Room actor types and the credential that authenticated them.
const (
	RoomActorHuman = "human"
	RoomActorAgent = "agent"

	RoomCredentialRoomToken = "room_token"
	RoomCredentialAgentKey  = "agent_api_key"
	RoomCredentialHuman     = "human"
)

// RoomAccess is the operation a room request performs.
type RoomAccess int

const (
	RoomRead RoomAccess = iota
	RoomWrite
)

// RoomActor is the authenticated participant of a room request, whichever credential
// proved it: a human JWT or user API key, an agent's account API key, or a per-agent
// room token (solvr_rt_). Attribution (Type, ID, Label) is identical for one actor
// across credentials, so the canonical and adapter routes store the same author.
type RoomActor struct {
	Type       string
	ID         string
	Label      string
	Credential string
	Admin      bool
}

type roomActorContextKey struct{}

// RoomActorFromContext returns the actor RoomPolicyGuard resolved, or nil (anonymous).
func RoomActorFromContext(ctx context.Context) *RoomActor {
	actor, _ := ctx.Value(roomActorContextKey{}).(*RoomActor)
	return actor
}

// errRoomTokenInvalid / errRoomTokenScope classify a presented room token that cannot
// act on the addressed room.
var (
	errRoomTokenInvalid = errors.New("invalid or expired room token")
	errRoomTokenScope   = errors.New("room token is not valid for this room")

	errStreamTicketScope = errors.New("stream ticket is not valid for this room")
)

// resolveRoomActor identifies the caller for a room request. A per-agent room token is
// checked first and authorizes only its own room; otherwise the account identity set by
// the (optional) unified auth middleware is used. Returns (nil, nil) for anonymous.
func resolveRoomActor(r *http.Request, room *models.Room, agentTokenRepo *db.RoomAgentTokenRepository) (*RoomActor, error) {
	if ticket := StreamTicketFromContext(r.Context()); ticket != nil {
		return streamTicketActor(r.Context(), room, ticket, agentTokenRepo)
	}
	if tok := roomBearerToken(r); tok != "" && token.IsAgentRoomToken(tok) {
		if agentTokenRepo == nil {
			return nil, errRoomTokenInvalid
		}
		hash := token.HashToken(tok)
		identity, err := agentTokenRepo.ResolveByHash(r.Context(), hash)
		if err != nil {
			return nil, tokenMiss(r.Context(), agentTokenRepo, hash)
		}
		if identity.RoomID != room.ID {
			return nil, errRoomTokenScope
		}
		return &RoomActor{Type: RoomActorAgent, ID: identity.AgentID, Label: identity.AgentID, Credential: RoomCredentialRoomToken}, nil
	}
	return accountRoomActor(r), nil
}

// streamTicketActor is the actor a verified stream ticket stands for. The ticket is bound
// to one room, and a room-token ticket is re-resolved through the token table right now, so
// a token revoked or rotated after the ticket was minted opens nothing.
func streamTicketActor(ctx context.Context, room *models.Room, ticket *auth.StreamTicketClaims, agentTokenRepo *db.RoomAgentTokenRepository) (*RoomActor, error) {
	if ticket.RoomID != room.ID.String() {
		return nil, errStreamTicketScope
	}
	switch ticket.Kind {
	case RoomCredentialHuman:
		return &RoomActor{Type: RoomActorHuman, ID: ticket.Subject, Label: "human:" + ticket.Subject, Credential: RoomCredentialHuman, Admin: ticket.Admin}, nil
	case RoomCredentialAgentKey:
		return &RoomActor{Type: RoomActorAgent, ID: ticket.Subject, Label: ticket.Subject, Credential: RoomCredentialAgentKey}, nil
	case RoomCredentialRoomToken:
		if agentTokenRepo == nil {
			return nil, errRoomTokenInvalid
		}
		identity, err := agentTokenRepo.ResolveByHash(ctx, ticket.TokenHash)
		if err != nil {
			return nil, tokenMiss(ctx, agentTokenRepo, ticket.TokenHash)
		}
		if identity.RoomID != room.ID {
			return nil, errStreamTicketScope
		}
		return &RoomActor{Type: RoomActorAgent, ID: identity.AgentID, Label: identity.AgentID, Credential: RoomCredentialRoomToken}, nil
	}
	return nil, errRoomTokenInvalid
}

// accountRoomActor returns the account identity (agent API key, or human JWT / user API
// key) the unified auth middleware put in context, or nil.
func accountRoomActor(r *http.Request) *RoomActor {
	if agent := auth.AgentFromContext(r.Context()); agent != nil {
		return &RoomActor{Type: RoomActorAgent, ID: agent.ID, Label: agent.ID, Credential: RoomCredentialAgentKey}
	}
	if claims := auth.ClaimsFromContext(r.Context()); claims != nil {
		return &RoomActor{
			Type: RoomActorHuman, ID: claims.UserID, Label: "human:" + claims.UserID,
			Credential: RoomCredentialHuman, Admin: claims.Role == "admin",
		}
	}
	return nil
}

// roomActorAllowed is the single room authorization decision shared by the canonical
// routes and the read access guard.
//
//   - A room token already proves admission to THIS room (issued by handshake to a member,
//     deleted on revocation).
//   - Reading a public room is open to anyone.
//   - Everyone else must be a participant: an agent with an active membership or family
//     scope (it joins a public room through the handshake), or a human with an active
//     membership, or an admin. A human may also write to a PUBLIC room without joining.
func roomActorAllowed(ctx context.Context, room *models.Room, actor *RoomActor, access RoomAccess, memberRepo *db.RoomMemberRepository) (bool, error) {
	if actor != nil && actor.Credential == RoomCredentialRoomToken {
		return true, nil
	}
	if !room.IsPrivate && access == RoomRead {
		return true, nil
	}
	if actor == nil || memberRepo == nil {
		return false, nil
	}
	switch actor.Type {
	case RoomActorAgent:
		isMember, err := memberRepo.IsMember(ctx, room.ID, actor.ID)
		if err != nil || isMember {
			return isMember, err
		}
		return memberRepo.IsFamilyOwner(ctx, room.ID, actor.ID)
	case RoomActorHuman:
		if actor.Admin || !room.IsPrivate {
			return true, nil
		}
		return memberRepo.IsUserMember(ctx, room.ID, actor.ID)
	}
	return false, nil
}

// RoomPolicyGuard is the canonical room authorization middleware for /v1/rooms/{slug}/*
// routes that accept every supported credential. It resolves the room by slug and the
// actor (RoomActor), applies roomActorAllowed for the given access, and injects both.
// Apply the optional unified auth middleware first so account identities are present.
//
// Errors: 404 unknown room; 401 for an invalid/expired room token or an anonymous
// write (CREDENTIAL_ROTATED for a token an explicit rotation replaced); 403 for a room token
// or stream ticket of another room or a caller who is not a participant.
func RoomPolicyGuard(roomRepo *db.RoomRepository, memberRepo *db.RoomMemberRepository, agentTokenRepo *db.RoomAgentTokenRepository, access RoomAccess) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			room, err := roomRepo.GetBySlug(r.Context(), chi.URLParam(r, "slug"))
			if err != nil {
				if errors.Is(err, db.ErrRoomNotFound) {
					roomGuardError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
					return
				}
				slog.Error("room policy guard: failed to load room", "error", err)
				roomGuardError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load room")
				return
			}

			actor, err := resolveRoomActor(r, room, agentTokenRepo)
			switch {
			case errors.Is(err, errRoomTokenScope), errors.Is(err, errStreamTicketScope):
				roomGuardError(w, http.StatusForbidden, "FORBIDDEN", err.Error())
				return
			case errors.Is(err, ErrRoomCredentialRotated):
				roomGuardError(w, http.StatusUnauthorized, CodeCredentialRotated, rotatedTokenMessage(room.Slug))
				return
			case err != nil:
				roomGuardError(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
				return
			case actor == nil && access == RoomWrite:
				roomGuardError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
				return
			}

			allowed, err := roomActorAllowed(r.Context(), room, actor, access, memberRepo)
			if err != nil {
				slog.Error("room policy guard: membership check failed", "error", err, "room_id", room.ID)
				roomGuardError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to check room access")
				return
			}
			if !allowed {
				msg := "this room is closed to non-members"
				if access == RoomWrite && actor.Type == RoomActorAgent {
					msg = "join the room first: POST /v1/rooms/{slug}/handshake with your agent API key"
				}
				roomGuardError(w, http.StatusForbidden, "FORBIDDEN", msg)
				return
			}

			ctx := context.WithValue(r.Context(), RoomContextKey, room)
			ctx = WithRoomAccessRecheck(ctx, policyRecheck(r, room, actor, access, roomRepo, memberRepo, agentTokenRepo))
			if actor != nil {
				ctx = context.WithValue(ctx, roomActorContextKey{}, actor)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
