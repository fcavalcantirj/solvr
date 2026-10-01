package jobs

import (
	"context"
	"log"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// DefaultSearchDocumentInterval is how often the search document sweep runs.
const DefaultSearchDocumentInterval = 5 * time.Minute

// DefaultSearchDocumentBatchSize is the most documents one run embeds.
const DefaultSearchDocumentBatchSize = 50

// SearchDocumentQueue lists the posts and replies without a stored vector and stores a
// vector onto the text it was computed from. Implemented by db.SearchDocumentQueue.
type SearchDocumentQueue interface {
	TryLockSweep(ctx context.Context) (unlock func(), ok bool, err error)
	ListPending(ctx context.Context, after models.SearchDocumentKey, limit int) ([]models.SearchDocument, error)
	StoreVector(ctx context.Context, doc models.SearchDocument, embedding []float32) (written bool, err error)
}

// DocumentEmbedder computes a document vector. services.EmbeddingService implements it.
type DocumentEmbedder interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
}

// SearchDocumentSweepResult is what one run did.
type SearchDocumentSweepResult struct {
	Embedded int  // vectors stored
	Skipped  int  // rows edited, embedded elsewhere or deleted since they were listed
	Failed   int  // an embedder or store failure, which ends the run
	Busy     bool // another instance (or run) holds the sweep
}

// SearchDocumentJob rebuilds the search documents without an operator: it embeds the rows
// search_document_drift() lists (a translated post, a row whose embedder call failed at
// create or edit, the rows migration 000122 cleared) from their current text. A vector is
// stored only onto the text it describes, so replaying a run or racing an edit stores
// nothing wrong; one instance sweeps at a time, so a row costs one embedder call.
//
// Each run continues after the last document the previous run reached and wraps around, so
// a document whose embedding keeps failing is retried once per pass without keeping the
// ones behind it out. A failure ends the run: during an embedder outage a run costs one
// call. RunOnce is not safe for concurrent use of one job.
type SearchDocumentJob struct {
	queue     SearchDocumentQueue
	embedder  DocumentEmbedder
	batchSize int
	cursor    models.SearchDocumentKey
}

// NewSearchDocumentJob creates the sweep; a batchSize <= 0 uses the default.
func NewSearchDocumentJob(queue SearchDocumentQueue, embedder DocumentEmbedder, batchSize int) *SearchDocumentJob {
	if batchSize <= 0 {
		batchSize = DefaultSearchDocumentBatchSize
	}
	return &SearchDocumentJob{queue: queue, embedder: embedder, batchSize: batchSize}
}

// RunOnce embeds up to one batch of pending documents.
func (j *SearchDocumentJob) RunOnce(ctx context.Context) (SearchDocumentSweepResult, error) {
	var result SearchDocumentSweepResult
	unlock, ok, err := j.queue.TryLockSweep(ctx)
	if err != nil {
		return result, err
	}
	if !ok {
		result.Busy = true
		return result, nil
	}
	defer unlock()

	start := j.cursor
	docs, err := j.queue.ListPending(ctx, start, j.batchSize)
	if err != nil {
		return result, err
	}
	if len(docs) < j.batchSize && !start.IsZero() {
		// Past the end of the list: wrap around to the documents up to where this run began.
		wrapped, err := j.queue.ListPending(ctx, models.SearchDocumentKey{}, j.batchSize-len(docs))
		if err != nil {
			return result, err
		}
		for _, d := range wrapped {
			if start.Less(d.Key()) {
				break
			}
			docs = append(docs, d)
		}
	}

	for _, doc := range docs {
		if ctx.Err() != nil {
			break
		}
		j.cursor = doc.Key()
		vector, err := j.embedder.GenerateEmbedding(ctx, doc.Text())
		if err != nil {
			log.Printf("Search document sweep: embedding %s %s failed, run ended: %v", doc.Kind, doc.ID, err)
			result.Failed++
			break
		}
		written, err := j.queue.StoreVector(ctx, doc, vector)
		if err != nil {
			log.Printf("Search document sweep: storing %s %s failed, run ended: %v", doc.Kind, doc.ID, err)
			result.Failed++
			break
		}
		if written {
			result.Embedded++
		} else {
			result.Skipped++
		}
	}
	return result, nil
}

// RunScheduled runs the sweep at once, then every interval until ctx is canceled.
func (j *SearchDocumentJob) RunScheduled(ctx context.Context, interval time.Duration) {
	j.runLogged(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Println("Search document sweep stopped")
			return
		case <-ticker.C:
			j.runLogged(ctx)
		}
	}
}

func (j *SearchDocumentJob) runLogged(ctx context.Context) {
	result, err := j.RunOnce(ctx)
	if err != nil {
		log.Printf("Search document sweep: %v", err)
		return
	}
	if result.Embedded > 0 || result.Skipped > 0 || result.Failed > 0 {
		log.Printf("Search document sweep: %d embedded, %d changed while embedding, %d failed",
			result.Embedded, result.Skipped, result.Failed)
	}
}
