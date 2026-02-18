// Package models contains data structures for the Solvr API.
package models

import (
	"time"
)

// PinStatus represents the status of a pin request.
type PinStatus string

const (
	PinStatusQueued  PinStatus = "queued"
	PinStatusPinning PinStatus = "pinning"
	PinStatusPinned  PinStatus = "pinned"
	PinStatusFailed  PinStatus = "failed"
)

// IsValidPinStatus checks if a pin status value is valid.
func IsValidPinStatus(s PinStatus) bool {
	switch s {
	case PinStatusQueued, PinStatusPinning, PinStatusPinned, PinStatusFailed:
		return true
	default:
		return false
	}
}

// Pin represents an IPFS pin request following the Pinning Service API spec.
// See SPEC.md Part 22 and prd-v6-ipfs-expanded.json.
type Pin struct {
	// ID is the unique identifier (requestid in Pinning Service API).
	ID string `json:"requestid"`

	// CID is the IPFS content identifier being pinned.
	CID string `json:"cid"`

	// Status is the current pin status: queued, pinning, pinned, or failed.
	Status PinStatus `json:"status"`

	// Name is an optional human-readable name for the pin.
	Name string `json:"name,omitempty"`

	// Origins is an optional list of multiaddrs known to provide the content.
	Origins []string `json:"origins,omitempty"`

	// Meta is optional metadata associated with the pin.
	Meta map[string]string `json:"meta,omitempty"`

	// Delegates is a list of multiaddrs of nodes delegated to pin the content.
	Delegates []string `json:"delegates,omitempty"`

	// OwnerID is the ID of the user or agent who owns this pin.
	OwnerID string `json:"owner_id"`

	// OwnerType is the type of owner: "user" or "agent".
	OwnerType string `json:"owner_type"`

	// SizeBytes is the size of the pinned content (populated after pin completes).
	SizeBytes *int64 `json:"size_bytes,omitempty"`

	// CreatedAt is when the pin request was created.
	CreatedAt time.Time `json:"created"`

	// UpdatedAt is when the pin was last updated.
	UpdatedAt time.Time `json:"updated_at"`

	// PinnedAt is when the content was successfully pinned (null if not yet pinned).
	PinnedAt *time.Time `json:"pinned_at,omitempty"`
}

// PinListOptions contains options for listing pins.
type PinListOptions struct {
	CID    string    // Filter by CID
	Name   string    // Filter by name (exact match)
	Status PinStatus // Filter by status
	Limit  int       // Max results (default 10, max 1000)
	Offset int       // Offset for pagination
}

// PinResponse represents the Pinning Service API response format.
type PinResponse struct {
	RequestID string    `json:"requestid"`
	Status    PinStatus `json:"status"`
	Created   time.Time `json:"created"`
	Pin       PinInfo   `json:"pin"`
	Delegates []string  `json:"delegates"`
	Info      *PinExtra `json:"info,omitempty"`
}

// PinInfo is the pin object within a Pinning Service API response.
type PinInfo struct {
	CID     string            `json:"cid"`
	Name    string            `json:"name,omitempty"`
	Origins []string          `json:"origins,omitempty"`
	Meta    map[string]string `json:"meta,omitempty"`
}

// PinExtra contains extra information about a pin.
type PinExtra struct {
	SizeBytes *int64 `json:"size_bytes,omitempty"`
}

// ToPinResponse converts a Pin model to a Pinning Service API response.
func (p *Pin) ToPinResponse() PinResponse {
	resp := PinResponse{
		RequestID: p.ID,
		Status:    p.Status,
		Created:   p.CreatedAt,
		Pin: PinInfo{
			CID:     p.CID,
			Name:    p.Name,
			Origins: p.Origins,
			Meta:    p.Meta,
		},
		Delegates: p.Delegates,
	}

	if p.Delegates == nil {
		resp.Delegates = []string{}
	}

	if p.SizeBytes != nil {
		resp.Info = &PinExtra{SizeBytes: p.SizeBytes}
	}

	return resp
}
