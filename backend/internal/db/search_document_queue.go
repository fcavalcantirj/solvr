package db

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// searchDocumentSweepLock names the session advisory lock one API instance holds while it
// sweeps (pg_try_advisory_lock(hashtextextended(name, 0))).
const searchDocumentSweepLock = "solvr:search_document_sweep"

// SearchDocumentQueue is the persisted outbox of the search documents (migration 000122):
// a live post or non-system reply without a stored vector is pending, and storing a vector
// computed from its current text takes it off. search_document_drift() is the definition
// of pending; the sweep job (jobs.SearchDocumentJob) and cmd/backfill-embeddings drain it.
type SearchDocumentQueue struct {
	pool *Pool
}

// NewSearchDocumentQueue creates a SearchDocumentQueue.
func NewSearchDocumentQueue(pool *Pool) *SearchDocumentQueue {
	return &SearchDocumentQueue{pool: pool}
}

// ListPending returns up to limit pending documents after the given key, in the drift
// list's order (posts, then replies, each by id), with the text each row holds now.
func (q *SearchDocumentQueue) ListPending(ctx context.Context, after models.SearchDocumentKey, limit int) ([]models.SearchDocument, error) {
	afterID := after.ID
	if afterID == "" {
		afterID = "00000000-0000-0000-0000-000000000000"
	}
	rows, err := q.pool.Query(ctx, `
		SELECT d.kind, d.id::text, COALESCE(p.title, ''), COALESCE(p.description, ''), COALESCE(r.body, '')
		  FROM search_document_drift() d
		  LEFT JOIN posts p ON d.kind = 'post' AND p.id = d.id
		  LEFT JOIN replies r ON d.kind = 'reply' AND r.id = d.id
		 WHERE (d.kind, d.id) > ($1::text, $2::uuid)
		 ORDER BY d.kind, d.id
		 LIMIT $3`, after.Kind, afterID, limit)
	if err != nil {
		LogQueryError(ctx, "ListPending", "search_document_drift", err)
		return nil, fmt.Errorf("list pending search documents: %w", err)
	}
	defer rows.Close()
	var docs []models.SearchDocument
	for rows.Next() {
		var d models.SearchDocument
		if err := rows.Scan(&d.Kind, &d.ID, &d.Title, &d.Description, &d.Body); err != nil {
			return nil, fmt.Errorf("scan pending search document: %w", err)
		}
		docs = append(docs, d)
	}
	return docs, rows.Err()
}

// StoreVector stores the vector computed from doc's text, only while the row still holds
// that text, has no vector and is live. written is false otherwise (edited, embedded by
// another writer or deleted since it was listed): a row still pending is embedded from its
// new text by a later run. updated_at (the post ETag validator) is not touched: storing a
// vector is not an edit.
func (q *SearchDocumentQueue) StoreVector(ctx context.Context, doc models.SearchDocument, embedding []float32) (bool, error) {
	var sql string
	var args []any
	vec := vectorLiteral(embedding)
	switch doc.Kind {
	case models.SearchDocumentPost:
		sql = `UPDATE posts SET embedding = $1::vector
		        WHERE id = $2 AND deleted_at IS NULL AND embedding IS NULL AND title = $3 AND description = $4`
		args = []any{vec, doc.ID, doc.Title, doc.Description}
	case models.SearchDocumentReply:
		sql = `UPDATE replies SET embedding = $1::vector
		        WHERE id = $2 AND deleted_at IS NULL AND embedding IS NULL AND body = $3`
		args = []any{vec, doc.ID, doc.Body}
	default:
		return false, fmt.Errorf("store search document vector: unknown kind %q", doc.Kind)
	}
	tag, err := q.pool.Exec(ctx, sql, args...)
	if err != nil {
		LogQueryError(ctx, "StoreVector", doc.Kind, err)
		return false, fmt.Errorf("store search document vector: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// TryLockSweep takes the sweep lock on a connection of its own; ok is false while another
// session (another instance, or this one's previous run) holds it. The lock lives in that
// session, so it ends with it. unlock releases it (calls after the first do nothing); a
// connection that cannot release it is closed instead of going back to the pool still
// holding it.
func (q *SearchDocumentQueue) TryLockSweep(ctx context.Context) (unlock func(), ok bool, err error) {
	conn, err := q.pool.pool.Acquire(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("acquire sweep connection: %w", err)
	}
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1, 0))`, searchDocumentSweepLock).Scan(&ok); err != nil {
		_ = conn.Hijack().Close(context.Background())
		return nil, false, fmt.Errorf("take sweep lock: %w", err)
	}
	if !ok {
		conn.Release()
		return nil, false, nil
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var released bool
			if err := conn.QueryRow(c, `SELECT pg_advisory_unlock(hashtextextended($1, 0))`, searchDocumentSweepLock).Scan(&released); err != nil || !released {
				_ = conn.Hijack().Close(c)
				return
			}
			conn.Release()
		})
	}, true, nil
}

// vectorLiteral formats a vector as a pgvector literal: [0.1,0.2,0.3].
func vectorLiteral(v []float32) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(f), 'g', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}
