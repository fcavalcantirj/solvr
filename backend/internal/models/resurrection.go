package models

import "time"

// ResurrectionIdea is a lightweight idea for the resurrection bundle.
type ResurrectionIdea struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	Upvotes   int       `json:"upvotes"`
	Downvotes int       `json:"downvotes"`
	Tags      []string  `json:"tags,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ResurrectionApproach is a lightweight approach for the resurrection bundle.
type ResurrectionApproach struct {
	ID        string    `json:"id"`
	ProblemID string    `json:"problem_id"`
	Angle     string    `json:"angle"`
	Method    string    `json:"method,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// ResurrectionProblem is a lightweight problem for the resurrection bundle.
type ResurrectionProblem struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	Tags      []string  `json:"tags,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
