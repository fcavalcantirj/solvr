package jobs

import (
	"context"
	"log"
	"time"
)

// DefaultTrafficClassifyInterval is how often the behavioural traffic rules judge new rows.
const DefaultTrafficClassifyInterval = 15 * time.Minute

// TrafficClassifyWindow is how far back each run judges. A connection start can't be judged
// at ingest: its skill fetch arrives 30-59 s later and a copy may come later still, so a run
// reads the last two hours, and the rules read whole clock hours from there (SPEC.md 27.6).
const TrafficClassifyWindow = 2 * time.Hour

// TrafficRules applies the behavioural rules over a window (db.TrafficRules, the same code
// path POST /admin/traffic/reclassify runs).
type TrafficRules interface {
	TryLock(ctx context.Context) (unlock func(), ok bool, err error)
	ApplyRules(ctx context.Context, from, to time.Time) (map[string]int, error)
}

// TrafficClassifyRun is what one run did.
type TrafficClassifyRun struct {
	Rules map[string]int // rows each rule set
	Busy  bool           // another run (another instance, or the operator's) holds the lock
}

// TrafficClassifyJob runs the behavioural traffic rules on live rows (REVIEW-r1 C3): a
// post-sourced start that an agent fetched and nobody copied is flagged within one period,
// with no operator call. One run at a time across instances.
type TrafficClassifyJob struct {
	rules TrafficRules
	now   func() time.Time
}

// NewTrafficClassifyJob creates the job.
func NewTrafficClassifyJob(rules TrafficRules) *TrafficClassifyJob {
	return &TrafficClassifyJob{rules: rules, now: time.Now}
}

// RunOnce judges the trailing window.
func (j *TrafficClassifyJob) RunOnce(ctx context.Context) (TrafficClassifyRun, error) {
	unlock, ok, err := j.rules.TryLock(ctx)
	if err != nil {
		return TrafficClassifyRun{}, err
	}
	if !ok {
		return TrafficClassifyRun{Busy: true}, nil
	}
	defer unlock()
	now := j.now()
	rules, err := j.rules.ApplyRules(ctx, now.Add(-TrafficClassifyWindow), now)
	return TrafficClassifyRun{Rules: rules}, err
}

// RunScheduled runs the job at once, then every interval until ctx is canceled.
func (j *TrafficClassifyJob) RunScheduled(ctx context.Context, interval time.Duration) {
	j.runLogged(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Println("Traffic classify job stopped")
			return
		case <-ticker.C:
			j.runLogged(ctx)
		}
	}
}

func (j *TrafficClassifyJob) runLogged(ctx context.Context) {
	if _, err := j.RunOnce(ctx); err != nil {
		log.Printf("Traffic classify: %v", err)
	}
}
