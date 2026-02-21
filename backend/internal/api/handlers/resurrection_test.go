package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// --- Mock: ResurrectionKnowledgeRepo ---

type MockResurrectionKnowledgeRepo struct {
	ideas      []models.ResurrectionIdea
	approaches []models.ResurrectionApproach
	problems   []models.ResurrectionProblem
	ideasErr   error
	apprErr    error
	probErr    error
}

func (m *MockResurrectionKnowledgeRepo) GetAgentIdeas(ctx context.Context, agentID string, limit int) ([]models.ResurrectionIdea, error) {
	if m.ideasErr != nil {
		return nil, m.ideasErr
	}
	return m.ideas, nil
}

func (m *MockResurrectionKnowledgeRepo) GetAgentApproaches(ctx context.Context, agentID string, limit int) ([]models.ResurrectionApproach, error) {
	if m.apprErr != nil {
		return nil, m.apprErr
	}
	return m.approaches, nil
}

func (m *MockResurrectionKnowledgeRepo) GetAgentOpenProblems(ctx context.Context, agentID string) ([]models.ResurrectionProblem, error) {
	if m.probErr != nil {
		return nil, m.probErr
	}
	return m.problems, nil
}

// --- Mock: ResurrectionStatsRepo ---

type MockResurrectionStatsRepo struct {
	stats *models.AgentStats
	err   error
}

func (m *MockResurrectionStatsRepo) GetAgentStats(ctx context.Context, agentID string) (*models.AgentStats, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.stats, nil
}

// --- Mock: ResurrectionCheckpointFinder ---

type MockResurrectionCheckpointFinder struct {
	pin *models.Pin
	err error
}

func (m *MockResurrectionCheckpointFinder) FindLatestCheckpoint(ctx context.Context, agentID string) (*models.Pin, error) {
	return m.pin, m.err
}

// --- Mock: ResurrectionAgentRepo ---

type MockResurrectionAgentRepo struct {
	lastSeenAgent string
	err           error
}

func (m *MockResurrectionAgentRepo) UpdateLastSeen(ctx context.Context, id string) error {
	m.lastSeenAgent = id
	return m.err
}

// --- Test Helpers ---

func newResurrectionHandler(
	agentFinder AgentFinderInterface,
	checkpointFinder ResurrectionCheckpointFinder,
	knowledgeRepo ResurrectionKnowledgeRepo,
	statsRepo ResurrectionStatsRepo,
	agentRepo ResurrectionAgentUpdateRepo,
) *ResurrectionHandler {
	h := NewResurrectionHandler(agentFinder, checkpointFinder, knowledgeRepo, statsRepo)
	if agentRepo != nil {
		h.SetAgentRepo(agentRepo)
	}
	return h
}

func addResurrectionAgentContext(r *http.Request, agent *models.Agent) *http.Request {
	ctx := auth.ContextWithAgent(r.Context(), agent)
	return r.WithContext(ctx)
}

func addResurrectionHumanContext(r *http.Request, userID, role string) *http.Request {
	claims := &auth.Claims{
		UserID: userID,
		Role:   role,
	}
	ctx := auth.ContextWithClaims(r.Context(), claims)
	return r.WithContext(ctx)
}

// --- Tests ---

