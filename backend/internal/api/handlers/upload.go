package handlers

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/fcavalcantirj/solvr/internal/api/response"
)

// DefaultMaxUploadSize is the default maximum upload size (100MB).
const DefaultMaxUploadSize = 100 * 1024 * 1024

// IPFSAdder defines the IPFS upload operation needed by the upload handler.
// Defined locally to avoid import cycle with services package.
type IPFSAdder interface {
	Add(ctx context.Context, reader io.Reader) (string, error)
}

// UploadHandler handles file upload to IPFS via POST /v1/add.
type UploadHandler struct {
	ipfs          IPFSAdder
	maxUploadSize int64
	logger        *slog.Logger
}

// NewUploadHandler creates a new UploadHandler.
func NewUploadHandler(ipfs IPFSAdder, maxUploadSize int64) *UploadHandler {
	return &UploadHandler{
		ipfs:          ipfs,
		maxUploadSize: maxUploadSize,
		logger:        slog.New(slog.NewJSONHandler(os.Stderr, nil)),
	}
}

// AddContentResponse represents the response from POST /v1/add.
type AddContentResponse struct {
	CID  string `json:"cid"`
	Size int64  `json:"size"`
}

// AddContent handles POST /v1/add — upload content to IPFS and return CID.
// Accepts multipart/form-data with a 'file' field.
// Does NOT auto-pin — user must call POST /v1/pins separately.
func (h *UploadHandler) AddContent(w http.ResponseWriter, r *http.Request) {
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		response.WriteUnauthorized(w, "authentication required")
		return
	}

	// Limit request body size to prevent abuse
	r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadSize)

	// Parse multipart form
	if err := r.ParseMultipartForm(h.maxUploadSize); err != nil {
		if err.Error() == "http: request body too large" {
			response.WriteError(w, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE",
				"file exceeds maximum upload size")
			return
		}
		response.WriteError(w, http.StatusBadRequest, response.ErrCodeValidation,
			"request must be multipart/form-data with a 'file' field")
		return
	}

	// Get the file from the 'file' field
	file, _, err := r.FormFile("file")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.ErrCodeValidation,
			"missing 'file' field in multipart form")
		return
	}
	defer file.Close()

	// Read file content to check size and prepare for IPFS upload
	var buf bytes.Buffer
	n, err := io.Copy(&buf, file)
	if err != nil {
		ctx := response.LogContext{
			Operation: "AddContent-read",
			Resource:  "upload",
			RequestID: r.Header.Get("X-Request-ID"),
		}
		response.WriteInternalErrorWithLog(w, "failed to read uploaded file", err, ctx, h.logger)
		return
	}

	// Reject empty files
	if n == 0 {
		response.WriteError(w, http.StatusBadRequest, response.ErrCodeValidation,
			"file must not be empty")
		return
	}

	// Upload to IPFS
	cid, err := h.ipfs.Add(r.Context(), &buf)
	if err != nil {
		ctx := response.LogContext{
			Operation: "AddContent-ipfs",
			Resource:  "upload",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra: map[string]string{
				"ownerType": string(authInfo.AuthorType),
				"ownerID":   authInfo.AuthorID,
			},
		}
		response.WriteInternalErrorWithLog(w, "failed to upload to IPFS", err, ctx, h.logger)
		return
	}

	response.WriteJSON(w, http.StatusOK, AddContentResponse{
		CID:  cid,
		Size: n,
	})
}
