package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// agent is one registered load-test agent with its own client address. Each agent sends
// CF-Connecting-IP, the only forwarded address the API trusts, so the API's per-IP write
// limit (60/min, router_rooms.go) buckets it apart, as it buckets distinct real clients; the
// harness never raises or removes that limit.
type agent struct {
	key  string
	ip   string
	slug string // the room it belongs to
}

type target struct {
	base         string
	client       *http.Client
	agents       []agent
	slugs        []string
	postIDs      []string
	terms        []string
	runID        string
	anonIPs      []string
	streamsEnded atomic.Int64 // streams the server closed before the run ended
}

// do sends one request and returns the status and body. The body is read fully so the
// measured latency includes the whole response.
func (t *target) do(ctx context.Context, method, path, bearer, ip string, body any) (int, []byte, error) {
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, t.base+path, rdr)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if ip != "" {
		req.Header.Set("CF-Connecting-IP", ip)
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	return resp.StatusCode, raw, err
}

func clientIP(block, i int) string { return fmt.Sprintf("10.%d.%d.%d", block, i/250, i%250+1) }

// waitHealthy polls /health until it answers 200 or the deadline passes.
func (t *target) waitHealthy(ctx context.Context, within time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, within)
	defer cancel()
	for {
		status, _, err := t.do(ctx, http.MethodGet, "/health", "", "", nil)
		if err == nil && status == http.StatusOK {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("API not healthy within %s (last status %d, err %v)", within, status, err)
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// setup registers the agents, opens the public rooms (one agent opens each, the rest
// handshake into theirs) and samples post ids and search terms from the dataset.
func (t *target) setup(ctx context.Context, agents, rooms int) error {
	for i := 0; i < agents; i++ {
		ip := clientIP(77, i)
		name := fmt.Sprintf("lt_%s_%04d", t.runID, i)
		status, raw, err := t.do(ctx, http.MethodPost, "/v1/agents/register", "", ip, map[string]string{"name": name})
		if err != nil || status != http.StatusCreated {
			return fmt.Errorf("register %s: %d %v %s", name, status, err, clipBody(raw))
		}
		var out struct {
			APIKey string `json:"api_key"`
		}
		if err := json.Unmarshal(raw, &out); err != nil || out.APIKey == "" {
			return fmt.Errorf("register %s: no api_key", name)
		}
		t.agents = append(t.agents, agent{key: out.APIKey, ip: ip})
	}
	for r := 0; r < rooms; r++ {
		a := &t.agents[r]
		slug := fmt.Sprintf("lt-%s-%02d", t.runID, r)
		status, raw, err := t.do(ctx, http.MethodPost, "/v1/rooms", a.key, a.ip,
			map[string]any{"display_name": "Load room " + slug, "slug": slug})
		if err != nil || status != http.StatusCreated {
			return fmt.Errorf("open room %s: %d %v %s", slug, status, err, clipBody(raw))
		}
		a.slug = slug
		t.slugs = append(t.slugs, slug)
	}
	for i := rooms; i < len(t.agents); i++ {
		a := &t.agents[i]
		a.slug = t.slugs[i%rooms]
		status, raw, err := t.do(ctx, http.MethodPost, "/v1/rooms/"+a.slug+"/handshake", a.key, a.ip, map[string]any{})
		if err != nil || status != http.StatusCreated {
			return fmt.Errorf("handshake into %s: %d %v %s", a.slug, status, err, clipBody(raw))
		}
	}

	status, raw, err := t.do(ctx, http.MethodGet, "/v1/posts?per_page=50", "", "", nil)
	if err != nil || status != http.StatusOK {
		return fmt.Errorf("sample posts: %d %v", status, err)
	}
	var posts struct {
		Data []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &posts); err != nil {
		return fmt.Errorf("sample posts: %w", err)
	}
	seen := map[string]bool{}
	for _, p := range posts.Data {
		t.postIDs = append(t.postIDs, p.ID)
		for _, w := range strings.Fields(strings.ToLower(p.Title)) {
			w = strings.Trim(w, `.,:;!?()[]"'`)
			if len(w) >= 5 && !seen[w] && len(t.terms) < 60 {
				seen[w] = true
				t.terms = append(t.terms, w)
			}
		}
	}
	if len(t.postIDs) == 0 {
		return fmt.Errorf("the dataset has no public posts to read")
	}
	if len(t.terms) == 0 {
		t.terms = []string{"error", "timeout", "database", "agent", "deploy"}
	}
	for i := 0; i < 200; i++ {
		t.anonIPs = append(t.anonIPs, clientIP(78, i))
	}
	return nil
}

func clipBody(raw []byte) string {
	if len(raw) > 200 {
		return string(raw[:200]) + "…"
	}
	return string(raw)
}
