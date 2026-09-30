package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeUnpinner struct {
	calls []string
	err   map[string]error
}

func (f *fakeUnpinner) Unpin(_ context.Context, cid string) error {
	f.calls = append(f.calls, cid)
	return f.err[cid]
}

type fakeGC struct{ removed int }

func (f *fakeGC) RepoGC(context.Context) (int, error) { return f.removed, nil }

type fakeRefs map[string][2]int

func (f fakeRefs) CIDReferences(_ context.Context, cid string) (int, int, error) {
	r := f[cid]
	return r[0], r[1], nil
}

const (
	cidFree     = "QmbrPqJC7j1mVmPsjbjyzhU2qq8FFeYyMxox7N5Ztsdzip"
	cidPinned   = "QmQ7HMadRQXr3QNZeF7dQGQtHWxgVDc9LaYdBRb5EP8zZi"
	cidPost     = "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG"
	cidGone     = "QmT78zSuBmuS4z925WZfrqQ1qHaJ56DQaTfyMUF7F8ff5o"
	cidV1       = "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi"
	adminKeyEnv = "anti-abuse-admin-key"
)

func unpinRequest(t *testing.T, h *AdminIPFSHandler, body string) (int, map[string]int, map[string]string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/admin/ipfs/unpin", strings.NewReader(body))
	req.Header.Set("X-Admin-API-Key", adminKeyEnv)
	rec := httptest.NewRecorder()
	h.Unpin(rec, req)
	var out struct {
		Data struct {
			Results []UnpinResult  `json:"results"`
			Summary map[string]int `json:"summary"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	byCID := map[string]string{}
	for _, r := range out.Data.Results {
		byCID[r.CID] = r.Status
	}
	return rec.Code, out.Data.Summary, byCID
}

func TestAdminIPFS_UnpinRefusesInUseAndInvalidCIDs(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", adminKeyEnv)
	unpinner := &fakeUnpinner{err: map[string]error{cidGone: errors.New(`ipfs: request returned status 500: {"Message":"not pinned or pinned indirectly"}`)}}
	h := NewAdminIPFSHandler(unpinner, &fakeGC{}, fakeRefs{cidPinned: {1, 0}, cidPost: {0, 1}})
	body := `{"cids":["` + cidFree + `","` + cidPinned + `","` + cidPost + `","` + cidGone + `","` + cidV1 + `","QmX&arg=QmY","Qm+/bad"]}`

	// Dry run: the report, and not one call to the node.
	code, summary, byCID := unpinRequest(t, h, strings.Replace(body, `]}`, `],"dry_run":true}`, 1))
	if code != http.StatusOK || len(unpinner.calls) != 0 {
		t.Fatalf("dry run: code %d, unpin calls %v", code, unpinner.calls)
	}
	if byCID[cidFree] != "would_unpin" || byCID[cidPinned] != "in_use_pin" || byCID[cidPost] != "in_use_post" ||
		byCID["QmX&arg=QmY"] != "invalid" || byCID["Qm+/bad"] != "invalid" || summary["would_unpin"] != 3 {
		t.Fatalf("dry run results = %v, summary %v", byCID, summary)
	}

	code, summary, byCID = unpinRequest(t, h, body)
	if code != http.StatusOK {
		t.Fatalf("code %d", code)
	}
	if byCID[cidFree] != "unpinned" || byCID[cidV1] != "unpinned" || byCID[cidGone] != "not_pinned" || byCID[cidPinned] != "in_use_pin" {
		t.Fatalf("results = %v", byCID)
	}
	if strings.Join(unpinner.calls, ",") != strings.Join([]string{cidFree, cidGone, cidV1}, ",") {
		t.Fatalf("the node was asked to unpin %v; in-use and invalid CIDs must never reach it", unpinner.calls)
	}
	if summary["invalid"] != 2 || summary["in_use_pin"] != 1 || summary["in_use_post"] != 1 {
		t.Fatalf("summary = %v", summary)
	}
}

func TestAdminIPFS_RequiresAdminKeyAndBoundsTheList(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", adminKeyEnv)
	h := NewAdminIPFSHandler(&fakeUnpinner{}, &fakeGC{removed: 3}, fakeRefs{})
	req := httptest.NewRequest(http.MethodPost, "/admin/ipfs/unpin", strings.NewReader(`{"cids":["`+cidFree+`"]}`))
	rec := httptest.NewRecorder()
	h.Unpin(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no key: %d", rec.Code)
	}
	if code, _, _ := unpinRequest(t, h, `{"cids":[]}`); code != http.StatusBadRequest {
		t.Fatalf("empty list: %d", code)
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/ipfs/gc", nil)
	req.Header.Set("X-Admin-API-Key", adminKeyEnv)
	rec = httptest.NewRecorder()
	h.GC(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"removed":3`) {
		t.Fatalf("gc: %d %s", rec.Code, rec.Body.String())
	}
}
