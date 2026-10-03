package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/api/response"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// ErrCodeLegacyFieldRetired answers a request that still sends a type, field, status or
// filter of the legacy post model retired in idx 68: 400 naming the field and what to send
// instead, never a silent success (the idx 52 rule for retired writes).
const ErrCodeLegacyFieldRetired = "LEGACY_FIELD_RETIRED"

// Instructions for each retired input.
const (
	legacyTypeInstead = "Every post is type \"post\": leave type out or send \"post\". The legacy problem, " +
		"question and idea types were retired with their typed workflows; contributions are replies " +
		"(POST /v1/posts/{id}/replies)."
	legacyProblemFieldInstead = "The legacy problem, question and idea fields (weight, success_criteria, " +
		"accepted_answer_id, evolved_into) were retired and are no longer stored: put what they carried in " +
		"the description or in a reply."
	legacyStatusInstead = "The legacy statuses in_progress, solved, answered, active, dormant and evolved were " +
		"retired: a post's status is draft, pending_review, rejected, open, closed or stale, and record an " +
		"outcome as a reply."
)

// legacyFieldDetails is the details object of a LEGACY_FIELD_RETIRED error.
type legacyFieldDetails struct {
	Field        string `json:"field"`
	Value        string `json:"value,omitempty"`
	Instructions string `json:"instructions"`
}

// writeLegacyFieldRetired answers 400 LEGACY_FIELD_RETIRED for field (and the value sent).
func writeLegacyFieldRetired(w http.ResponseWriter, field, value, instructions string) {
	response.WriteErrorWithDetails(w, http.StatusBadRequest, ErrCodeLegacyFieldRetired,
		field+" was retired with the legacy post types (idx 68). "+instructions,
		legacyFieldDetails{Field: field, Value: value, Instructions: instructions})
}

// retiredPostFields are the legacy problem-only fields a post write body may no longer carry.
var retiredPostFields = []string{"success_criteria", "weight", "accepted_answer_id", "evolved_into"}

// decodePostBody reads a post write body into req and returns the first retired field it
// carries (present at all, null included), or "". The request structs stay the published
// contract; the retired keys are looked up in the raw object.
func decodePostBody(r *http.Request, req any) (string, error) {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(raw, req); err != nil {
		return "", err
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		return "", err
	}
	for _, field := range retiredPostFields {
		if _, ok := keys[field]; ok {
			return field, nil
		}
	}
	return "", nil
}

// legacyFieldError is a retired legacy type or status sent as a list or search filter.
type legacyFieldError struct {
	field, value, instructions string
}

func (e *legacyFieldError) Error() string {
	return e.field + " " + e.value + " was retired with the legacy post types (idx 68)"
}

// checkLegacyPostFilters refuses a legacy type or status filter: a type filter is "post" or
// "all" (both mean every post) or absent, and a status is never one of the retired statuses. It
// returns the type filter to apply, always "" since every post is type "post".
func checkLegacyPostFilters(typeParam, statusParam string) (models.PostType, error) {
	switch {
	case typeParam == "" || typeParam == "all" || models.IsValidPostType(models.PostType(typeParam)):
	case models.IsRetiredPostType(models.PostType(typeParam)):
		return "", &legacyFieldError{field: "type", value: typeParam, instructions: legacyTypeInstead}
	default:
		return "", fmt.Errorf("type must be post or all, got %q", typeParam)
	}
	if models.IsRetiredPostStatus(models.PostStatus(statusParam)) {
		return "", &legacyFieldError{field: "status", value: statusParam, instructions: legacyStatusInstead}
	}
	return "", nil
}

// writeFilterError answers a filter error: LEGACY_FIELD_RETIRED for a retired type or status,
// otherwise 400 VALIDATION_ERROR.
func writeFilterError(w http.ResponseWriter, err error) {
	var legacy *legacyFieldError
	if errors.As(err, &legacy) {
		writeLegacyFieldRetired(w, legacy.field, legacy.value, legacy.instructions)
		return
	}
	response.WriteValidationError(w, err.Error(), nil)
}