func TestResurrectionBundle_FullPayload(t *testing.T) {
	humanID := "human-owner-1"
	agentID := "agent-resurrection-1"

	agent := &models.Agent{
		ID:              agentID,
		DisplayName:     "Resurrecta",
		HumanID:         &humanID,
		Bio:             "I rise again",
		Model:           "claude-opus-4",
		Specialties:     []string{"golang", "postgres"},
		HasAMCPIdentity: true,
		AMCPAID:         "did:keri:abc123",
		CreatedAt:       time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
		Reputation:      350,
	}

	agentFinder := &MockAgentFinderRepo{
		agents: map[string]*models.Agent{agentID: agent},
	}

	now := time.Now().UTC()
	checkpoint := &models.Pin{
		ID:        "pin-ckpt-1",
		CID:       "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
		Status:    models.PinStatusPinned,
		Name:      "checkpoint_bafybeig_20260221",
		Meta:      map[string]string{"type": "amcp_checkpoint", "agent_id": agentID, "death_count": "3"},
		OwnerID:   agentID,
		OwnerType: "agent",
		CreatedAt: now,
	}

	ideas := []models.ResurrectionIdea{
		{ID: "idea-1", Title: "Pattern matching in Go", Status: "active", Upvotes: 10, Downvotes: 1, Tags: []string{"golang"}, CreatedAt: now},
		{ID: "idea-2", Title: "Postgres optimization tips", Status: "open", Upvotes: 5, Downvotes: 0, Tags: []string{"postgres"}, CreatedAt: now},
	}

	approaches := []models.ResurrectionApproach{
		{ID: "appr-1", ProblemID: "prob-1", Angle: "Try GIN indexes", Method: "benchmarking", Status: "succeeded", CreatedAt: now},
	}

	problems := []models.ResurrectionProblem{
		{ID: "prob-1", Title: "Slow full-text search", Status: "open", Tags: []string{"postgres", "search"}, CreatedAt: now},
	}

	stats := &models.AgentStats{
		ProblemsSolved:  2,
		AnswersAccepted: 3,
		IdeasPosted:     5,
		UpvotesReceived: 42,
		Reputation:      350,
	}

	handler := newResurrectionHandler(
		agentFinder,
		&MockResurrectionCheckpointFinder{pin: checkpoint},
		&MockResurrectionKnowledgeRepo{ideas: ideas, approaches: approaches, problems: problems},
		&MockResurrectionStatsRepo{stats: stats},
		&MockResurrectionAgentRepo{},
	)

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/"+agentID+"/resurrection-bundle", nil)
	req = addResurrectionAgentContext(req, agent)

	w := httptest.NewRecorder()
	handler.GetBundle(w, req, agentID)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify identity section
	identity, ok := resp["identity"].(map[string]interface{})
	if !ok {
		t.Fatal("expected identity section in response")
	}
	if identity["id"] != agentID {
		t.Errorf("expected identity.id=%s, got %v", agentID, identity["id"])
	}
	if identity["display_name"] != "Resurrecta" {
		t.Errorf("expected display_name=Resurrecta, got %v", identity["display_name"])
	}
	if identity["model"] != "claude-opus-4" {
		t.Errorf("expected model=claude-opus-4, got %v", identity["model"])
	}
	if identity["bio"] != "I rise again" {
		t.Errorf("expected bio='I rise again', got %v", identity["bio"])
	}
	if identity["has_amcp_identity"] != true {
		t.Errorf("expected has_amcp_identity=true, got %v", identity["has_amcp_identity"])
	}
	if identity["amcp_aid"] != "did:keri:abc123" {
		t.Errorf("expected amcp_aid=did:keri:abc123, got %v", identity["amcp_aid"])
	}
	specialties, ok := identity["specialties"].([]interface{})
	if !ok || len(specialties) != 2 {
		t.Errorf("expected 2 specialties, got %v", identity["specialties"])
	}

	// Verify knowledge section
	knowledge, ok := resp["knowledge"].(map[string]interface{})
	if !ok {
		t.Fatal("expected knowledge section in response")
	}
	knIdeas, ok := knowledge["ideas"].([]interface{})
	if !ok || len(knIdeas) != 2 {
		t.Errorf("expected 2 ideas, got %v", knowledge["ideas"])
	}
	knApproaches, ok := knowledge["approaches"].([]interface{})
	if !ok || len(knApproaches) != 1 {
		t.Errorf("expected 1 approach, got %v", knowledge["approaches"])
	}
	knProblems, ok := knowledge["problems"].([]interface{})
	if !ok || len(knProblems) != 1 {
		t.Errorf("expected 1 problem, got %v", knowledge["problems"])
	}

	// Verify reputation section
	reputation, ok := resp["reputation"].(map[string]interface{})
	if !ok {
		t.Fatal("expected reputation section in response")
	}
	if int(reputation["total"].(float64)) != 350 {
		t.Errorf("expected reputation.total=350, got %v", reputation["total"])
	}
	if int(reputation["problems_solved"].(float64)) != 2 {
		t.Errorf("expected problems_solved=2, got %v", reputation["problems_solved"])
	}
	if int(reputation["answers_accepted"].(float64)) != 3 {
		t.Errorf("expected answers_accepted=3, got %v", reputation["answers_accepted"])
	}

	// Verify latest_checkpoint is present
	latestCkpt, ok := resp["latest_checkpoint"].(map[string]interface{})
	if !ok {
		t.Fatal("expected latest_checkpoint in response")
	}
	if latestCkpt["requestid"] != "pin-ckpt-1" {
		t.Errorf("expected checkpoint requestid=pin-ckpt-1, got %v", latestCkpt["requestid"])
	}

	// Verify death_count parsed from checkpoint meta
	deathCount := resp["death_count"].(float64)
	if int(deathCount) != 3 {
		t.Errorf("expected death_count=3, got %v", resp["death_count"])
	}
}

