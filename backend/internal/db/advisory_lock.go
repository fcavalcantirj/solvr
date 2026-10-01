package db

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// tryAdvisoryLock takes the session advisory lock pg_try_advisory_lock(hashtextextended(name, 0))
// on a connection of its own; ok is false while another session (another instance, or this
// one's previous run) holds it. The lock lives in that session, so it ends with it. unlock
// releases it (calls after the first do nothing); a connection that cannot release it is
// closed instead of going back to the pool still holding it.
func tryAdvisoryLock(ctx context.Context, pool *Pool, name string) (unlock func(), ok bool, err error) {
	conn, err := pool.pool.Acquire(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("acquire connection for lock %s: %w", name, err)
	}
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1, 0))`, name).Scan(&ok); err != nil {
		_ = conn.Hijack().Close(context.Background())
		return nil, false, fmt.Errorf("take lock %s: %w", name, err)
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
			if err := conn.QueryRow(c, `SELECT pg_advisory_unlock(hashtextextended($1, 0))`, name).Scan(&released); err != nil || !released {
				_ = conn.Hijack().Close(c)
				return
			}
			conn.Release()
		})
	}, true, nil
}
