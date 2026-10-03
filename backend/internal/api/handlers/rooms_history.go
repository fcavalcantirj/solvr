package handlers

import (
	"context"
	"log/slog"
	"net/http"

	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// HistoryPageSize is how many sequence numbers one transcript page spans (task idx 81,
// SPEC.md Part 27.2). Page N holds sequences (N-1)*HistoryPageSize+1 through
// N*HistoryPageSize; room sequences only grow, so a page never moves.
const HistoryPageSize = 100

// canonicalPageNumber parses a page number written the one canonical way: a positive
// decimal integer without sign or leading zeros (so /history/01 is not /history/1).
func canonicalPageNumber(raw string) (int, bool) {
	if raw == "" || len(raw) > 9 || raw[0] < '1' || raw[0] > '9' {
		return 0, false
	}
	n := 0
	for _, c := range raw {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

// historyTotalPages is how many transcript pages a room whose latest sequence is
// maxSequence spans (0 for a room without entries).
func historyTotalPages(maxSequence int) int {
	return (maxSequence + HistoryPageSize - 1) / HistoryPageSize
}

// RoomHistoryInfo is GET /v1/rooms/{slug}'s data.history: the archive's size.
type RoomHistoryInfo struct {
	PageSize   int `json:"page_size"`
	TotalPages int `json:"total_pages"`
}

// RoomHistoryPage is GET /v1/rooms/{slug}/history/{page}'s data.
type RoomHistoryPage struct {
	Page         int              `json:"page"`
	PageSize     int              `json:"page_size"`
	FromSequence int              `json:"from_sequence"`
	ToSequence   int              `json:"to_sequence"`
	TotalPages   int              `json:"total_pages"`
	PrevPage     *int             `json:"prev_page"`
	NextPage     *int             `json:"next_page"`
	Messages     []models.Message `json:"messages"`
}

// roomSequenceReader reads a room's latest timeline sequence (deleted entries included).
type roomSequenceReader interface {
	MaxSequence(ctx context.Context, roomID uuid.UUID) (int, error)
}

// roomRangeReader reads the live messages of one sequence range.
type roomRangeReader interface {
	ListSequenceRange(ctx context.Context, roomID uuid.UUID, from, to int) ([]models.Message, error)
}

// RoomHistoryHandler serves the crawlable transcript archive.
type RoomHistoryHandler struct {
	sequences roomSequenceReader
	messages  roomRangeReader
}

// NewRoomHistoryHandler creates a RoomHistoryHandler.
func NewRoomHistoryHandler(sequences roomSequenceReader, messages roomRangeReader) *RoomHistoryHandler {
	return &RoomHistoryHandler{sequences: sequences, messages: messages}
}

// GetPage handles GET /v1/rooms/{slug}/history/{page}. It is mounted behind the room
// read policy, which resolves the room (404 when gone, 403 when closed to the caller).
func (h *RoomHistoryHandler) GetPage(w http.ResponseWriter, r *http.Request) {
	room := apimiddleware.RoomFromContext(r.Context())
	if room == nil {
		roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
		return
	}
	page, ok := canonicalPageNumber(chi.URLParam(r, "page"))
	if !ok {
		roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "no such transcript page")
		return
	}
	maxSequence, err := h.sequences.MaxSequence(r.Context(), room.ID)
	if err != nil {
		slog.Error("failed to read room sequence", "error", err, "room_id", room.ID)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to read the transcript")
		return
	}
	total := historyTotalPages(maxSequence)
	if page > total {
		roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "no such transcript page")
		return
	}
	from, to := (page-1)*HistoryPageSize+1, page*HistoryPageSize
	messages, err := h.messages.ListSequenceRange(r.Context(), room.ID, from, to)
	if err != nil {
		slog.Error("failed to read transcript page", "error", err, "room_id", room.ID, "page", page)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to read the transcript")
		return
	}
	out := RoomHistoryPage{
		Page: page, PageSize: HistoryPageSize, FromSequence: from, ToSequence: to,
		TotalPages: total, Messages: messages,
	}
	if page > 1 {
		prev := page - 1
		out.PrevPage = &prev
	}
	if page < total {
		next := page + 1
		out.NextPage = &next
	}
	roomWriteJSON(w, http.StatusOK, map[string]RoomHistoryPage{"data": out})
}

// SetHistoryReader wires the archive size into GET /v1/rooms/{slug} (data.history).
// Optional: with none wired the room read carries no history.
func (h *RoomHandler) SetHistoryReader(sequences roomSequenceReader) {
	h.history = sequences
}

// historyInfo is the room read's data.history; an error is the caller's to answer.
func (h *RoomHandler) historyInfo(ctx context.Context, roomID uuid.UUID) (*RoomHistoryInfo, error) {
	if h.history == nil {
		return nil, nil
	}
	maxSequence, err := h.history.MaxSequence(ctx, roomID)
	if err != nil {
		return nil, err
	}
	return &RoomHistoryInfo{PageSize: HistoryPageSize, TotalPages: historyTotalPages(maxSequence)}, nil
}
