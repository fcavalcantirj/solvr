package handlers

import "errors"

// Common errors for handlers
var (
	ErrPostNotFound     = errors.New("post not found")
	ErrIdeaNotFound     = errors.New("idea not found")
	ErrProblemNotFound  = errors.New("problem not found")
	ErrQuestionNotFound = errors.New("question not found")
	ErrApproachNotFound = errors.New("approach not found")
	ErrAnswerNotFound   = errors.New("answer not found")
	ErrDuplicateVote    = errors.New("duplicate vote")
)

// urlParamKey is a context key for URL parameters
type urlParamKey string
