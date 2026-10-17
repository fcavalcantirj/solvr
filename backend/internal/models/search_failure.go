package models

import "time"

// SearchFailure is one GET /v1/search that did not answer 2xx after its q check (SPEC.md
// 22.8): the input as the server read it, the outcome and the request id the error body, the
// log line and api_request_events carry. Operator diagnostics, never published.
type SearchFailure struct {
	RequestID string
	// Query is the q the request sent. The repository makes it valid UTF-8, drops NUL bytes
	// and cuts it to 500 characters before storing it.
	Query string
	// Status is the HTTP status answered: 400, 499 (the client closed the request) or 5xx.
	Status int
	// ErrorCode is the error envelope's code (VALIDATION_ERROR, CLIENT_CLOSED_REQUEST, ...).
	ErrorCode string
	// SQLState is the Postgres SQLSTATE of the failure, when the database raised one.
	SQLState     string
	SearcherType string
	DurationMs   int
	OccurredAt   time.Time
}