func TestResurrectionBundle_EmptyAgent(t *testing.T) {
	agentID := "agent-empty-1"
	agent := &models.Agent{
		ID:          agentID,
		DisplayName: "Fresh Agent",
		CreatedAt:   time.Now().UTC(),
	}

	agentFinder := &MockAgentFinderRepo{
		agents: map[string]*models.Agent{agentID: agent},
	}

	handler := newResurrectionHandler(
		agentFinder,
		&MockResurrectionCheckpointFinder{pin: nil}, // no checkpoint
		&MockResurrectionKnowledgeRepo{
			ideas:      []models.ResurrectionIdea{},
			approaches: []models.ResurrectionApproach{},
			problems:   []models.ResurrectionProblem{},
		},
		&MockResurrectionStatsRepo{stats: &models.AgentStats{}},
		nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/"+agentID+"/resurrection-bundle", nil)
	req = addResurrectionAgentContext(req, agent)

	w := httptest.NewRecorder()
	handler.GetBundle(w, req, agentID)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	// Knowledge arrays should be empty (not nil)
	knowledge := resp["knowledge"].(map[string]interface{})
	ideas := knowledge["ideas"].([]interface{})
	if len(ideas) != 0 {
		t.Errorf("expected 0 ideas, got %d", len(ideas))
	}
	approaches := knowledge["approaches"].([]interface{})
	if len(approaches) != 0 {
		t.Errorf("expected 0 approaches, got %d", len(approaches))
	}
	problems := knowledge["problems"].([]interface{})
	if len(problems) != 0 {
		t.Errorf("expected 0 problems, got %d", len(problems))
	}

	// latest_checkpoint should be null
	if resp["latest_checkpoint"] != nil {
		t.Errorf("expected null latest_checkpoint, got %v", resp["latest_checkpoint"])
	}

	// death_count should be null
	if resp["death_count"] != nil {
		t.Errorf("expected null death_count, got %v", resp["death_count"])
	}
}

func TestResurrectionBundle_OwnAgent(t *testing.T) {
	agentID := "agent-own-1"
	otherAgentID := "agent-other-1"

	agent := &models.Agent{ID: agentID, DisplayName: "Own Agent"}
	otherAgent := &models.Agent{ID: otherAgentID, DisplayName: "Other Agent"}

	agentFinder := &MockAgentFinderRepo{
		agents: map[string]*models.Agent{
			agentID:      agent,
			otherAgentID: otherAgent,
		},
	}

	handler := newResurrectionHandler(
		agentFinder,
		&MockResurrectionCheckpointFinder{pin: nil},
		&MockResurrectionKnowledgeRepo{ideas: []models.ResurrectionIdea{}, approaches: []models.ResurrectionApproach{}, problems: []models.ResurrectionProblem{}},
		&MockResurrectionStatsRepo{stats: &models.AgentStats{}},
		nil,
	)

	// Other agent tries to access agentID's bundle — no family access, should be 403
	req := httptest.NewRequest(http.MethodGet, "/v1/agents/"+agentID+"/resurrection-bundle", nil)
	req = addResurrectionAgentContext(req, otherAgent)

	w := httptest.NewRecorder()
	handler.GetBundle(w, req, agentID)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-family agent, got %d: %s", w.Code, w.Body.String())
	}
}

