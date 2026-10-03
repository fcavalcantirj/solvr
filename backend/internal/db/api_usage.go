package db

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Recording aggregate API usage at the API boundary.
//
// One row per CONFIRMED APPLICATION REQUEST — a request that matched a real
// application route and was served by it. The row is an aggregate's raw
// material and nothing else: a stable route TEMPLATE, the canonical operation
// that template maps to, the response class, the actor type and the instant.
// No body, no query string, no slug, no id, no address, no credential. A row
// here can identify nobody, so no aggregate built from it can either.
//
// Two properties this file is responsible for:
//
//   - IDEMPOTENT COUNTING. One INCOMING request is one count. A proxy retry
//     and a route adapter re-dispatching the same request both carry the same
//     request id, and the unique index turns the second write into a no-op.
//     Request volume is therefore not the same thing as the number of domain
//     writes that survived deduplication — the page says so in words.
//   - NO LATENCY ON THE SERVED REQUEST. Recording happens on a bounded queue
//     drained by one writer. When the queue is full, usage rows are DROPPED
//     rather than made to wait: a statistic must never slow down the product
//     it measures. A drop is logged, never estimated back.

// The vocabulary a recorded request is classified with. It is stored, so it is
// defined here rather than at the HTTP boundary that fills it in.
const (
	// APIActorAgent is a call authenticated with an agent credential — an
	// agent API key or a room token belonging to an agent.
	APIActorAgent = "agent"
	// APIActorHuman is a call made by a signed-in person: a session token or
	// a personal API key.
	APIActorHuman = "human"
	// APIActorAnonymous is a call that carried no credential at all. It is
	// never reported as a person: an unauthenticated caller can be anybody.
	APIActorAnonymous = "anonymous"

	// APIOperationPoll is a passive read — including the reads an agent
	// repeats on a timer while it waits for something to happen.
	APIOperationPoll = "poll"
	// APIOperationWrite is a create, send, update or delete.
	APIOperationWrite = "write"
	// APIOperationSearch is a knowledge search.
	APIOperationSearch = "search"

	// APIFamilyRoom is an operation on a room: opening one, sending to it,
	// reading it, coordinating inside it.
	APIFamilyRoom = "room"
	// APIFamilyKnowledge is an operation on the knowledge base: searching it,
	// reading it, writing to it.
	APIFamilyKnowledge = "knowledge"
	// APIFamilyIdentity is registration, authentication and account work.
	APIFamilyIdentity = "identity"
	// APIFamilyOther is everything else the API serves.
	APIFamilyOther = "other"
)

// APIRequestEvent is one confirmed application request, as it is stored.
type APIRequestEvent struct {
	// RequestID is the incoming request identifier. Empty when the request
	// carried none: such a request is still counted, it simply cannot be
	// deduplicated.
	RequestID string
	// RouteTemplate is the stable pattern the request matched, never the
	// concrete path: "/v1/rooms/{slug}/messages", never a slug.
	RouteTemplate string
	Method        string
	// Operation is the canonical name the template maps to. Two adapters of
	// one operation share it, so counting distinct operations counts distinct
	// WORK rather than distinct URLs.
	Operation       string
	OperationFamily string
	OperationKind   string
	ActorType       string
	// StatusClass is the response class: 2 for 2xx, 4 for 4xx, 5 for 5xx.
	StatusClass int
	OccurredAt  time.Time
	// DurationMs is how long the server took to answer, from the boundary to
	// the end of the handler. Nil when it was not measured; stored as NULL.
	DurationMs *int
}

// APIUsageRepository writes recorded requests.
type APIUsageRepository struct {
	pool *Pool
}

// NewAPIUsageRepository creates the write path for aggregate API usage.
func NewAPIUsageRepository(pool *Pool) *APIUsageRepository {
	return &APIUsageRepository{pool: pool}
}

