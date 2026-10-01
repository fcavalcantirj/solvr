package db

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// The search document sweep (idx 77 slice 8) drains what search_document_drift() lists: the
// live posts and non-system replies without a stored vector. Measured before it (slice 8
// spike, prod-shaped copies): nothing but an operator run of cmd/backfill-embeddings wrote a
// vector onto an existing row, so every translated post (63 of 524 live posts) and every
// row whose embedder call failed stayed out of semantic search until someone ran it.

func pendingKeys(docs []models.SearchDocument) []searchDocument {
	out := make([]searchDocument, 0, len(docs))
	for _, d := range docs {
		out = append(out, searchDocument{d.Kind, d.ID})
	}
	return out
}

func TestSearchDocumentQueue_ListsWhatTheDriftCheckListsWithTheCurrentText(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	author := authorAgent(ctx, t, pool, "queue_"+uuid.NewString()[:8])
	posts := NewPostRepository(pool)
	queue := NewSearchDocumentQueue(pool)
	vec := unitVector(6)

	docs, err := queue.ListPending(ctx, models.SearchDocumentKey{}, 100)
	require.NoError(t, err)
	require.Empty(t, docs)

	embedded := createVectorPost(ctx, t, pool, author, "An embedded post", &vec)
	bare := createVectorPost(ctx, t, pool, author, "A post the embedder never reached", nil)
	gone := createVectorPost(ctx, t, pool, author, "A deleted post without a vector", nil)
	require.NoError(t, posts.Delete(ctx, gone))
	translated := createVectorPost(ctx, t, pool, author, "Pergunta sobre o pool de conexoes", &vec)
	require.NoError(t, posts.ApplyTranslation(ctx, translated, "A question about the connection pool", "Translated description"))
	bareReply := insertSearchReply(t, pool, ctx, embedded, author, "a reply without a vector", nil, false)
	insertSearchReply(t, pool, ctx, embedded, author, "an embedded reply", &vec, false)
	insertSearchReply(t, pool, ctx, embedded, author, "a deleted reply without a vector", nil, true)
	_, err = pool.Exec(ctx, `INSERT INTO replies (post_id, author_type, author_id, body)
		VALUES ($1, 'system', 'moderation', 'a system verdict is not knowledge')`, embedded)
	require.NoError(t, err)

	docs, err = queue.ListPending(ctx, models.SearchDocumentKey{}, 100)
	require.NoError(t, err)
	require.Equal(t, readSearchDocumentDrift(ctx, t, pool), pendingKeys(docs), "the sweep lists exactly the drift check's rows, posts first, in id order")
	text := map[string]string{}
	for _, d := range docs {
		text[d.ID] = d.Text()
	}
	require.Equal(t, "A post the embedder never reached A post the embedder never reached, the description of the post", text[bare])
	require.Equal(t, "A question about the connection pool Translated description", text[translated], "a translated post is embedded from its English text")
	require.Equal(t, "a reply without a vector", text[bareReply])

	// Keyset pages: each page starts after the previous page's last key, and the pages
	// together are the whole list once.
	var paged []models.SearchDocument
	after := models.SearchDocumentKey{}
	for i := 0; i < 5; i++ {
		page, err := queue.ListPending(ctx, after, 1)
		require.NoError(t, err)
		if len(page) == 0 {
			break
		}
		require.Len(t, page, 1)
		paged = append(paged, page...)
		after = page[0].Key()
	}
	require.Equal(t, pendingKeys(docs), pendingKeys(paged))
	page, err := queue.ListPending(ctx, after, 10)
	require.NoError(t, err)
	require.Empty(t, page, "nothing is listed after the last key")
}

