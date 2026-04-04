package jobs

import (
	"context"
	"log"
	"time"

	"github.com/fcavalcantirj/solvr/internal/hub"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// DefaultPresenceReaperInterval is how often the reaper runs (60 seconds per D-26).
const DefaultPresenceReaperInterval = 60 * time.Second

// PresenceExpirer deletes expired agent presence records from the database.
// Implemented by db.AgentPresenceRepository.
type PresenceExpirer interface {
	DeleteExpired(ctx context.Context) ([]models.ExpiredPresence, error)
}

// RoomExpirer deletes rooms past their expires_at timestamp.
// Implemented by db.RoomRepository.
type RoomExpirer interface {
	DeleteExpiredRooms(ctx context.Context) (int64, error)
}

// PresenceReaperResult holds the results of a single reaper run.
type PresenceReaperResult struct {
	ExpiredAgents int
	ExpiredRooms  int64
}

// PresenceReaperJob handles periodic cleanup of expired agent presence
// and expired rooms. It is the 7th background job in main.go (D-26).
//
// On each cycle it:
//  1. Calls PresenceExpirer.DeleteExpired to atomically evict expired agents from DB
//  2. Removes each evicted agent from the in-memory PresenceRegistry
//  3. Calls Hub.Unsubscribe to emit presence_leave SSE events (D-27)
//  4. Calls RoomExpirer.DeleteExpiredRooms to soft-delete rooms past expires_at (D-29)
type PresenceReaperJob struct {
	presenceExpirer PresenceExpirer
	roomExpirer     RoomExpirer
	registry        *hub.PresenceRegistry
	hubMgr          *hub.HubManager
}

// NewPresenceReaperJob creates a new presence reaper job.
func NewPresenceReaperJob(
	presenceExpirer PresenceExpirer,
	roomExpirer RoomExpirer,
	registry *hub.PresenceRegistry,
	hubMgr *hub.HubManager,
) *PresenceReaperJob {
	return &PresenceReaperJob{
		presenceExpirer: presenceExpirer,
		roomExpirer:     roomExpirer,
		registry:        registry,
		hubMgr:          hubMgr,
	}
}

// RunOnce executes a single reaper cycle:
//  1. Delete expired agent presence from DB
//  2. Remove from in-memory registry + hub (emit presence_leave per D-27)
//  3. Delete expired rooms (D-29)
//
// Each step is independent -- errors in one step do not prevent others.
func (j *PresenceReaperJob) RunOnce(ctx context.Context) PresenceReaperResult {
	var result PresenceReaperResult

	// Step 1: Delete expired agent presence from DB.
	removed, err := j.presenceExpirer.DeleteExpired(ctx)
	if err != nil {
		log.Printf("Presence reaper: failed to delete expired agents: %v", err)
	} else {
		result.ExpiredAgents = len(removed)

		// Step 2 & 3: Remove from registry and hub (emit presence_leave per D-27).
		for _, row := range removed {
			roomID := hub.NewRoomID(row.RoomID)
			j.registry.Remove(roomID, row.AgentName)
			if h := j.hubMgr.Get(roomID); h != nil {
				h.Unsubscribe(row.AgentName) // emits presence_leave event
			}
		}
	}

	// Step 4: Delete expired rooms (D-29).
	expiredRooms, err := j.roomExpirer.DeleteExpiredRooms(ctx)
	if err != nil {
		log.Printf("Presence reaper: failed to delete expired rooms: %v", err)
	} else {
		result.ExpiredRooms = expiredRooms
	}

	if result.ExpiredAgents > 0 || result.ExpiredRooms > 0 {
		log.Printf("Presence reaper: evicted %d agents, deleted %d rooms",
			result.ExpiredAgents, result.ExpiredRooms)
	}

	return result
}

// RunScheduled runs the reaper job on a schedule.
// It runs on each ticker interval and stops when the context is cancelled.
func (j *PresenceReaperJob) RunScheduled(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	log.Println("Presence reaper job started (runs every 60 seconds)")

	for {
		select {
		case <-ctx.Done():
			log.Println("Presence reaper job stopped")
			return
		case <-ticker.C:
			j.RunOnce(ctx)
		}
	}
}
