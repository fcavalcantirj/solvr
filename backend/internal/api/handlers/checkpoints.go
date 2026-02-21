// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/fcavalcantirj/solvr/internal/api/response"
	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// CheckpointsHandler handles AMCP checkpoint endpoints.
// Checkpoints are stored as pins with meta.type="amcp_checkpoint".
type CheckpointsHandler struct {
	repo        PinRepositoryInterface
	ipfs        IPFSPinner
	storageRepo StorageRepositoryInterface
	agentFinder AgentFinderInterface
	agentRepo   CheckpointAgentRepo
	logger      *slog.Logger
}

// CheckpointAgentRepo defines agent operations needed by the checkpoints handler.
type CheckpointAgentRepo interface {
	UpdateLastSeen(ctx context.Context, id string) error
}

// NewCheckpointsHandler creates a new CheckpointsHandler.
func NewCheckpointsHandler(repo PinRepositoryInterface, ipfs IPFSPinner) *CheckpointsHandler {
	return &CheckpointsHandler{
		repo:   repo,
		ipfs:   ipfs,
		logger: slog.New(slog.NewJSONHandler(os.Stderr, nil)),
	}
}

// SetLogger sets a custom logger for the handler.
func (h *CheckpointsHandler) SetLogger(logger *slog.Logger) {
	h.logger = logger
}

// SetStorageRepo sets the storage repository for quota enforcement.
func (h *CheckpointsHandler) SetStorageRepo(repo StorageRepositoryInterface) {
	h.storageRepo = repo
}

// SetAgentFinderRepo sets the agent finder for family access checks.
func (h *CheckpointsHandler) SetAgentFinderRepo(repo AgentFinderInterface) {
	h.agentFinder = repo
}

// SetAgentRepo sets the agent repository for UpdateLastSeen.
func (h *CheckpointsHandler) SetAgentRepo(repo CheckpointAgentRepo) {
	h.agentRepo = repo
}

// CreateCheckpointRequest represents the request body for POST /v1/agents/me/checkpoints.
// The cid field is required. Dynamic meta fields (death_count, memory_hash, etc.)
// are passed as top-level fields and merged into pin meta alongside auto-injected
// type=amcp_checkpoint and agent_id.
type CreateCheckpointRequest struct {
	CID  string `json:"cid"`
	Name string `json:"name,omitempty"`
}

// Create handles POST /v1/agents/me/checkpoints — create a new checkpoint.
// Agent API key only. Humans get 403.
func (h *CheckpointsHandler) Create(w http.ResponseWriter, r *http.Request) {
	// Must be an agent (API key auth only)
	agent := auth.AgentFromContext(r.Context())
	if agent == nil {
		// Check if human JWT — forbidden for humans
		claims := auth.ClaimsFromContext(r.Context())
		if claims != nil {
			response.WriteForbidden(w, "only agents can create checkpoints")
			return
		}
		response.WriteUnauthorized(w, "authentication required")
		return
	}

	// Parse request body as generic map to capture dynamic meta fields
	var rawBody map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&rawBody); err != nil {
		response.WriteError(w, http.StatusBadRequest, response.ErrCodeValidation, "invalid JSON body")
		return
	}

	// Extract CID (required)
	cid, _ := rawBody["cid"].(string)
	if cid == "" {
		response.WriteError(w, http.StatusBadRequest, response.ErrCodeValidation, "cid is required")
		return
	}
	if !IsValidCID(cid) {
		response.WriteError(w, http.StatusBadRequest, response.ErrCodeValidation, "invalid CID format: must be a valid CIDv0 (Qm...) or CIDv1 (bafy...)")
		return
	}

	// Extract optional name
	name, _ := rawBody["name"].(string)

	// Check storage quota if storage repo is configured
	if h.storageRepo != nil {
		used, quota, err := h.storageRepo.GetStorageUsage(r.Context(), agent.ID, "agent")
		if err != nil {
			h.logger.Error("failed to check storage quota", "ownerID", agent.ID, "error", err.Error())
			// Fail open — allow the checkpoint if we can't check quota
		} else if used >= quota {
			response.WriteError(w, http.StatusPaymentRequired, "QUOTA_EXCEEDED", "storage quota exceeded")
			return
		}
	}

	// Build meta: start with dynamic fields, then auto-inject fixed fields
	meta := make(map[string]string)
	// Collect dynamic meta from top-level body fields (excluding known fields)
	knownFields := map[string]bool{"cid": true, "name": true, "origins": true, "meta": true}
	for k, v := range rawBody {
		if knownFields[k] {
			continue
		}
		if str, ok := v.(string); ok {
			meta[k] = str
		}
	}

	// Auto-inject fixed checkpoint meta (overrides any user-provided values)
	meta["type"] = "amcp_checkpoint"
	meta["agent_id"] = agent.ID

	// Auto-generate checkpoint name if not provided
	if name == "" {
		cidPrefix := cid
		if len(cidPrefix) > 8 {
			cidPrefix = cidPrefix[:8]
		}
		name = fmt.Sprintf("checkpoint_%s_%s", cidPrefix, time.Now().UTC().Format("20060102"))
	}

	// Build pin model
	pin := &models.Pin{
		CID:       cid,
		Status:    models.PinStatusQueued,
		Name:      name,
		Origins:   []string{},
		Meta:      meta,
		Delegates: []string{},
		OwnerID:   agent.ID,
		OwnerType: "agent",
	}

	// Create pin record in DB
	err := h.repo.Create(r.Context(), pin)
	if err != nil {
		if errors.Is(err, db.ErrDuplicatePin) {
			response.WriteError(w, http.StatusConflict, response.ErrCodeDuplicateContent, "checkpoint already exists for this CID")
			return
		}
		logCtx := response.LogContext{
			Operation: "CreateCheckpoint",
			Resource:  "checkpoint",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra: map[string]string{
				"cid":     cid,
				"agentID": agent.ID,
			},
		}
		response.WriteInternalErrorWithLog(w, "failed to create checkpoint", err, logCtx, h.logger)
		return
	}

	// Spawn async goroutine to pin content on IPFS
	go h.asyncPin(pin.ID, pin.CID)

	// Update last_seen_at for liveness tracking
	if h.agentRepo != nil {
		_ = h.agentRepo.UpdateLastSeen(r.Context(), agent.ID)
	}

	// Return 202 Accepted with pin response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(pin.ToPinResponse())
}

