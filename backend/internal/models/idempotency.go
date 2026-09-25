package models

import "time"

// Idempotency record states (idx 73 step 4).
const (
	IdempotencyPending   = "pending"
	IdempotencyCompleted = "completed"
)

// IdempotencyScope identifies one Idempotency-Key: a key is private to the
// authenticated actor and to the create operation it was sent to.
type IdempotencyScope struct {
	ActorType string
	ActorID   string
	Operation string
	Key       string
}

// IdempotencyRecord is the stored outcome of a keyed create. A pending record
// is a request still in flight; a completed one holds the response to replay.
type IdempotencyRecord struct {
	RequestHash         string
	Status              string
	ResponseStatus      int
	ResponseContentType string
	ResponseBody        []byte
	CreatedAt           time.Time
}