func TestSearchDocumentQueue_StoresAVectorOnlyOntoTheTextItDescribes(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	author := authorAgent(ctx, t, pool, "store_"+uuid.NewString()[:8])
	posts := NewPostRepository(pool)
	queue := NewSearchDocumentQueue(pool)
	vector := func(axis int) []float32 {
		v := make([]float32, 1024)
		v[axis] = 1
		return v
	}
	read := func(id string) models.SearchDocument {
		t.Helper()
		docs, err := queue.ListPending(ctx, models.SearchDocumentKey{}, 100)
		require.NoError(t, err)
		for _, d := range docs {
			if d.ID == id {
				return d
			}
		}
		t.Fatalf("%s is not pending", id)
		return models.SearchDocument{}
	}
	stamp := func(table, id string) time.Time {
		t.Helper()
		var at time.Time
		require.NoError(t, pool.QueryRow(ctx, `SELECT updated_at FROM `+table+` WHERE id = $1`, id).Scan(&at))
		return at
	}

	post := createVectorPost(ctx, t, pool, author, "A post the embedder never reached", nil)
	stale := read(post)
	_, err := pool.Exec(ctx, `UPDATE posts SET title = 'Retitled while the sweep embedded' WHERE id = $1`, post)
	require.NoError(t, err)
	written, err := queue.StoreVector(ctx, stale, vector(1))
	require.NoError(t, err)
	require.False(t, written, "a vector computed from the old title is not stored on the new one")
	require.Equal(t, "", storedVector(ctx, t, pool, "posts", post))

	current := read(post)
	before := stamp("posts", post)
	written, err = queue.StoreVector(ctx, current, vector(2))
	require.NoError(t, err)
	require.True(t, written)
	require.Equal(t, unitVector(2), storedVector(ctx, t, pool, "posts", post))
	require.Equal(t, before, stamp("posts", post), "storing a vector is not an edit: updated_at (the ETag) stays")
	written, err = queue.StoreVector(ctx, current, vector(3))
	require.NoError(t, err)
	require.False(t, written, "a replayed write does not replace a stored vector")
	require.Equal(t, unitVector(2), storedVector(ctx, t, pool, "posts", post))

	deleted := createVectorPost(ctx, t, pool, author, "A post deleted while the sweep embedded", nil)
	doc := read(deleted)
	require.NoError(t, posts.Delete(ctx, deleted))
	written, err = queue.StoreVector(ctx, doc, vector(4))
	require.NoError(t, err)
	require.False(t, written, "a deleted post gets no vector")

	reply := insertSearchReply(t, pool, ctx, post, author, "a reply without a vector", nil, false)
	staleReply := read(reply)
	_, err = pool.Exec(ctx, `UPDATE replies SET body = 'a reply edited while the sweep embedded' WHERE id = $1`, reply)
	require.NoError(t, err)
	written, err = queue.StoreVector(ctx, staleReply, vector(5))
	require.NoError(t, err)
	require.False(t, written)
	before = stamp("replies", reply)
	written, err = queue.StoreVector(ctx, read(reply), vector(6))
	require.NoError(t, err)
	require.True(t, written)
	require.Equal(t, unitVector(6), storedVector(ctx, t, pool, "replies", reply))
	require.Equal(t, before, stamp("replies", reply))

	_, err = queue.StoreVector(ctx, models.SearchDocument{Kind: "answer", ID: reply}, vector(7))
	require.Error(t, err, "only posts and replies are search documents")
	require.Empty(t, readSearchDocumentDrift(ctx, t, pool))
}

// Every API instance schedules the sweep; one at a time holds it, so a pending row costs one
// embedder call however many instances run. The lock lives in the holder's database session,
// so an instance that dies mid-sweep does not keep it.
func TestSearchDocumentQueue_OneInstanceSweepsAtATime(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	other, err := NewPool(ctx, pool.Config().ConnString())
	require.NoError(t, err)
	defer other.Close()
	a, b := NewSearchDocumentQueue(pool), NewSearchDocumentQueue(other)
	// Every lock taken is released when the test ends, failed or not: a connection left
	// holding one would keep the pools' Close waiting.
	var held []func()
	defer func() {
		for _, unlock := range held {
			unlock()
		}
	}()
	lock := func(q *SearchDocumentQueue) (func(), bool) {
		t.Helper()
		unlock, ok, err := q.TryLockSweep(ctx)
		require.NoError(t, err)
		if ok {
			held = append(held, unlock)
		}
		return unlock, ok
	}

	unlockA, ok := lock(a)
	require.True(t, ok)
	_, ok = lock(b)
	require.False(t, ok, "a second instance does not sweep while the first does")
	_, ok = lock(a)
	require.False(t, ok, "nor does a second run of the same instance")
	unlockA()

	unlockB, ok := lock(b)
	require.True(t, ok, "released, the lock goes to whoever asks next")
	unlockB()
	unlockB() // a second call does nothing

	// The holder's session ends before its unlock (a crashed instance, a dropped connection).
	unlockA, ok = lock(a)
	require.True(t, ok)
	var terminated bool
	require.NoError(t, other.QueryRow(ctx, `SELECT pg_terminate_backend(l.pid) FROM pg_locks l
		WHERE l.locktype = 'advisory' AND l.granted AND l.database = (SELECT oid FROM pg_database WHERE datname = current_database())`).Scan(&terminated))
	require.True(t, terminated)
	deadline := time.Now().Add(10 * time.Second)
	for {
		unlockB, ok = lock(b)
		if ok {
			break
		}
		require.True(t, time.Now().Before(deadline), "the lock of a dead session was never released")
		time.Sleep(50 * time.Millisecond)
	}
	// The late unlock on the dead connection neither fails nor releases b's lock, and gives
	// the connection back (the pool's Close would wait for it otherwise).
	unlockA()
	_, ok = lock(a)
	require.False(t, ok, "b still holds the sweep")
	unlockB()
	unlockA, ok = lock(a)
	require.True(t, ok)
	unlockA()
}