// asyncPin performs the actual IPFS pinning in the background.
func (h *CheckpointsHandler) asyncPin(pinID, cid string) {
	ctx := context.Background()

	// Update status to pinning
	_ = h.repo.UpdateStatus(ctx, pinID, models.PinStatusPinning)

	// Pin on IPFS
	err := h.ipfs.Pin(ctx, cid)
	if err != nil {
		h.logger.Error("async IPFS pin failed", "pinID", pinID, "cid", cid, "error", err.Error())
		_ = h.repo.UpdateStatus(ctx, pinID, models.PinStatusFailed)
		return
	}

	// Get content size from IPFS
	var sizeBytes int64
	size, statErr := h.ipfs.ObjectStat(ctx, cid)
	if statErr != nil {
		h.logger.Error("ObjectStat failed after pin", "pinID", pinID, "cid", cid, "error", statErr.Error())
	} else {
		sizeBytes = size
	}

	// Pin succeeded — update status and size
	_ = h.repo.UpdateStatusAndSize(ctx, pinID, models.PinStatusPinned, sizeBytes)

	// Increment storage usage if size is known
	if h.storageRepo != nil && sizeBytes > 0 {
		pin, getErr := h.repo.GetByID(ctx, pinID)
		if getErr != nil {
			h.logger.Error("failed to get pin for storage update", "pinID", pinID, "error", getErr.Error())
		} else {
			if updateErr := h.storageRepo.UpdateStorageUsed(ctx, pin.OwnerID, pin.OwnerType, sizeBytes); updateErr != nil {
				h.logger.Error("failed to increment storage usage", "ownerID", pin.OwnerID, "error", updateErr.Error())
			}
		}
	}
}

// ListCheckpoints handles GET /v1/agents/{id}/checkpoints — list an agent's checkpoints.
// Accessible by the agent itself, sibling agents (same human via isFamilyAccess),
// or the claiming human (JWT).
func (h *CheckpointsHandler) ListCheckpoints(w http.ResponseWriter, r *http.Request, agentID string) {
	ctx := r.Context()

	// Check agent API key auth first
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
				response.WriteForbidden(w, "agents can only access their own or sibling agents' checkpoints")
				return
			}
		}
	} else {
		// Check human JWT auth
		claims := auth.ClaimsFromContext(ctx)
		if claims == nil {
			response.WriteUnauthorized(w, "authentication required")
			return
		}

		// Look up the agent
		agent, err := h.agentFinder.FindByID(ctx, agentID)
		if err != nil {
			response.WriteNotFound(w, "agent not found")
			return
		}

		// Verify the human is the claiming owner
		if agent.HumanID == nil || *agent.HumanID != claims.UserID {
			response.WriteForbidden(w, "you must be the claiming owner of this agent")
			return
		}
	}

	// Always filter by meta type=amcp_checkpoint
	opts := models.PinListOptions{
		Meta: map[string]string{
			"type": "amcp_checkpoint",
		},
	}

	pins, total, err := h.repo.ListByOwner(ctx, agentID, "agent", opts)
	if err != nil {
		logCtx := response.LogContext{
			Operation: "ListCheckpoints",
			Resource:  "checkpoint",
			RequestID: r.Header.Get("X-Request-ID"),
		}
		response.WriteInternalErrorWithLog(w, "failed to list checkpoints", err, logCtx, h.logger)
		return
	}

	results := make([]models.PinResponse, len(pins))
	for i := range pins {
		results[i] = pins[i].ToPinResponse()
	}

	// Build response with latest field
	respBody := map[string]interface{}{
		"count":   total,
		"results": results,
		"latest":  nil,
	}

	// latest = first result (results are ORDER BY created_at DESC)
	if len(results) > 0 {
		respBody["latest"] = results[0]
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(respBody)
}
