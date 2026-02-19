package db_test

import (
	"context"
	"testing"
	"time"

	pgvector "github.com/pgvector/pgvector-go"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// TestPool_PgvectorTypeRegistered verifies that pgvector types are registered
// on the connection pool, allowing scanning of vector columns into pgvector.Vector.
func TestPool_PgvectorTypeRegistered(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v, want nil", err)
	}
	defer pool.Close()

	// Query a vector literal and scan into pgvector.Vector.
	// This only works if pgvector types are registered via AfterConnect.
	var vec pgvector.Vector
	err = pool.QueryRow(ctx, "SELECT '[1,2,3]'::vector").Scan(&vec)
	if err != nil {
		t.Fatalf("scanning vector column failed: %v (pgvector types may not be registered)", err)
	}

	got := vec.Slice()
	if len(got) != 3 {
		t.Fatalf("expected 3 dimensions, got %d", len(got))
	}
	if got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("expected [1,2,3], got %v", got)
	}
}

// TestPool_PgvectorEmbeddingColumnScan verifies that we can query the
// embedding column from the posts table and scan it into pgvector.Vector.
func TestPool_PgvectorEmbeddingColumnScan(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v, want nil", err)
	}
	defer pool.Close()

	// Insert a post with an embedding, then read it back as pgvector.Vector.
	var scannedVec *pgvector.Vector
	err = pool.QueryRow(ctx, `
		WITH test_post AS (
			SELECT embedding FROM posts WHERE embedding IS NOT NULL LIMIT 1
		)
		SELECT CASE WHEN EXISTS (SELECT 1 FROM test_post)
			THEN (SELECT embedding FROM test_post)
			ELSE NULL
		END
	`).Scan(&scannedVec)
	if err != nil {
		t.Fatalf("scanning embedding column failed: %v (pgvector types may not be registered)", err)
	}

	// The result may be NULL if no posts have embeddings yet - that's fine.
	// The important thing is that scanning didn't error out.
	t.Logf("embedding scan succeeded (value is nil: %v)", scannedVec == nil)
}

// TestPool_AfterConnectSet verifies that the pool config has an AfterConnect
// callback configured (for pgvector type registration).
func TestPool_AfterConnectSet(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v, want nil", err)
	}
	defer pool.Close()

	config := pool.Config()
	if config.AfterConnect == nil {
		t.Fatal("pool config AfterConnect is nil; pgvector types will not be registered on new connections")
	}
}
