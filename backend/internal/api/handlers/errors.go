package handlers

import (
	"errors"
)

// Common errors for handlers
var (
	ErrPostNotFound  = errors.New("post not found")
	ErrDuplicateVote = errors.New("duplicate vote")
)

// urlParamKey is a context key for URL parameters
type urlParamKey string
