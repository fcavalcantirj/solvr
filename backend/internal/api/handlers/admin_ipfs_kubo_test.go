package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/services"
)

type noRefs struct{}

func (noRefs) CIDReferences(context.Context, string) (int, int, error) { return 0, 0, nil }

// Anti-abuse W5 against a real Kubo node (the local solvr-ipfs container): add pins by default,
// the endpoint unpins, and a second run reports not_pinned. Skipped without IPFS_API_URL.
func TestAdminIPFS_UnpinOnAKuboNode(t *testing.T) {
	apiURL := os.Getenv("IPFS_API_URL")
	if apiURL == "" {
		t.Skip("IPFS_API_URL not set, skipping Kubo integration test")
	}
	t.Setenv("ADMIN_API_KEY", "anti-abuse-admin-key")
	kubo := services.NewKuboIPFSService(apiURL)
	cid, err := kubo.Add(context.Background(), strings.NewReader("anti-abuse unpin probe "+t.Name()+" "+os.Getenv("HOSTNAME")))
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	h := handlers.NewAdminIPFSHandler(kubo, kubo, noRefs{})

	run := func() string {
		req := httptest.NewRequest(http.MethodPost, "/admin/ipfs/unpin", strings.NewReader(`{"cids":["`+cid+`"]}`))
		req.Header.Set("X-Admin-API-Key", "anti-abuse-admin-key")
		rec := httptest.NewRecorder()
		h.Unpin(rec, req)
		var out struct {
			Data struct {
				Results []handlers.UnpinResult `json:"results"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || len(out.Data.Results) != 1 {
			t.Fatalf("response %d: %s", rec.Code, rec.Body.String())
		}
		return out.Data.Results[0].Status + " " + out.Data.Results[0].Detail
	}
	if got := run(); got != "unpinned " {
		t.Fatalf("first run = %q, want unpinned", got)
	}
	if got := run(); got != "not_pinned " {
		t.Fatalf("second run = %q, want not_pinned", got)
	}
}
