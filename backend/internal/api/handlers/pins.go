// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/api/response"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// PinRepositoryInterface defines database operations for pins.
type PinRepositoryInterface interface {
	Create(ctx context.Context, pin *models.Pin) error
	GetByID(ctx context.Context, id string) (*models.Pin, error)
	GetByCID(ctx context.Context, cid, ownerID string) (*models.Pin, error)
	ListByOwner(ctx context.Context, ownerID, ownerType string, opts models.PinListOptions) ([]models.Pin, int, error)
	UpdateStatus(ctx context.Context, id string, status models.PinStatus) error
	Delete(ctx context.Context, id string) error
}

// IPFSPinner defines the IPFS operations needed by the pins handler.
// Defined locally to avoid import cycle with services package.
type IPFSPinner interface {
	Pin(ctx context.Context, cid string) error
	Unpin(ctx context.Context, cid string) error
}

// PinsHandler handles IPFS pinning API HTTP requests.
// Follows the IPFS Pinning Service API spec for interoperability.
type PinsHandler struct {
	repo        PinRepositoryInterface
	ipfs        IPFSPinner
	storageRepo StorageRepositoryInterface
	logger      *slog.Logger
}

// NewPinsHandler creates a new PinsHandler.
func NewPinsHandler(repo PinRepositoryInterface, ipfs IPFSPinner) *PinsHandler {
	return &PinsHandler{
		repo:   repo,
		ipfs:   ipfs,
		logger: slog.New(slog.NewJSONHandler(os.Stderr, nil)),
	}
}

// SetLogger sets a custom logger for the handler.
func (h *PinsHandler) SetLogger(logger *slog.Logger) {
	h.logger = logger
}

// SetStorageRepo sets the storage repository for quota enforcement.
// When set, pin creation checks quota before allowing new pins.
func (h *PinsHandler) SetStorageRepo(repo StorageRepositoryInterface) {
	h.storageRepo = repo
}

// CreatePinRequest represents the request body for POST /v1/pins.
type CreatePinRequest struct {
	CID     string            `json:"cid"`
	Name    string            `json:"name,omitempty"`
	Origins []string          `json:"origins,omitempty"`
	Meta    map[string]string `json:"meta,omitempty"`
}

// Create handles POST /v1/pins — create a new pin request.
// Response follows the IPFS Pinning Service API spec.
func (h *PinsHandler) Create(w http.ResponseWriter, r *http.Request) {
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		response.WriteUnauthorized(w, "authentication required")
		return
	}

	var req CreatePinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, response.ErrCodeValidation, "invalid JSON body")
		return
	}

	// Validate CID
	if req.CID == "" {
		response.WriteError(w, http.StatusBadRequest, response.ErrCodeValidation, "cid is required")
		return
	}
	if !IsValidCID(req.CID) {
		response.WriteError(w, http.StatusBadRequest, response.ErrCodeValidation, "invalid CID format: must be a valid CIDv0 (Qm...) or CIDv1 (bafy...)")
		return
	}

	// Check storage quota if storage repo is configured
	if h.storageRepo != nil {
		used, quota, err := h.storageRepo.GetStorageUsage(r.Context(), authInfo.AuthorID, string(authInfo.AuthorType))
		if err != nil {
			h.logger.Error("failed to check storage quota", "ownerID", authInfo.AuthorID, "error", err.Error())
			// Fail open — allow the pin if we can't check quota
		} else if used >= quota {
			response.WriteError(w, http.StatusPaymentRequired, "QUOTA_EXCEEDED", "storage quota exceeded")
			return
		}
	}

	// Build pin model
	pin := &models.Pin{
		CID:       req.CID,
		Status:    models.PinStatusQueued,
		Name:      req.Name,
		Origins:   req.Origins,
		Meta:      req.Meta,
		Delegates: []string{},
		OwnerID:   authInfo.AuthorID,
		OwnerType: string(authInfo.AuthorType),
	}

	// Create pin record in DB
	err := h.repo.Create(r.Context(), pin)
	if err != nil {
		if errors.Is(err, db.ErrDuplicatePin) {
			response.WriteError(w, http.StatusConflict, response.ErrCodeDuplicateContent, "pin already exists for this CID and owner")
			return
		}
		ctx := response.LogContext{
			Operation: "Create",
			Resource:  "pin",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra: map[string]string{
				"cid":       req.CID,
				"ownerType": string(authInfo.AuthorType),
				"ownerID":   authInfo.AuthorID,
			},
		}
		response.WriteInternalErrorWithLog(w, "failed to create pin", err, ctx, h.logger)
		return
	}

	// Spawn async goroutine to pin content on IPFS
	go h.asyncPin(pin.ID, pin.CID)

	// Return 202 Accepted with pin response in Pinning Service API format.
	// Uses raw encoding (no data envelope) for IPFS Pinning Service API compliance.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(pin.ToPinResponse())
}

// asyncPin performs the actual IPFS pinning in the background.
func (h *PinsHandler) asyncPin(pinID, cid string) {
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

	// Pin succeeded
	_ = h.repo.UpdateStatus(ctx, pinID, models.PinStatusPinned)
}

