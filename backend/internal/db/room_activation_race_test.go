package db_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// Concurrent writers (two API instances handling two agents' first messages at once) all
// call RecordActivation for the same room. The milestone must still be recorded once.
func TestRecordRoomActivation_ConcurrentCallsRecordOnce(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newRoomStatsFixture(t, ctx, pool)
	events := db.NewRoomEventRepository(pool)

	for attempt := 0; attempt < 5; attempt++ {
		room := f.room(fmt.Sprintf("race%d", attempt), false, nil, false)
		f.message(room, "agent", "first"+f.suffix, nil, 2*time.Minute, false)
		f.message(room, "agent", "second"+f.suffix, nil, time.Minute, false)

		const callers = 8
		var wg sync.WaitGroup
		var mu sync.Mutex
		recorded := 0
		start := make(chan struct{})
		for i := 0; i < callers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				ok, err := events.RecordActivation(ctx, room)
				assert.NoError(t, err)
				if ok {
					mu.Lock()
					recorded++
					mu.Unlock()
				}
			}()
		}
		close(start)
		wg.Wait()

		require.Equal(t, 1, recorded, "attempt %d: exactly one caller reports recording", attempt)
		require.Equal(t, 1, countActivations(t, ctx, pool, room), "attempt %d: one milestone row", attempt)
	}
}
