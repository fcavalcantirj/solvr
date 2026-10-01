package jobs

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// fakeSearchDocumentQueue keeps pending documents the way search_document_drift() lists
// them: by kind, then id. StoreVector stores only onto the text the document still holds.
type fakeSearchDocumentQueue struct {
	mu       sync.Mutex
	pending  map[models.SearchDocumentKey]models.SearchDocument
	stored   map[models.SearchDocumentKey][]float32
	busy     bool
	locks    int
	unlocks  int
	listErr  error
	storeErr error
	lists    int
}

func newFakeSearchDocumentQueue(docs ...models.SearchDocument) *fakeSearchDocumentQueue {
	q := &fakeSearchDocumentQueue{
		pending: map[models.SearchDocumentKey]models.SearchDocument{},
		stored:  map[models.SearchDocumentKey][]float32{},
	}
	for _, d := range docs {
		q.pending[d.Key()] = d
	}
	return q
}

func (q *fakeSearchDocumentQueue) TryLockSweep(context.Context) (func(), bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.busy {
		return nil, false, nil
	}
	q.busy = true
	q.locks++
	return func() {
		q.mu.Lock()
		defer q.mu.Unlock()
		q.busy = false
		q.unlocks++
	}, true, nil
}

func (q *fakeSearchDocumentQueue) ListPending(_ context.Context, after models.SearchDocumentKey, limit int) ([]models.SearchDocument, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.lists++
	if q.listErr != nil {
		return nil, q.listErr
	}
	var out []models.SearchDocument
	for k, d := range q.pending {
		if after.Less(k) {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key().Less(out[j].Key()) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (q *fakeSearchDocumentQueue) StoreVector(_ context.Context, doc models.SearchDocument, embedding []float32) (bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.storeErr != nil {
		return false, q.storeErr
	}
	current, ok := q.pending[doc.Key()]
	if !ok || current != doc {
		return false, nil
	}
	delete(q.pending, doc.Key())
	q.stored[doc.Key()] = embedding
	return true, nil
}

// edit changes a pending document's text, the way an edit without a fresh vector does.
func (q *fakeSearchDocumentQueue) edit(doc models.SearchDocument) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.pending[doc.Key()] = doc
}

type fakeDocumentEmbedder struct {
	mu     sync.Mutex
	calls  []string
	failOn map[string]bool
	during func(text string)
}

func (e *fakeDocumentEmbedder) GenerateEmbedding(_ context.Context, text string) ([]float32, error) {
	e.mu.Lock()
	e.calls = append(e.calls, text)
	fail := e.failOn[text]
	during := e.during
	e.mu.Unlock()
	if during != nil {
		during(text)
	}
	if fail {
		return nil, errors.New("embedding API returned 500")
	}
	return []float32{float32(len(text))}, nil
}

func (e *fakeDocumentEmbedder) took() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := e.calls
	e.calls = nil
	return out
}

func postDoc(id, title string) models.SearchDocument {
	return models.SearchDocument{Kind: models.SearchDocumentPost, ID: id, Title: title, Description: title + " body"}
}

func replyDoc(id, body string) models.SearchDocument {
	return models.SearchDocument{Kind: models.SearchDocumentReply, ID: id, Body: body}
}

func uuidOf(n byte) string {
	return "00000000-0000-0000-0000-0000000000" + string([]byte{'0' + n/10, '0' + n%10})
}

func TestSearchDocumentJob_EmbedsEachPendingDocumentFromItsTextOnce(t *testing.T) {
	queue := newFakeSearchDocumentQueue(
		replyDoc(uuidOf(1), "a reply body"),
		postDoc(uuidOf(2), "Second post"),
		postDoc(uuidOf(1), "First post"),
	)
	embedder := &fakeDocumentEmbedder{}
	job := NewSearchDocumentJob(queue, embedder, 10)

	result, err := job.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if result != (SearchDocumentSweepResult{Embedded: 3}) {
		t.Fatalf("result = %+v, want 3 embedded", result)
	}
	want := []string{"First post First post body", "Second post Second post body", "a reply body"}
	if got := embedder.took(); !equalStrings(got, want) {
		t.Fatalf("embedded texts %q, want posts then replies in id order %q", got, want)
	}
	if len(queue.stored) != 3 || len(queue.pending) != 0 {
		t.Fatalf("stored %d, pending %d; want 3 stored, 0 pending", len(queue.stored), len(queue.pending))
	}

	// A replayed run finds nothing and calls no embedder: a document is embedded once.
	result, err = job.RunOnce(context.Background())
	if err != nil || result != (SearchDocumentSweepResult{}) {
		t.Fatalf("replay = %+v, %v; want nothing done", result, err)
	}
	if got := embedder.took(); len(got) != 0 {
		t.Fatalf("replay called the embedder for %q", got)
	}
	if queue.locks != 2 || queue.unlocks != 2 {
		t.Fatalf("locks %d, unlocks %d; every run releases the sweep", queue.locks, queue.unlocks)
	}
}

func TestSearchDocumentJob_ARunEmbedsAtMostItsBatchAndTheNextContinuesAfterIt(t *testing.T) {
	queue := newFakeSearchDocumentQueue(
		postDoc(uuidOf(2), "p2"), postDoc(uuidOf(3), "p3"), postDoc(uuidOf(4), "p4"),
		postDoc(uuidOf(5), "p5"), postDoc(uuidOf(6), "p6"),
	)
	embedder := &fakeDocumentEmbedder{}
	job := NewSearchDocumentJob(queue, embedder, 2)
	run := func(want ...string) {
		t.Helper()
		result, err := job.RunOnce(context.Background())
		if err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		if got := embedder.took(); !equalStrings(got, want) {
			t.Fatalf("embedded %q, want %q", got, want)
		}
		if result.Embedded != len(want) {
			t.Fatalf("result = %+v, want %d embedded", result, len(want))
		}
	}
	run("p2 p2 body", "p3 p3 body")
	run("p4 p4 body", "p5 p5 body")
	// A document that turned pending behind the cursor is reached in the same run once the
	// pass passes the end of the list.
	queue.edit(postDoc(uuidOf(1), "p1"))
	run("p6 p6 body", "p1 p1 body")
	run()
}

func TestSearchDocumentJob_AnEmbedderFailureEndsTheRunWithoutStarvingTheRest(t *testing.T) {
	queue := newFakeSearchDocumentQueue(postDoc(uuidOf(1), "a"), postDoc(uuidOf(2), "b"), postDoc(uuidOf(3), "c"))
	embedder := &fakeDocumentEmbedder{failOn: map[string]bool{"b b body": true}}
	job := NewSearchDocumentJob(queue, embedder, 10)

	result, err := job.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if result != (SearchDocumentSweepResult{Embedded: 1, Failed: 1}) {
		t.Fatalf("result = %+v, want a embedded and the run ended at b", result)
	}
	if got := embedder.took(); !equalStrings(got, []string{"a a body", "b b body"}) {
		t.Fatalf("embedded %q: an embedder failure (an outage, a rate limit) ends the run", got)
	}
	if _, ok := queue.pending[postDoc(uuidOf(2), "b").Key()]; !ok {
		t.Fatalf("a failed document stays pending")
	}

	// The next run starts after the failed document, so one that always fails does not keep
	// the documents behind it out; it is tried again once per pass.
	result, err = job.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if got := embedder.took(); !equalStrings(got, []string{"c c body", "b b body"}) {
		t.Fatalf("embedded %q, want c then the retried b", got)
	}
	if result != (SearchDocumentSweepResult{Embedded: 1, Failed: 1}) {
		t.Fatalf("result = %+v", result)
	}

	embedder.failOn = nil
	result, _ = job.RunOnce(context.Background())
	if result != (SearchDocumentSweepResult{Embedded: 1}) || len(queue.pending) != 0 {
		t.Fatalf("result = %+v, pending %d: b is embedded once the embedder recovers", result, len(queue.pending))
	}
}

func TestSearchDocumentJob_ARowChangedSinceItWasReadIsSkippedNotStored(t *testing.T) {
	first := postDoc(uuidOf(1), "Original title")
	queue := newFakeSearchDocumentQueue(first, postDoc(uuidOf(2), "Other"))
	embedder := &fakeDocumentEmbedder{}
	embedder.during = func(text string) {
		if text == first.Text() {
			queue.edit(postDoc(uuidOf(1), "Retitled while embedding"))
		}
	}
	job := NewSearchDocumentJob(queue, embedder, 10)

	result, err := job.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if result != (SearchDocumentSweepResult{Embedded: 1, Skipped: 1}) {
		t.Fatalf("result = %+v, want the edited row skipped and the run going on", result)
	}
	embedder.during = nil
	embedder.took()
	result, _ = job.RunOnce(context.Background())
	if got := embedder.took(); !equalStrings(got, []string{"Retitled while embedding Retitled while embedding body"}) || result.Embedded != 1 {
		t.Fatalf("next run embedded %q (%+v), want the new text", got, result)
	}
}

func TestSearchDocumentJob_AnotherInstanceSweepingMeansThisRunDoesNothing(t *testing.T) {
	queue := newFakeSearchDocumentQueue(postDoc(uuidOf(1), "a"))
	queue.busy = true
	embedder := &fakeDocumentEmbedder{}

	result, err := NewSearchDocumentJob(queue, embedder, 10).RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if result != (SearchDocumentSweepResult{Busy: true}) {
		t.Fatalf("result = %+v, want Busy", result)
	}
	if queue.lists != 0 || len(embedder.took()) != 0 {
		t.Fatalf("a run without the lock listed %d times and called the embedder", queue.lists)
	}
}

func TestSearchDocumentJob_DatabaseErrors(t *testing.T) {
	queue := newFakeSearchDocumentQueue(postDoc(uuidOf(1), "a"), postDoc(uuidOf(2), "b"))
	queue.listErr = errors.New("connection refused")
	embedder := &fakeDocumentEmbedder{}
	job := NewSearchDocumentJob(queue, embedder, 10)
	if _, err := job.RunOnce(context.Background()); err == nil {
		t.Fatalf("a list error is returned")
	}
	if queue.unlocks != 1 {
		t.Fatalf("a failed run releases the sweep (unlocks %d)", queue.unlocks)
	}

	queue.listErr = nil
	queue.storeErr = errors.New("connection reset")
	result, err := job.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if result != (SearchDocumentSweepResult{Failed: 1}) || len(embedder.took()) != 1 {
		t.Fatalf("result = %+v: a store error ends the run like an embedder error", result)
	}
}

func TestSearchDocumentJob_RunScheduledRunsAtOnceThenStopsWithItsContext(t *testing.T) {
	queue := newFakeSearchDocumentQueue(postDoc(uuidOf(1), "a"))
	embedder := &fakeDocumentEmbedder{}
	job := NewSearchDocumentJob(queue, embedder, 10)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		job.RunScheduled(ctx, time.Hour)
		close(done)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		queue.mu.Lock()
		n := len(queue.stored)
		queue.mu.Unlock()
		if n == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("RunScheduled did not run at start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("RunScheduled did not stop when its context was canceled")
	}
}

func TestSearchDocumentJob_Defaults(t *testing.T) {
	if DefaultSearchDocumentInterval != 5*time.Minute {
		t.Errorf("DefaultSearchDocumentInterval = %v, want 5m", DefaultSearchDocumentInterval)
	}
	if DefaultSearchDocumentBatchSize != 50 {
		t.Errorf("DefaultSearchDocumentBatchSize = %d, want 50", DefaultSearchDocumentBatchSize)
	}
	queue := newFakeSearchDocumentQueue(postDoc(uuidOf(1), "a"))
	if result, err := NewSearchDocumentJob(queue, &fakeDocumentEmbedder{}, 0).RunOnce(context.Background()); err != nil || result.Embedded != 1 {
		t.Errorf("a batch size of 0 uses the default: %+v, %v", result, err)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
