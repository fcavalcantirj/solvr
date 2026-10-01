package jobs_test

import (
	"context"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/jobs"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// textEmbedder returns a one-hot vector at hash(text) % 1024, so a stored vector names the
// text it was computed from. gate, when set, holds the first call until it is closed.
type textEmbedder struct {
	mu      sync.Mutex
	calls   map[string]int
	gate    chan struct{}
	entered chan struct{}
	during  func(text string)
}

func newTextEmbedder() *textEmbedder { return &textEmbedder{calls: map[string]int{}} }

func (e *textEmbedder) GenerateEmbedding(_ context.Context, text string) ([]float32, error) {
	e.mu.Lock()
	e.calls[text]++
	gate, entered, during := e.gate, e.entered, e.during
	e.gate, e.entered = nil, nil
	e.mu.Unlock()
	if entered != nil {
		close(entered)
	}
	if gate != nil {
		<-gate
	}
	if during != nil {
		during(text)
	}
	v := make([]float32, 1024)
	v[oneHotAxis(text)] = 1
	return v, nil
}

func (e *textEmbedder) total() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	n := 0
	for _, c := range e.calls {
		n += c
	}
	return n
}

func oneHotAxis(text string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(text))
	return int(h.Sum32() % 1024)
}

func oneHot(text string) string {
	parts := make([]string, 1024)
	for i := range parts {
		parts[i] = "0"
	}
	parts[oneHotAxis(text)] = "1"
	return "[" + strings.Join(parts, ",") + "]"
}

func searchVector(ctx context.Context, t *testing.T, pool *db.Pool, table, id string) string {
	t.Helper()
	var vec *string
	if err := pool.QueryRow(ctx, `SELECT embedding::text FROM `+table+` WHERE id = $1`, id).Scan(&vec); err != nil {
		t.Fatalf("read %s vector: %v", table, err)
	}
	if vec == nil {
		return ""
	}
	return *vec
}

