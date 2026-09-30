package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
)

// IPFSUnpinner removes a pin from the IPFS node (services.KuboIPFSService).
type IPFSUnpinner interface {
	Unpin(ctx context.Context, cid string) error
}

// IPFSGarbageCollector runs the node's repo/gc (services.KuboIPFSService).
type IPFSGarbageCollector interface {
	RepoGC(ctx context.Context) (int, error)
}

// CIDReferenceChecker counts the rows that still name a CID (db.IPFSReferenceRepository).
type CIDReferenceChecker interface {
	CIDReferences(ctx context.Context, cid string) (pins, posts int, err error)
}

// AdminIPFSHandler serves the operator IPFS endpoints (anti-abuse W5): unpinning content that
// no longer belongs to Solvr, and garbage collection, without SSH to the IPFS server.
type AdminIPFSHandler struct {
	unpinner IPFSUnpinner
	gc       IPFSGarbageCollector
	refs     CIDReferenceChecker
}

// NewAdminIPFSHandler creates an AdminIPFSHandler.
func NewAdminIPFSHandler(unpinner IPFSUnpinner, gc IPFSGarbageCollector, refs CIDReferenceChecker) *AdminIPFSHandler {
	return &AdminIPFSHandler{unpinner: unpinner, gc: gc, refs: refs}
}

// maxUnpinCIDs caps one request; the 2026-09-29 purge list has 87.
const maxUnpinCIDs = 500

// validCID accepts CIDv0 (base58btc "Qm…") and CIDv1 in base32 ("b…"), and nothing that could
// carry query syntax.
var validCID = regexp.MustCompile(`^(Qm[1-9A-HJ-NP-Za-km-z]{44}|b[a-z2-7]{50,100})$`)

// UnpinResult is one CID's outcome.
type UnpinResult struct {
	CID    string `json:"cid"`
	Status string `json:"status"` // invalid | in_use_pin | in_use_post | would_unpin | unpinned | not_pinned | error
	Detail string `json:"detail,omitempty"`
}

// Unpin handles POST /admin/ipfs/unpin {"cids": [...], "dry_run": bool}. It refuses any CID a
// pins row or a post's crystallization_cid still names, and reports per CID.
func (h *AdminIPFSHandler) Unpin(w http.ResponseWriter, r *http.Request) {
	if !(&AdminHandler{}).checkAdminAuth(w, r) {
		return
	}
	var body struct {
		CIDs   []string `json:"cids"`
		DryRun bool     `json:"dry_run"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAdminError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}
	if len(body.CIDs) == 0 || len(body.CIDs) > maxUnpinCIDs {
		writeAdminError(w, http.StatusBadRequest, "VALIDATION_ERROR", "cids must hold 1 to 500 CIDs")
		return
	}
	results := make([]UnpinResult, 0, len(body.CIDs))
	summary := map[string]int{}
	for _, cid := range body.CIDs {
		res := h.unpinOne(r.Context(), strings.TrimSpace(cid), body.DryRun)
		summary[res.Status]++
		results = append(results, res)
	}
	if !body.DryRun {
		slog.Warn("ipfs unpin run", "cids", len(body.CIDs), "summary", summary)
	}
	writeAdminJSON(w, http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{"results": results, "summary": summary}, "dry_run": body.DryRun,
	})
}

func (h *AdminIPFSHandler) unpinOne(ctx context.Context, cid string, dryRun bool) UnpinResult {
	res := UnpinResult{CID: cid}
	if !validCID.MatchString(cid) {
		res.Status = "invalid"
		return res
	}
	pins, posts, err := h.refs.CIDReferences(ctx, cid)
	switch {
	case err != nil:
		res.Status, res.Detail = "error", "reference check failed"
		return res
	case pins > 0:
		res.Status = "in_use_pin"
		return res
	case posts > 0:
		res.Status = "in_use_post"
		return res
	case dryRun:
		res.Status = "would_unpin"
		return res
	}
	if err := h.unpinner.Unpin(ctx, cid); err != nil {
		if strings.Contains(err.Error(), "not pinned") {
			res.Status = "not_pinned"
			return res
		}
		res.Status, res.Detail = "error", err.Error()
		return res
	}
	res.Status = "unpinned"
	return res
}

// GC handles POST /admin/ipfs/gc: the node's repo/gc, run only as its own explicit step.
func (h *AdminIPFSHandler) GC(w http.ResponseWriter, r *http.Request) {
	if !(&AdminHandler{}).checkAdminAuth(w, r) {
		return
	}
	removed, err := h.gc.RepoGC(r.Context())
	if err != nil {
		slog.Error("ipfs repo/gc failed", "error", err)
		writeAdminError(w, http.StatusBadGateway, "IPFS_ERROR", "repo/gc failed: "+err.Error())
		return
	}
	slog.Warn("ipfs repo/gc run", "removed", removed)
	writeAdminJSON(w, http.StatusOK, map[string]interface{}{"data": map[string]int{"removed": removed}})
}
