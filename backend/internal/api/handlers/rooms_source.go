package handlers

import (
	"context"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// scrubRoomTemplate removes credentials and private-room links from the text a source
// room hands to a new room, the same way the connect flow scrubs the seeded task.
func scrubRoomTemplate(ctx context.Context, tmpl *models.RoomTemplate, rooms connectRoomSourceLookup) {
	if tmpl.Description != nil {
		d := publicTemplateText(ctx, *tmpl.Description, rooms)
		tmpl.Description = &d
	}
}

// applyRoomTemplate copies a public source room's SELECTED task structure into a create
// request — description, category and tags, each only where the request left it out —
// and records the source. Nothing else of the source room exists in a RoomTemplate, so
// memberships, credentials, pins, events and results cannot travel (idx 88 step 2).
func applyRoomTemplate(req *createRoomRequest, tmpl *models.RoomTemplate) {
	if req.Description == nil && tmpl.Description != nil {
		d := *tmpl.Description
		req.Description = &d
	}
	if req.Category == nil && tmpl.Category != nil {
		c := *tmpl.Category
		req.Category = &c
	}
	if req.Tags == nil && len(tmpl.Tags) > 0 {
		req.Tags = append([]string(nil), tmpl.Tags...)
	}
	id := tmpl.RoomID
	req.sourceRoomID = &id
}
