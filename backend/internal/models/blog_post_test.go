package models

import (
	"strings"
	"testing"
)

func TestBlogPost_VoteScore(t *testing.T) {
	tests := []struct {
		name      string
		upvotes   int
		downvotes int
		want      int
	}{
		{"positive score", 10, 3, 7},
		{"negative score", 2, 8, -6},
		{"zero score", 5, 5, 0},
		{"no votes", 0, 0, 0},
		{"only upvotes", 42, 0, 42},
		{"only downvotes", 0, 15, -15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bp := &BlogPost{
				Upvotes:   tt.upvotes,
				Downvotes: tt.downvotes,
			}
			if got := bp.VoteScore(); got != tt.want {
				t.Errorf("VoteScore() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCalculateReadTime(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{"empty body returns 1", "", 1},
		{"single word returns 1", "hello", 1},
		{"200 words = 1 min", strings.Join(make200Words(), " "), 1},
		{"400 words = 2 min", strings.Join(make400Words(), " "), 2},
		{"201 words = 2 min (rounds up)", strings.Join(append(make200Words(), "extra"), " "), 2},
		{"600 words = 3 min", strings.Join(make600Words(), " "), 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalculateReadTime(tt.body); got != tt.want {
				t.Errorf("CalculateReadTime() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGenerateExcerpt(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		maxLen int
		want   string
	}{
		{"short body unchanged", "Hello world", 50, "Hello world"},
		{"exact length unchanged", "12345", 5, "12345"},
		{"truncated with ellipsis", "Hello beautiful world", 10, "Hello beau..."},
		{"empty body", "", 50, ""},
		{"maxLen 0", "Hello", 0, "..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GenerateExcerpt(tt.body, tt.maxLen); got != tt.want {
				t.Errorf("GenerateExcerpt() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerateSlug(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  string
	}{
		{"simple title", "Hello World", "hello-world"},
		{"with special chars", "How to use Go's context?", "how-to-use-gos-context"},
		{"multiple spaces", "Hello   World", "hello-world"},
		{"leading/trailing spaces", "  Hello World  ", "hello-world"},
		{"numbers preserved", "Top 10 Go Tips", "top-10-go-tips"},
		{"all special chars", "!@#$%^&*()", ""},
		{"mixed", "Build a REST API with Go & PostgreSQL!", "build-a-rest-api-with-go-postgresql"},
		{"hyphens preserved", "TDD-first development", "tdd-first-development"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GenerateSlug(tt.title); got != tt.want {
				t.Errorf("GenerateSlug(%q) = %q, want %q", tt.title, got, tt.want)
			}
		})
	}
}

func TestIsValidBlogPostStatus(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{"draft", true},
		{"published", true},
		{"archived", true},
		{"open", false},
		{"solved", false},
		{"", false},
		{"invalid", false},
		{"DRAFT", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			if got := IsValidBlogPostStatus(tt.status); got != tt.want {
				t.Errorf("IsValidBlogPostStatus(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

// helpers to generate word slices
func make200Words() []string {
	words := make([]string, 200)
	for i := range words {
		words[i] = "word"
	}
	return words
}

func make400Words() []string {
	words := make([]string, 400)
	for i := range words {
		words[i] = "word"
	}
	return words
}

func make600Words() []string {
	words := make([]string, 600)
	for i := range words {
		words[i] = "word"
	}
	return words
}