// GetByRequestID handles GET /v1/pins/:requestid — check pin status.
func (h *PinsHandler) GetByRequestID(w http.ResponseWriter, r *http.Request) {
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		response.WriteUnauthorized(w, "authentication required")
		return
	}

	requestID := chi.URLParam(r, "requestid")

	pin, err := h.repo.GetByID(r.Context(), requestID)
	if err != nil {
		if errors.Is(err, db.ErrPinNotFound) {
			response.WriteNotFound(w, "pin not found")
			return
		}
		ctx := response.LogContext{
			Operation: "GetByRequestID",
			Resource:  "pin",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra:     map[string]string{"requestid": requestID},
		}
		response.WriteInternalErrorWithLog(w, "failed to get pin", err, ctx, h.logger)
		return
	}

	// Verify ownership
	if pin.OwnerID != authInfo.AuthorID {
		response.WriteForbidden(w, "you can only view your own pins")
		return
	}

	// Raw encoding (no data envelope) for IPFS Pinning Service API compliance.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(pin.ToPinResponse())
}

// List handles GET /v1/pins — list user's pins.
func (h *PinsHandler) List(w http.ResponseWriter, r *http.Request) {
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		response.WriteUnauthorized(w, "authentication required")
		return
	}

	// Parse query params
	opts := models.PinListOptions{
		CID:    r.URL.Query().Get("cid"),
		Name:   r.URL.Query().Get("name"),
		Status: models.PinStatus(r.URL.Query().Get("status")),
	}

	// Parse limit (default 10, max 1000)
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 1 {
			response.WriteError(w, http.StatusBadRequest, response.ErrCodeValidation, "limit must be a positive integer")
			return
		}
		if limit > 1000 {
			limit = 1000
		}
		opts.Limit = limit
	}

	// Validate status filter if provided
	if opts.Status != "" && !models.IsValidPinStatus(opts.Status) {
		response.WriteError(w, http.StatusBadRequest, response.ErrCodeValidation, "status must be one of: queued, pinning, pinned, failed")
		return
	}

	pins, total, err := h.repo.ListByOwner(r.Context(), authInfo.AuthorID, string(authInfo.AuthorType), opts)
	if err != nil {
		ctx := response.LogContext{
			Operation: "List",
			Resource:  "pin",
			RequestID: r.Header.Get("X-Request-ID"),
		}
		response.WriteInternalErrorWithLog(w, "failed to list pins", err, ctx, h.logger)
		return
	}

	// Convert to Pinning Service API response format
	results := make([]models.PinResponse, len(pins))
	for i := range pins {
		results[i] = pins[i].ToPinResponse()
	}

	// Pinning Service API uses count/results format
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"count":   total,
		"results": results,
	})
}

// Delete handles DELETE /v1/pins/:requestid — unpin content.
func (h *PinsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		response.WriteUnauthorized(w, "authentication required")
		return
	}

	requestID := chi.URLParam(r, "requestid")

	// Verify pin exists and ownership
	pin, err := h.repo.GetByID(r.Context(), requestID)
	if err != nil {
		if errors.Is(err, db.ErrPinNotFound) {
			response.WriteNotFound(w, "pin not found")
			return
		}
		ctx := response.LogContext{
			Operation: "Delete-lookup",
			Resource:  "pin",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra:     map[string]string{"requestid": requestID},
		}
		response.WriteInternalErrorWithLog(w, "failed to get pin", err, ctx, h.logger)
		return
	}

	if pin.OwnerID != authInfo.AuthorID {
		response.WriteForbidden(w, "you can only delete your own pins")
		return
	}

	// Delete pin record
	err = h.repo.Delete(r.Context(), requestID)
	if err != nil {
		ctx := response.LogContext{
			Operation: "Delete",
			Resource:  "pin",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra:     map[string]string{"requestid": requestID},
		}
		response.WriteInternalErrorWithLog(w, "failed to delete pin", err, ctx, h.logger)
		return
	}

	// Decrement storage usage if pin had a known size
	if h.storageRepo != nil && pin.SizeBytes != nil && *pin.SizeBytes > 0 {
		if updateErr := h.storageRepo.UpdateStorageUsed(r.Context(), authInfo.AuthorID, string(authInfo.AuthorType), -*pin.SizeBytes); updateErr != nil {
			h.logger.Error("failed to decrement storage usage", "ownerID", authInfo.AuthorID, "error", updateErr.Error())
		}
	}

	// Async unpin from IPFS
	go func() {
		if unpinErr := h.ipfs.Unpin(context.Background(), pin.CID); unpinErr != nil {
			h.logger.Error("async IPFS unpin failed", "cid", pin.CID, "error", unpinErr.Error())
		}
	}()

	w.WriteHeader(http.StatusAccepted)
}

// IsValidCID validates an IPFS CID format.
// Accepts CIDv0 (Qm..., 46 chars base58) and CIDv1 (bafy.../bafk... base32).
func IsValidCID(cid string) bool {
	if len(cid) < 10 {
		return false
	}

	// CIDv0: starts with "Qm" and is base58btc encoded (46 chars typical)
	if strings.HasPrefix(cid, "Qm") && len(cid) >= 44 {
		return isBase58(cid)
	}

	// CIDv1: starts with "baf" and uses base32lower (a-z,2-7) or base36lower (a-z,0-9)
	if strings.HasPrefix(cid, "baf") && len(cid) >= 50 {
		return isAlnumLower(cid)
	}

	return false
}

// isBase58 checks if a string contains only base58 characters.
func isBase58(s string) bool {
	for _, c := range s {
		if !((c >= '1' && c <= '9') || (c >= 'A' && c <= 'H') || (c >= 'J' && c <= 'N') ||
			(c >= 'P' && c <= 'Z') || (c >= 'a' && c <= 'k') || (c >= 'm' && c <= 'z')) {
			return false
		}
	}
	return true
}

// isAlnumLower checks if a string contains only lowercase alphanumeric characters.
// CIDv1 can use base32lower (a-z,2-7) or base36lower (a-z,0-9).
func isAlnumLower(s string) bool {
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}