func searchDrift(ctx context.Context, t *testing.T, pool *db.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM search_document_drift()`).Scan(&n); err != nil {
		t.Fatalf("drift: %v", err)
	}
	return n
}

// The rows migration 000122 and its triggers leave without a vector (a translated post, a
// row whose embedder call failed, a reply) regain one from their current text with no
// operator run, once however many API instances sweep, and a replay embeds nothing.
func TestSearchDocumentJob_ClearedVectorsComeBackOnceAcrossInstances(t *testing.T) {
	url := newMigratedScratchURL(t, "solvr_search_docs_")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	open := func() *db.Pool {
		t.Helper()
		pool, err := db.NewPool(ctx, url)
		if err != nil {
			t.Fatalf("pool: %v", err)
		}
		t.Cleanup(pool.Close)
		return pool
	}
	poolA, poolB := open(), open()

	agent := fmt.Sprintf("sweep_agent_%d", time.Now().UnixNano())
	if _, err := poolA.Exec(ctx, `INSERT INTO agents (id, display_name) VALUES ($1, 'Sweep Agent')`, agent); err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	posts := db.NewPostRepository(poolA)
	create := func(title string, vec *string) string {
		t.Helper()
		p, err := posts.Create(ctx, &models.Post{
			Type: models.PostTypeQuestion, Title: title, Description: title + ", described",
			Tags: []string{"sweep"}, PostedByType: models.AuthorTypeAgent, PostedByID: agent,
			Status: models.PostStatusOpen, EmbeddingStr: vec,
		})
		if err != nil {
			t.Fatalf("create post: %v", err)
		}
		return p.ID
	}
	original := oneHot("Pergunta sobre pool de conexoes")
	translated := create("Pergunta sobre pool de conexoes", &original)
	if err := posts.ApplyTranslation(ctx, translated, "A question about connection pools", "The English description"); err != nil {
		t.Fatalf("translate: %v", err)
	}
	failed := create("A post whose embedder call failed", nil)
	kept := oneHot("An embedded post An embedded post, described")
	embedded := create("An embedded post", &kept)
	reply, err := db.NewReplyRepository(poolA).Create(ctx, &models.Reply{PostID: embedded,
		AuthorType: models.AuthorTypeAgent, AuthorID: agent, Body: "A reply the embedder never reached"})
	if err != nil {
		t.Fatalf("create reply: %v", err)
	}
	if got := searchVector(ctx, t, poolA, "posts", translated); got != "" {
		t.Fatalf("precondition: the translation clears the original-language vector (000122)")
	}
	if n := searchDrift(ctx, t, poolA); n != 3 {
		t.Fatalf("precondition: drift = %d, want 3 pending", n)
	}

	embedder := newTextEmbedder()
	entered, gate := make(chan struct{}), make(chan struct{})
	embedder.gate, embedder.entered = gate, entered
	jobA := jobs.NewSearchDocumentJob(db.NewSearchDocumentQueue(poolA), embedder, jobs.DefaultSearchDocumentBatchSize)
	jobB := jobs.NewSearchDocumentJob(db.NewSearchDocumentQueue(poolB), embedder, jobs.DefaultSearchDocumentBatchSize)

	type outcome struct {
		result jobs.SearchDocumentSweepResult
		err    error
	}
	doneA := make(chan outcome, 1)
	var openGate sync.Once
	// A failed check below still lets A finish, so its lock connection goes back before the
	// pools close.
	defer openGate.Do(func() { close(gate) })
	go func() {
		r, err := jobA.RunOnce(ctx)
		doneA <- outcome{r, err}
	}()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatalf("instance A never reached the embedder")
	}
	// A is inside its sweep: B's run on the same database does nothing.
	resultB, err := jobB.RunOnce(ctx)
	if err != nil || resultB != (jobs.SearchDocumentSweepResult{Busy: true}) {
		t.Fatalf("instance B = %+v, %v while A sweeps; want Busy", resultB, err)
	}
	openGate.Do(func() { close(gate) })
	a := <-doneA
	if a.err != nil || a.result != (jobs.SearchDocumentSweepResult{Embedded: 3}) {
		t.Fatalf("instance A = %+v, %v; want 3 embedded", a.result, a.err)
	}
	if n := embedder.total(); n != 3 {
		t.Fatalf("embedder called %d times for 3 pending rows", n)
	}
	for _, c := range []struct{ table, id, text string }{
		{"posts", translated, "A question about connection pools The English description"},
		{"posts", failed, "A post whose embedder call failed A post whose embedder call failed, described"},
		{"posts", embedded, "An embedded post An embedded post, described"},
		{"replies", reply.ID, "A reply the embedder never reached"},
	} {
		if got := searchVector(ctx, t, poolA, c.table, c.id); got != oneHot(c.text) {
			t.Errorf("%s %s does not hold the vector of its current text %q", c.table, c.id, c.text)
		}
	}
	if n := searchDrift(ctx, t, poolA); n != 0 {
		t.Fatalf("drift = %d after the sweep, want 0", n)
	}

	// A replayed sweep, on either instance, embeds nothing and calls no embedder.
	for _, job := range []*jobs.SearchDocumentJob{jobA, jobB} {
		if r, err := job.RunOnce(ctx); err != nil || r != (jobs.SearchDocumentSweepResult{}) {
			t.Fatalf("replay = %+v, %v; want nothing done", r, err)
		}
	}
	if n := embedder.total(); n != 3 {
		t.Fatalf("a replay called the embedder (%d calls)", n)
	}

	// An edit that lands while the sweep embeds the old text: the old text's vector is not
	// stored, and the next run embeds the new text.
	if _, err := poolA.Exec(ctx, `UPDATE posts SET title = 'A retitled post' WHERE id = $1`, embedded); err != nil {
		t.Fatalf("retitle: %v", err)
	}
	embedder.during = func(string) {
		if _, err := poolB.Exec(ctx, `UPDATE posts SET title = 'A post retitled while embedding' WHERE id = $1`, embedded); err != nil {
			t.Errorf("concurrent retitle: %v", err)
		}
	}
	if r, err := jobB.RunOnce(ctx); err != nil || r != (jobs.SearchDocumentSweepResult{Skipped: 1}) {
		t.Fatalf("sweep across an edit = %+v, %v; want 1 skipped", r, err)
	}
	embedder.during = nil
	if got := searchVector(ctx, t, poolA, "posts", embedded); got != "" {
		t.Fatalf("the vector of the text before the edit was stored")
	}
	if r, err := jobA.RunOnce(ctx); err != nil || r != (jobs.SearchDocumentSweepResult{Embedded: 1}) {
		t.Fatalf("next sweep = %+v, %v; want 1 embedded", r, err)
	}
	if got := searchVector(ctx, t, poolA, "posts", embedded); got != oneHot("A post retitled while embedding An embedded post, described") {
		t.Fatalf("the post does not hold the vector of its current text")
	}
	if n := searchDrift(ctx, t, poolA); n != 0 {
		t.Fatalf("drift = %d, want 0", n)
	}
}
