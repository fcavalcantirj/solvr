package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Anti-abuse W5: a CID is escaped at every arg= site, so an operator-supplied value cannot add
// or change query parameters; repo/gc reports how many keys it removed.
func TestKuboIPFSService_EscapesArgAndCountsGC(t *testing.T) {
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.Path+"?"+r.URL.RawQuery)
		if r.URL.Path == "/api/v0/repo/gc" {
			w.Write([]byte("{\"Key\":{\"/\":\"QmA\"}}\n{\"Key\":{\"/\":\"QmB\"}}\n")) //nolint:errcheck
			return
		}
		w.Write([]byte(`{"Keys":{"x":{"Type":"recursive"}},"Size":1}`)) //nolint:errcheck
	}))
	defer srv.Close()
	svc := NewKuboIPFSServiceWithConfig(srv.URL, IPFSConfig{Timeout: 0})
	ctx := context.Background()
	evil := "QmX&arg=QmY+z/w"

	_ = svc.Unpin(ctx, evil)
	_ = svc.Pin(ctx, evil)
	_, _ = svc.PinStatus(ctx, evil)
	_, _ = svc.ObjectStat(ctx, evil)
	want := []string{
		"/api/v0/pin/rm?arg=QmX%26arg%3DQmY%2Bz%2Fw",
		"/api/v0/pin/add?arg=QmX%26arg%3DQmY%2Bz%2Fw&progress=false",
		"/api/v0/pin/ls?arg=QmX%26arg%3DQmY%2Bz%2Fw",
		"/api/v0/dag/stat?arg=QmX%26arg%3DQmY%2Bz%2Fw",
	}
	if len(queries) < len(want) {
		t.Fatalf("requests = %v", queries)
	}
	for i, q := range want {
		if queries[i] != q {
			t.Errorf("request %d = %q, want %q", i, queries[i], q)
		}
	}

	removed, err := svc.RepoGC(ctx)
	if err != nil || removed != 2 {
		t.Fatalf("RepoGC = %d, %v; want 2 keys", removed, err)
	}
}
