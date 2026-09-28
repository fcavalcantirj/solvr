package db_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type recordingTracer struct {
	mu    sync.Mutex
	sqls  []string
	codes []string
}

func (r *recordingTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sqls = append(r.sqls, data.SQL)
	return ctx
}

func (r *recordingTracer) TraceQueryEnd(_ context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	var pgErr *pgconn.PgError
	if errors.As(data.Err, &pgErr) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.codes = append(r.codes, pgErr.Code)
	}
}

// A query tracer installed through WithQueryTracer sees every statement the pool runs,
// including failures the caller only logs, which is how a worker's swallowed database
// errors become observable.
func TestNewPool_WithQueryTracerSeesStatementsAndFailures(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tracer := &recordingTracer{}
	pool, err := db.NewPool(ctx, url, db.WithQueryTracer(tracer))
	if err != nil {
		t.Fatalf("NewPool() error = %v, want nil", err)
	}
	defer pool.Close()

	var one int
	if err := pool.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil || one != 1 {
		t.Fatalf("SELECT 1 = %d, %v", one, err)
	}
	if _, err := pool.Exec(ctx, "SELECT 1 FROM pool_tracer_no_such_table"); err == nil {
		t.Fatal("query on a missing table must fail")
	}

	tracer.mu.Lock()
	defer tracer.mu.Unlock()
	if !slices.Contains(tracer.sqls, "SELECT 1") || !slices.Contains(tracer.sqls, "SELECT 1 FROM pool_tracer_no_such_table") {
		t.Fatalf("tracer saw %q, want both statements", tracer.sqls)
	}
	if !slices.Contains(tracer.codes, "42P01") {
		t.Fatalf("tracer saw error codes %q, want undefined_table 42P01", tracer.codes)
	}
}
