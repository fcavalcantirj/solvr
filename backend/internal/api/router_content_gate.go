package api

import (
	"github.com/fcavalcantirj/solvr/internal/contentgate"
	"github.com/fcavalcantirj/solvr/internal/db"
)

// contentGated is a create handler that runs the anti-abuse content checks (W1).
type contentGated interface {
	SetContentGate(*contentgate.Gate)
}

// wireContentGate gives every create handler the same gate, reading the authoritative
// content tables. Without a database there is nothing to compare against.
func wireContentGate(pool *db.Pool, targets ...contentGated) {
	if pool == nil {
		return
	}
	gate := contentgate.New(db.NewContentDuplicateRepository(pool))
	for _, t := range targets {
		t.SetContentGate(gate)
	}
}
