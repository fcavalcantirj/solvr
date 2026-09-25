package handlers

import (
	"context"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/hub"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// TimelineFrameSource is the hub relay's delivery record: the committed room timeline,
// rendered as the same stream frames the reconnect replay sends, so an entry delivered
// live (by any instance) is identical to the one replayed after a reconnect.
type TimelineFrameSource struct {
	repo *db.RoomEntryRepository
}

// NewTimelineFrameSource reads frames from the room_entries timeline.
func NewTimelineFrameSource(repo *db.RoomEntryRepository) *TimelineFrameSource {
	return &TimelineFrameSource{repo: repo}
}

// MaxSequence is the room's latest timeline sequence.
func (s *TimelineFrameSource) MaxSequence(ctx context.Context, roomID uuid.UUID) (int, error) {
	return s.repo.MaxSequence(ctx, roomID)
}

// FramesAfter returns the frames of up to limit entries after sequence after.
func (s *TimelineFrameSource) FramesAfter(ctx context.Context, roomID uuid.UUID, after, limit int) ([]hub.RoomEvent, error) {
	entries, err := s.repo.ListPage(ctx, models.RoomEntryPageParams{RoomID: roomID, AfterSequence: after, Limit: limit})
	if err != nil {
		return nil, err
	}
	frames := make([]hub.RoomEvent, 0, len(entries))
	for i := range entries {
		frames = append(frames, timelineHubEvent(&entries[i]))
	}
	return frames, nil
}
