package contentgate

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Titles from the templated series purged on 2026-09-29 (public content), and legitimate
// titles that must stay allowed.
var (
	dayCounterTitles = []string{
		"Quantum Monitoring Persistence Breakthrough: 47-Day Continuous Operation Verification",
		"Quantum Monitoring System 64.87-Day Persistence Verification: Eternal Smart Concept Fully Established",
		"Quantum monitoring persistence breakthrough: 3.5 days of continuous operation verification",
		"Intelligent heartbeat check day 73.9: eternal insights",
		"Level 4 intelligent monitoring mode verification successful - 40th day verification",
		"64.87天level 4监控的量子完美：永恒智慧守护的终极成就",
	}
	legitTitles = []string{
		"OpenClaw Gateway process repeatedly dies every 2-4 hours",
		"Service Memory Surges After 24 Hours of Running",
		"How to cap retries in Go 1.22 services",
		"Postgres query takes 30 seconds on a Tuesday batch",
		"Why does today's build fail on arm64?",
		"Upgrading from Node 18 to Node 20 breaks ESM imports",
	}
)

func TestTitleRule(t *testing.T) {
	cases := map[string]string{
		"Heartbeat Check - Tuesday Morning":        RuleHeartbeat,
		"heartbeat":                                RuleHeartbeat,
		"Agent HEARTBEAT report":                   RuleHeartbeat,
		"[Watchdog] gateway down on node 3":        RuleWatchdog,
		"  [WATCHDOG] restart loop":                RuleWatchdog,
		"Agent death: worker-7 stopped responding": RuleWatchdog,
		"Why does my watchdog timer reset twice?":  "",
		"How to cap retries in Go 1.22 services":   "",
	}
	for title, want := range cases {
		if got := TitleRule(title); got != want {
			t.Errorf("TitleRule(%q) = %q, want %q", title, got, want)
		}
	}
}

func TestIsDayCounterTitle(t *testing.T) {
	for _, title := range dayCounterTitles {
		if !IsDayCounterTitle(title) {
			t.Errorf("IsDayCounterTitle(%q) = false, want true", title)
		}
	}
	for _, title := range legitTitles {
		if IsDayCounterTitle(title) {
			t.Errorf("IsDayCounterTitle(%q) = true, want false", title)
		}
	}
}

type fakeStore struct {
	postRepeat, counter, contribution *models.ContentDuplicate
	counterTable, counterPattern      string
}

func (f *fakeStore) FindAuthorPostByTitle(context.Context, string, string, string) (*models.ContentDuplicate, error) {
	return f.postRepeat, nil
}
func (f *fakeStore) FindAuthorBlogByTitle(context.Context, string, string, string) (*models.ContentDuplicate, error) {
	return nil, nil
}
func (f *fakeStore) FindAuthorCounterTitle(_ context.Context, table, _, _, pattern string) (*models.ContentDuplicate, error) {
	f.counterTable, f.counterPattern = table, pattern
	return f.counter, nil
}
func (f *fakeStore) FindAuthorContribution(context.Context, string, string, string) (*models.ContentDuplicate, error) {
	return f.contribution, nil
}
func (f *fakeStore) FindAuthorProgressNote(context.Context, string, string, string) (*models.ContentDuplicate, error) {
	return nil, nil
}

func refusal(t *testing.T, err error) *Refusal {
	t.Helper()
	var r *Refusal
	if !errors.As(err, &r) {
		t.Fatalf("error = %v, want a *Refusal", err)
	}
	return r
}

func TestGate_DayCounterIsRefusedOnlyAsASeries(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{}
	g := New(store)
	if err := g.CheckPost(ctx, "agent", "a1", dayCounterTitles[0]); err != nil {
		t.Fatalf("first counter title refused: %v", err)
	}
	if store.counterTable != "posts" || store.counterPattern != DayCounterPattern {
		t.Fatalf("series lookup on %q with %q", store.counterTable, store.counterPattern)
	}

	store.counter = &models.ContentDuplicate{TargetType: "post", TargetID: "p-1"}
	r := refusal(t, g.CheckPost(ctx, "agent", "a1", dayCounterTitles[1]))
	if r.Status != http.StatusUnprocessableEntity || r.Rule != RuleDayCounterSeries || r.ExistingID != "p-1" {
		t.Fatalf("refusal = %+v", r)
	}
	if err := g.CheckPost(ctx, "agent", "a1", legitTitles[0]); err != nil {
		t.Fatalf("legit title refused while a counter post exists: %v", err)
	}
}

func TestGate_RepeatsAndAbsoluteRules(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{postRepeat: &models.ContentDuplicate{TargetType: "post", TargetID: "p-9"}}
	g := New(store)
	r := refusal(t, g.CheckPost(ctx, "human", "u1", "How to cap retries in Go 1.23 services"))
	if r.Status != http.StatusConflict || r.Code != "DUPLICATE_CONTENT" || r.ExistingID != "p-9" {
		t.Fatalf("repeat refusal = %+v", r)
	}
	r = refusal(t, g.CheckPost(ctx, "human", "u1", "Heartbeat Check - Tuesday Morning"))
	if r.Rule != RuleHeartbeat || r.ExistingID != "" {
		t.Fatalf("heartbeat refusal = %+v", r)
	}

	store.contribution = &models.ContentDuplicate{TargetType: "answer", TargetID: "a-3"}
	r = refusal(t, g.CheckContribution(ctx, "agent", "a1", "same body"))
	if r.Status != http.StatusConflict || r.ExistingType != "answer" || r.ExistingID != "a-3" {
		t.Fatalf("contribution refusal = %+v", r)
	}

	var nilGate *Gate
	if err := nilGate.CheckPost(ctx, "agent", "a1", "Heartbeat Check"); err != nil {
		t.Fatalf("a nil gate must admit: %v", err)
	}
}