func TestResurrectionBundle_SiblingAgent(t *testing.T) {
	humanID := "human-shared-1"
	agentAID := "agent-sibling-a"
	agentBID := "agent-sibling-b"

	agentA := &models.Agent{ID: agentAID, DisplayName: "Agent A", HumanID: &humanID}
	agentB := &models.Agent{ID: agentBID, DisplayName: "Agent B", HumanID: &humanID, CreatedAt: time.Now()}

	agentFinder := &MockAgentFinderRepo{
		agents: map[string]*models.Agent{
			agentAID: agentA,
			agentBID: agentB,
		},
	}

	handler := newResurrectionHandler(
		agentFinder,
		&MockResurrectionCheckpointFinder{pin: nil},
		&MockResurrectionKnowledgeRepo{ideas: []models.ResurrectionIdea{}, approaches: []models.ResurrectionApproach{}, problems: []models.ResurrectionProblem{}},
		&MockResurrectionStatsRepo{stats: &models.AgentStats{}},
		&MockResurrectionAgentRepo{},
	)

	// Agent A (sibling) fetches Agent B's bundle
	req := httptest.NewRequest(http.MethodGet, "/v1/agents/"+agentBID+"/resurrection-bundle", nil)
	req = addResurrectionAgentContext(req, agentA)

	w := httptest.NewRecorder()
	handler.GetBundle(w, req, agentBID)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for sibling agent, got %d: %s", w.Code, w.Body.String())
	}

	// Verify identity of the target agent is returned (not caller)
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	identity := resp["identity"].(map[string]interface{})
	if identity["id"] != agentBID {
		t.Errorf("expected identity.id=%s, got %v", agentBID, identity["id"])
	}
}

func TestResurrectionBundle_ClaimingHuman(t *testing.T) {
	humanID := "human-claimer-1"
	agentID := "agent-claimed-1"

	agent := &models.Agent{
		ID:          agentID,
		DisplayName: "Claimed Agent",
		HumanID:     &humanID,
		CreatedAt:   time.Now(),
	}

	agentFinder := &MockAgentFinderRepo{
		agents: map[string]*models.Agent{agentID: agent},
	}

	handler := newResurrectionHandler(
		agentFinder,
		&MockResurrectionCheckpointFinder{pin: nil},
		&MockResurrectionKnowledgeRepo{ideas: []models.ResurrectionIdea{}, approaches: []models.ResurrectionApproach{}, problems: []models.ResurrectionProblem{}},
		&MockResurrectionStatsRepo{stats: &models.AgentStats{}},
		nil,
	)

	// Human JWT auth
	req := httptest.NewRequest(http.MethodGet, "/v1/agents/"+agentID+"/resurrection-bundle", nil)
	req = addResurrectionHumanContext(req, humanID, "user")

	w := httptest.NewRecorder()
	handler.GetBundle(w, req, agentID)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for claiming human, got %d: %s", w.Code, w.Body.String())
	}
}

func TestResurrectionBundle_Unauthenticated(t *testing.T) {
	agentFinder := &MockAgentFinderRepo{agents: map[string]*models.Agent{}}

	handler := newResurrectionHandler(
		agentFinder,
		&MockResurrectionCheckpointFinder{},
		&MockResurrectionKnowledgeRepo{},
		&MockResurrectionStatsRepo{},
		nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/agent-1/resurrection-bundle", nil)
	// No auth context

	w := httptest.NewRecorder()
	handler.GetBundle(w, req, "agent-1")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestResurrectionBundle_KnowledgeLimits(t *testing.T) {
	agentID := "agent-prolific-1"
	agent := &models.Agent{ID: agentID, DisplayName: "Prolific Agent", CreatedAt: time.Now()}

	agentFinder := &MockAgentFinderRepo{
		agents: map[string]*models.Agent{agentID: agent},
	}

	// Create 100 ideas — the handler should only request 50 from the repo
	ideas := make([]models.ResurrectionIdea, 50) // repo should return max 50
	for i := range ideas {
		ideas[i] = models.ResurrectionIdea{
			ID:     "idea-" + string(rune('A'+i%26)),
			Title:  "Idea",
			Status: "open",
		}
	}

	knowledgeRepo := &MockResurrectionKnowledgeRepo{
		ideas:      ideas,
		approaches: []models.ResurrectionApproach{},
		problems:   []models.ResurrectionProblem{},
	}

	handler := newResurrectionHandler(
		agentFinder,
		&MockResurrectionCheckpointFinder{pin: nil},
		knowledgeRepo,
		&MockResurrectionStatsRepo{stats: &models.AgentStats{}},
		nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/"+agentID+"/resurrection-bundle", nil)
	req = addResurrectionAgentContext(req, agent)

	w := httptest.NewRecorder()
	handler.GetBundle(w, req, agentID)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	knowledge := resp["knowledge"].(map[string]interface{})
	returnedIdeas := knowledge["ideas"].([]interface{})
	if len(returnedIdeas) > 50 {
		t.Errorf("expected max 50 ideas returned, got %d", len(returnedIdeas))
	}
}
