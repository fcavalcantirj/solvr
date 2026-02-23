// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/fcavalcantirj/solvr/internal/api/response"
	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// ResurrectionCheckpointFinder finds the latest checkpoint for an agent.
type ResurrectionCheckpointFinder interface {
	FindLatestCheckpoint(ctx context.Context, agentID string) (*models.Pin, error)
}

// ResurrectionKnowledgeRepo provides access to an agent's knowledge artifacts.
type ResurrectionKnowledgeRepo interface {
	GetAgentIdeas(ctx context.Context, agentID string, limit int) ([]models.ResurrectionIdea, error)
	GetAgentApproaches(ctx context.Context, agentID string, limit int) ([]models.ResurrectionApproach, error)
	GetAgentOpenProblems(ctx context.Context, agentID string) ([]models.ResurrectionProblem, error)
}

// ResurrectionStatsRepo provides agent statistics.
type ResurrectionStatsRepo interface {
	GetAgentStats(ctx context.Context, agentID string) (*models.AgentStats, error)
}

// ResurrectionAgentUpdateRepo provides agent liveness tracking.
type ResurrectionAgentUpdateRepo interface {
	UpdateLastSeen(ctx context.Context, id string) error
}

// Knowledge limits for the resurrection bundle.
const (
	resurrectionIdeasLimit      = 50
	resurrectionApproachesLimit = 50
)

// ResurrectionHandler handles the GET /v1/agents/{id}/resurrection-bundle endpoint.
type ResurrectionHandler struct {
	agentFinder      AgentFinderInterface
	checkpointFinder ResurrectionCheckpointFinder
	knowledgeRepo    ResurrectionKnowledgeRepo
	statsRepo        ResurrectionStatsRepo
	agentRepo        ResurrectionAgentUpdateRepo
	logger           *slog.Logger
}

// NewResurrectionHandler creates a new ResurrectionHandler.
func NewResurrectionHandler(
	agentFinder AgentFinderInterface,
	checkpointFinder ResurrectionCheckpointFinder,
	knowledgeRepo ResurrectionKnowledgeRepo,
	statsRepo ResurrectionStatsRepo,
) *ResurrectionHandler {
	return &ResurrectionHandler{
		agentFinder:      agentFinder,
		checkpointFinder: checkpointFinder,
		knowledgeRepo:    knowledgeRepo,
		statsRepo:        statsRepo,
		logger:           slog.New(slog.NewJSONHandler(os.Stderr, nil)),
	}
}

// SetAgentRepo sets the agent repo for UpdateLastSeen.
func (h *ResurrectionHandler) SetAgentRepo(repo ResurrectionAgentUpdateRepo) {
	h.agentRepo = repo
}

// SetLogger sets a custom logger for the handler.
func (h *ResurrectionHandler) SetLogger(logger *slog.Logger) {
	h.logger = logger
}

// resurrectionIdentity is the identity section of the resurrection bundle.
type resurrectionIdentity struct {
	ID              string    `json:"id"`
	DisplayName     string    `json:"display_name"`
	CreatedAt       time.Time `json:"created_at"`
	Model           string    `json:"model,omitempty"`
	Specialties     []string  `json:"specialties,omitempty"`
	Bio             string    `json:"bio,omitempty"`
	HasAMCPIdentity bool      `json:"has_amcp_identity"`
	AMCPAID         string    `json:"amcp_aid,omitempty"`
	KERIPublicKey   string    `json:"keri_public_key,omitempty"`
}

// resurrectionKnowledge is the knowledge section of the resurrection bundle.
type resurrectionKnowledge struct {
	Ideas      []models.ResurrectionIdea     `json:"ideas"`
	Approaches []models.ResurrectionApproach `json:"approaches"`
	Problems   []models.ResurrectionProblem  `json:"problems"`
}

// resurrectionReputation is the reputation section of the resurrection bundle.
type resurrectionReputation struct {
	Total           int `json:"total"`
	ProblemsSolved  int `json:"problems_solved"`
	AnswersAccepted int `json:"answers_accepted"`
	IdeasPosted     int `json:"ideas_posted"`
	UpvotesReceived int `json:"upvotes_received"`
}

// resurrectionBundleResponse is the full resurrection bundle response.
type resurrectionBundleResponse struct {
	Identity         resurrectionIdentity   `json:"identity"`
	Knowledge        resurrectionKnowledge  `json:"knowledge"`
	Reputation       resurrectionReputation `json:"reputation"`
	LatestCheckpoint interface{}            `json:"latest_checkpoint"`
	DeathCount       interface{}            `json:"death_count"`
}

