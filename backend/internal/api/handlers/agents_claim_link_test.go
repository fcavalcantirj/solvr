package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// The claim link is how an agent gets a human to become the person behind it, so the token
// in it is a credential. It must not sit in a URL path or query (browser history, the
// frontend host's access log and analytics page views all record those), and the API must
// not take it from a URL either (idx 75, step 5).

// DeleteUnusedByAgentID clears every unused token of an agent, as the database repository does.
func (m *MockClaimTokenRepository) DeleteUnusedByAgentID(ctx context.Context, agentID string) (int64, error) {
	m.deleteUnusedCalled = true
	var deleted int64
	for key, token := range m.tokens {
		if token.AgentID == agentID && !token.IsUsed() {
			delete(m.tokens, key)
			deleted++
		}
	}
	return deleted, nil
}

func claimHandlerFor(agent *models.Agent, claimRepo *MockClaimTokenRepository) *AgentsHandler {
	agentRepo := NewMockAgentRepository()
	agentRepo.agents[agent.ID] = agent
	handler := NewAgentsHandler(agentRepo, "test-secret")
	handler.SetClaimTokenRepository(claimRepo)
	handler.SetBaseURL("https://solvr.dev")
	return handler
}

func generateClaimAs(handler *AgentsHandler, agent *models.Agent) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/me/claim", nil)
	req = req.WithContext(auth.ContextWithAgent(req.Context(), agent))
	w := httptest.NewRecorder()
	handler.GenerateClaim(w, req)
	return w
}

func lookupClaimBody(handler *AgentsHandler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/claim/lookup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.LookupClaim(w, req)
	return w
}

func TestGenerateClaim_LinkCarriesTheTokenInAFragmentNeverInThePathOrQuery(t *testing.T) {
	agent := &models.Agent{ID: "frag_agent", DisplayName: "Fragment Agent", Status: "active"}
	handler := claimHandlerFor(agent, NewMockClaimTokenRepository())

	w := generateClaimAs(handler, agent)
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var resp GenerateClaimResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Token == "" {
		t.Fatal("no token in the response")
	}

	u, err := url.Parse(resp.ClaimURL)
	if err != nil {
		t.Fatalf("claim_url %q does not parse: %v", resp.ClaimURL, err)
	}
	if u.Scheme != "https" || u.Host != "solvr.dev" || u.Path != "/claim" {
		t.Errorf("claim_url = %q; want https://solvr.dev/claim with the token after #", resp.ClaimURL)
	}
	if u.RawQuery != "" {
		t.Errorf("claim_url has a query %q; a query is sent to the server and logged", u.RawQuery)
	}
	if u.Fragment != "token="+resp.Token {
		t.Errorf("claim_url fragment = %q, want token=<the token>", u.Fragment)
	}
	if strings.Contains(u.Path, resp.Token) {
		t.Error("the token is in the URL path")
	}
}

func TestGenerateClaim_RepeatRequestHandsTheSameLinkBack(t *testing.T) {
	agent := &models.Agent{ID: "repeat_agent", DisplayName: "Repeat Agent", Status: "active"}
	handler := claimHandlerFor(agent, NewMockClaimTokenRepository())

	first := generateClaimAs(handler, agent)
	second := generateClaimAs(handler, agent)
	if first.Code != http.StatusCreated || second.Code != http.StatusOK {
		t.Fatalf("statuses %d then %d, want 201 then 200", first.Code, second.Code)
	}
	var a, b GenerateClaimResponse
	if err := json.Unmarshal(first.Body.Bytes(), &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(second.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	if a.ClaimURL == "" || a.ClaimURL != b.ClaimURL || a.Token != b.Token {
		t.Errorf("a repeat request must not invalidate the first link: %q then %q", a.ClaimURL, b.ClaimURL)
	}
}

func TestGenerateClaim_ALiveTokenThatCannotBeShownIsReplacedNotReturnedBlank(t *testing.T) {
	agent := &models.Agent{ID: "unshowable_agent", DisplayName: "Unshowable Agent", Status: "active"}
	claimRepo := NewMockClaimTokenRepository()
	// What the database repository returns for a row created before tokens were sealed, or
	// sealed under a secret that has since changed: live, but the token is unrecoverable.
	claimRepo.tokens["legacy"] = &models.ClaimToken{
		ID: "legacy-id", AgentID: agent.ID, ExpiresAt: time.Now().Add(2 * time.Hour), CreatedAt: time.Now().Add(-2 * time.Hour),
	}
	handler := claimHandlerFor(agent, claimRepo)

	w := generateClaimAs(handler, agent)
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d, want 201 with a fresh token: %s", w.Code, w.Body.String())
	}
	var resp GenerateClaimResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Token == "" || !strings.HasSuffix(resp.ClaimURL, "#token="+resp.Token) {
		t.Errorf("response %+v; a blank token or link must never be handed out", resp)
	}
	if !claimRepo.deleteUnusedCalled {
		t.Error("the unshowable live token was not cleared before creating the new one")
	}
	if _, stillThere := claimRepo.tokens["legacy"]; stillThere {
		t.Error("the unshowable live token is still in the repository")
	}
}

func TestLookupClaim_ReadsTheTokenFromTheBodyOnly(t *testing.T) {
	agent := &models.Agent{ID: "lookup_agent", DisplayName: "Lookup Agent", Status: "active", APIKeyHash: "hash-not-for-clients"}
	claimRepo := NewMockClaimTokenRepository()
	claimRepo.tokens["live_token_value"] = &models.ClaimToken{
		ID: "live-id", Token: "live_token_value", AgentID: agent.ID, ExpiresAt: time.Now().Add(time.Hour), CreatedAt: time.Now(),
	}
	handler := claimHandlerFor(agent, claimRepo)

	w := lookupClaimBody(handler, `{"token":"live_token_value"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var resp ClaimInfoResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !resp.TokenValid || resp.Agent == nil || resp.Agent.ID != agent.ID {
		t.Errorf("response %+v; want a valid token and the agent", resp)
	}
	if strings.Contains(w.Body.String(), "hash-not-for-clients") {
		t.Error("the agent's api_key_hash was sent to a public caller")
	}

	// A token in the URL is never read: the endpoint has no path parameter and ignores the query.
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/claim/lookup?token=live_token_value", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	handler.LookupClaim(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("a token only in the URL answered %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestLookupClaim_RefusesBadBodiesWithStableCodes(t *testing.T) {
	agent := &models.Agent{ID: "lookup_bad_agent", DisplayName: "Lookup Bad Agent", Status: "active"}
	handler := claimHandlerFor(agent, NewMockClaimTokenRepository())

	cases := []struct {
		name     string
		body     string
		wantCode string
	}{
		{"malformed JSON", `{"token":`, "VALIDATION_ERROR"},
		{"not an object", `["live_token_value"]`, "VALIDATION_ERROR"},
		{"missing token", `{}`, "MISSING_TOKEN"},
		{"empty token", `{"token":""}`, "MISSING_TOKEN"},
		{"oversized body", `{"token":"` + strings.Repeat("a", 8192) + `"}`, "VALIDATION_ERROR"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := lookupClaimBody(handler, tc.body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status %d, want 400: %s", w.Code, w.Body.String())
			}
			var resp struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || resp.Error.Code != tc.wantCode {
				t.Errorf("error code %q (%v), want %q: %s", resp.Error.Code, err, tc.wantCode, w.Body.String())
			}
		})
	}
}
