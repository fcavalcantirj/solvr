package handlers

import (
	"context"

	"github.com/google/uuid"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// Share attribution on browser funnel steps (idx 88 step 4). A step may name the public
// room (by slug) or post (by id) it came from. Only a public record resolves; anything
// else is dropped, so the row holds the record's id or nothing — never client text, and
// never a secret (a share link carries a public slug or post id only).

// funnelSourceRefMax bounds a reported source identifier.
const funnelSourceRefMax = 200

type funnelSourceRef struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref"`
}

type funnelRoomSource interface {
	PublicRoomID(ctx context.Context, slug string) (uuid.UUID, error)
}

// SetSourceResolvers enables attributing browser steps to a public room or post.
func (h *FunnelHandler) SetSourceResolvers(rooms funnelRoomSource, posts connectPostLookup) {
	h.sourceRooms = rooms
	h.sourcePosts = posts
}

// resolveSource turns a reported source into a stored one, or the zero (unattributed)
// source when it does not name a public record.
func (h *FunnelHandler) resolveSource(ctx context.Context, ref *funnelSourceRef) db.FunnelSource {
	if ref == nil || ref.Ref == "" {
		return db.FunnelSource{}
	}
	switch ref.Kind {
	case models.FunnelSourceKindRoom:
		if h.sourceRooms == nil {
			return db.FunnelSource{}
		}
		if id, err := h.sourceRooms.PublicRoomID(ctx, ref.Ref); err == nil {
			return db.FunnelSource{Kind: models.FunnelSourceKindRoom, ID: id}
		}
	case models.FunnelSourceKindPost:
		if h.sourcePosts == nil {
			return db.FunnelSource{}
		}
		if postID, _, err := h.sourcePosts.FindPublicPostRef(ctx, ref.Ref); err == nil {
			if id, perr := uuid.Parse(postID); perr == nil {
				return db.FunnelSource{Kind: models.FunnelSourceKindPost, ID: id}
			}
		}
	}
	return db.FunnelSource{}
}