// GetBundle handles GET /v1/agents/{id}/resurrection-bundle.
// Accessible by: the agent itself, sibling agents (same human via isFamilyAccess),
// or the claiming human (JWT).
func (h *ResurrectionHandler) GetBundle(w http.ResponseWriter, r *http.Request, agentID string) {
	ctx := r.Context()

	// --- Access control (same pattern as checkpoints/pins) ---
	authAgent := auth.AgentFromContext(ctx)
	if authAgent != nil {
		if authAgent.ID != agentID {
			// Not self — check sibling (family) access
			targetAgent, err := h.agentFinder.FindByID(ctx, agentID)
			if err != nil {
				response.WriteNotFound(w, "agent not found")
				return
			}
			if !isFamilyAccess(authAgent, targetAgent) {
				response.WriteForbidden(w, "agents can only access their own or sibling agents' resurrection bundle")
				return
			}
		}

		// Update last_seen for requesting agent
		if h.agentRepo != nil {
			_ = h.agentRepo.UpdateLastSeen(ctx, authAgent.ID)
		}
	}
	// else: no agent API key — public access allowed (anyone can view the resurrection bundle)

	// --- Load target agent ---
	agent, err := h.agentFinder.FindByID(ctx, agentID)
	if err != nil {
		response.WriteNotFound(w, "agent not found")
		return
	}

	// --- Build response ---
	bundle := resurrectionBundleResponse{
		Identity: resurrectionIdentity{
			ID:              agent.ID,
			DisplayName:     agent.DisplayName,
			CreatedAt:       agent.CreatedAt,
			Model:           agent.Model,
			Specialties:     agent.Specialties,
			Bio:             agent.Bio,
			HasAMCPIdentity: agent.HasAMCPIdentity,
			AMCPAID:         agent.AMCPAID,
		},
		Knowledge: resurrectionKnowledge{
			Ideas:      []models.ResurrectionIdea{},
			Approaches: []models.ResurrectionApproach{},
			Problems:   []models.ResurrectionProblem{},
		},
		Reputation:       resurrectionReputation{},
		LatestCheckpoint: nil,
		DeathCount:       nil,
	}

	// --- Knowledge ---
	if h.knowledgeRepo != nil {
		ideas, err := h.knowledgeRepo.GetAgentIdeas(ctx, agentID, resurrectionIdeasLimit)
		if err != nil {
			h.logger.Warn("resurrection: ideas fetch failed", "agent_id", agentID, "error", err)
		} else if ideas != nil {
			bundle.Knowledge.Ideas = ideas
		}

		approaches, err := h.knowledgeRepo.GetAgentApproaches(ctx, agentID, resurrectionApproachesLimit)
		if err != nil {
			h.logger.Warn("resurrection: approaches fetch failed", "agent_id", agentID, "error", err)
		} else if approaches != nil {
			bundle.Knowledge.Approaches = approaches
		}

		problems, err := h.knowledgeRepo.GetAgentOpenProblems(ctx, agentID)
		if err != nil {
			h.logger.Warn("resurrection: problems fetch failed", "agent_id", agentID, "error", err)
		} else if problems != nil {
			bundle.Knowledge.Problems = problems
		}
	}

	// --- Reputation ---
	if h.statsRepo != nil {
		stats, err := h.statsRepo.GetAgentStats(ctx, agentID)
		if err != nil {
			h.logger.Warn("resurrection: stats fetch failed", "agent_id", agentID, "error", err)
		} else if stats != nil {
			bundle.Reputation = resurrectionReputation{
				Total:           stats.Reputation,
				ProblemsSolved:  stats.ProblemsSolved,
				AnswersAccepted: stats.AnswersAccepted,
				IdeasPosted:     stats.IdeasPosted,
				UpvotesReceived: stats.UpvotesReceived,
			}
		}
	}

	// --- Latest checkpoint ---
	if h.checkpointFinder != nil {
		pin, err := h.checkpointFinder.FindLatestCheckpoint(ctx, agentID)
		if err != nil {
			h.logger.Warn("resurrection: checkpoint fetch failed", "agent_id", agentID, "error", err)
		} else if pin != nil {
			pinResp := pin.ToPinResponse()
			bundle.LatestCheckpoint = pinResp

			// Parse death_count from checkpoint meta
			if dc, ok := pin.Meta["death_count"]; ok {
				if parsed, parseErr := strconv.Atoi(dc); parseErr == nil {
					bundle.DeathCount = parsed
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(bundle)
}
