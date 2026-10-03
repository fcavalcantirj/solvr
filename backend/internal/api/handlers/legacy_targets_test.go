package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// idx 68: the legacy contribution types became replies and the legacy archive migration
// narrows the reports and flags target checks to post and reply. A report or flag naming a
// legacy type is refused with 400 LEGACY_FIELD_RETIRED (target_type) before it reaches the
// database, never a 500 from the check.

type legacyTargetReportsRepo struct{ created *models.Report }

func (r *legacyTargetReportsRepo) Create(_ context.Context, report *models.Report) (*models.Report, error) {
	r.created = report
	return report, nil
}

func (r *legacyTargetReportsRepo) HasReported(context.Context, models.ReportTargetType, string, string, string) (bool, error) {
	return false, nil
}

func TestCreateReport_LegacyTargetTypeIsLegacyFieldRetired(t *testing.T) {
	for _, target := range []string{"answer", "approach", "response", "comment"} {
		repo := &legacyTargetReportsRepo{}
		raw, _ := json.Marshal(map[string]any{"target_type": target, "target_id": uuid.NewString(), "reason": "spam"})
		req := addAuthContext(httptest.NewRequest(http.MethodPost, "/v1/reports", bytes.NewReader(raw)), "user-123", "user")
		w := httptest.NewRecorder()
		NewReportsHandler(repo).Create(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: status %d, want 400: %s", target, w.Code, w.Body.String())
		}
		if code, field := decodeLegacyFieldError(t, w); code != ErrCodeLegacyFieldRetired || field != "target_type" {
			t.Errorf("%s: code %q field %q, want %s/target_type", target, code, field, ErrCodeLegacyFieldRetired)
		}
		if repo.created != nil {
			t.Errorf("%s: a refused report must not be stored", target)
		}
	}
	for _, target := range []string{"post", "reply"} {
		repo := &legacyTargetReportsRepo{}
		raw, _ := json.Marshal(map[string]any{"target_type": target, "target_id": uuid.NewString(), "reason": "spam"})
		req := addAuthContext(httptest.NewRequest(http.MethodPost, "/v1/reports", bytes.NewReader(raw)), "user-123", "user")
		w := httptest.NewRecorder()
		NewReportsHandler(repo).Create(w, req)
		if w.Code != http.StatusCreated || repo.created == nil {
			t.Errorf("%s: status %d, want 201 and a stored report: %s", target, w.Code, w.Body.String())
		}
	}
}

func TestCheckReport_LegacyTargetTypeIsLegacyFieldRetired(t *testing.T) {
	req := addAuthContext(httptest.NewRequest(http.MethodGet, "/v1/reports/check?target_type=answer&target_id="+uuid.NewString(), nil), "user-123", "user")
	w := httptest.NewRecorder()
	NewReportsHandler(&legacyTargetReportsRepo{}).Check(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400: %s", w.Code, w.Body.String())
	}
	if code, field := decodeLegacyFieldError(t, w); code != ErrCodeLegacyFieldRetired || field != "target_type" {
		t.Errorf("code %q field %q, want %s/target_type", code, field, ErrCodeLegacyFieldRetired)
	}
}

func TestCreateFlag_LegacyTargetTypeIsLegacyFieldRetired(t *testing.T) {
	for _, target := range []string{"comment", "answer", "approach", "response", "progress_note"} {
		stored := false
		repo := &MockFlagsRepository{CreateFlagFunc: func(_ context.Context, flag *models.Flag) (*models.Flag, error) {
			stored = true
			return flag, nil
		}}
		raw, _ := json.Marshal(map[string]any{"target_type": target, "target_id": uuid.NewString(), "reason": "spam"})
		req := addFlagsAuthContext(httptest.NewRequest(http.MethodPost, "/v1/flags", bytes.NewReader(raw)),
			&auth.Claims{UserID: uuid.NewString(), Email: "flagger@example.com", Role: "user"})
		w := httptest.NewRecorder()
		NewFlagsHandler(repo).Create(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: status %d, want 400: %s", target, w.Code, w.Body.String())
		}
		if code, field := decodeLegacyFieldError(t, w); code != ErrCodeLegacyFieldRetired || field != "target_type" {
			t.Errorf("%s: code %q field %q, want %s/target_type", target, code, field, ErrCodeLegacyFieldRetired)
		}
		if stored {
			t.Errorf("%s: a refused flag must not be stored", target)
		}
	}
}
