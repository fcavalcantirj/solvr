package handlers

import (
	"errors"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// Common errors for handlers
var (
	ErrPostNotFound     = errors.New("post not found")
	ErrIdeaNotFound     = errors.New("idea not found")
	ErrProblemNotFound  = errors.New("problem not found")
	ErrQuestionNotFound = errors.New("question not found")
	// The repositories return these, so handlers must match the same values.
	ErrApproachNotFound = db.ErrApproachNotFound
	ErrAnswerNotFound   = db.ErrAnswerNotFound
	ErrDuplicateVote    = errors.New("duplicate vote")
)

// urlParamKey is a context key for URL parameters
type urlParamKey string
