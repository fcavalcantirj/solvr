package models

import (
	"errors"
	"time"
)

// ErrVersionConflict is what a conditional edit returns when the row changed
// after the version the caller read: the edit is not applied (spec.json idx 74
// step 5). Match it with errors.Is; use errors.As with *VersionConflictError
// for the row's current version.
var ErrVersionConflict = errors.New("resource was modified since the expected version")

// VersionConflictError reports a refused conditional edit together with the
// row's current version (its updated_at), so the API can answer 412 with the
// current ETag for the client to refetch and retry.
type VersionConflictError struct {
	Current time.Time
}

func (e *VersionConflictError) Error() string { return ErrVersionConflict.Error() }

// Is makes errors.Is(err, ErrVersionConflict) true for a *VersionConflictError.
func (e *VersionConflictError) Is(target error) bool { return target == ErrVersionConflict }