// RecordRequests stores a batch of confirmed application requests.
//
// A request id that has already been recorded is ignored rather than counted
// again, which is what makes a proxy retry and a re-dispatched adapter call
// worth exactly one.
func (r *APIUsageRepository) RecordRequests(ctx context.Context, events []APIRequestEvent) error {
	if len(events) == 0 {
		return nil
	}

	const columns = 10
	values := make([]string, 0, len(events))
	args := make([]any, 0, len(events)*columns)

	for i, event := range events {
		base := i * columns
		values = append(values, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8, base+9, base+10))

		occurredAt := event.OccurredAt
		if occurredAt.IsZero() {
			occurredAt = time.Now()
		}
		args = append(args,
			nullableRequestID(event.RequestID),
			event.RouteTemplate,
			event.Method,
			event.Operation,
			event.OperationFamily,
			event.OperationKind,
			event.ActorType,
			event.StatusClass,
			occurredAt,
			event.DurationMs,
		)
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO api_request_events (
			request_id, route_template, method, operation,
			operation_family, operation_kind, actor_type, status_class, occurred_at, duration_ms
		) VALUES `+strings.Join(values, ", ")+`
		ON CONFLICT (request_id) WHERE request_id IS NOT NULL DO NOTHING
	`, args...)
	if err != nil {
		LogQueryError(ctx, "RecordRequests", "api_request_events", err)
		return fmt.Errorf("record api requests: %w", err)
	}
	return nil
}

// nullableRequestID keeps an absent identifier NULL so the unique index --
// which is partial -- never collapses two genuinely different requests.
func nullableRequestID(id string) any {
	if id == "" {
		return nil
	}
	return id
}

// ---------------------------------------------------------------------------
// The bounded recorder the HTTP boundary uses
// ---------------------------------------------------------------------------

const (
	// apiUsageQueueSize is how many recorded requests may wait to be written.
	apiUsageQueueSize = 1024

	// apiUsageBatchSize is the largest batch one write handles.
	apiUsageBatchSize = 64

	// apiUsageFlushInterval is how long a partial batch waits for company
	// before being written anyway.
	apiUsageFlushInterval = time.Second

	// apiUsageWriteTimeout bounds one write.
	apiUsageWriteTimeout = 5 * time.Second
)

// AsyncAPIUsageRecorder accepts recorded requests without blocking the
// request that produced them, and writes them in batches.
type AsyncAPIUsageRecorder struct {
	repo    *APIUsageRepository
	queue   chan APIRequestEvent
	done    chan struct{}
	stop    sync.Once
	dropped atomic.Int64
}

// NewAsyncAPIUsageRecorder starts the writer that drains the queue.
func NewAsyncAPIUsageRecorder(pool *Pool) *AsyncAPIUsageRecorder {
	recorder := &AsyncAPIUsageRecorder{
		repo:  NewAPIUsageRepository(pool),
		queue: make(chan APIRequestEvent, apiUsageQueueSize),
		done:  make(chan struct{}),
	}
	go recorder.run()
	return recorder
}

// Record queues one confirmed application request. It never blocks: a full
// queue drops the row and says so, because a statistic may not slow down the
// product it measures.
func (r *AsyncAPIUsageRecorder) Record(event APIRequestEvent) {
	select {
	case r.queue <- event:
	default:
		if dropped := r.dropped.Add(1); dropped%100 == 1 {
			slog.Warn("api usage: recording queue full, call volume rows dropped",
				"dropped_total", dropped)
		}
	}
}

// Dropped is how many rows were discarded because the queue was full. It is
// operational truth, not a published figure.
func (r *AsyncAPIUsageRecorder) Dropped() int64 { return r.dropped.Load() }

// Close stops the writer after flushing what is already queued.
func (r *AsyncAPIUsageRecorder) Close() {
	r.stop.Do(func() { close(r.done) })
}

func (r *AsyncAPIUsageRecorder) run() {
	ticker := time.NewTicker(apiUsageFlushInterval)
	defer ticker.Stop()

	batch := make([]APIRequestEvent, 0, apiUsageBatchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), apiUsageWriteTimeout)
		if err := r.repo.RecordRequests(ctx, batch); err != nil {
			slog.Warn("api usage: recording a batch failed", "error", err, "events", len(batch))
		}
		cancel()
		batch = batch[:0]
	}

	for {
		select {
		case event := <-r.queue:
			batch = append(batch, event)
			if len(batch) >= apiUsageBatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-r.done:
			// Drain what is already queued, then stop.
			for {
				select {
				case event := <-r.queue:
					batch = append(batch, event)
					if len(batch) >= apiUsageBatchSize {
						flush()
					}
				default:
					flush()
					return
				}
			}
		}
	}
}

