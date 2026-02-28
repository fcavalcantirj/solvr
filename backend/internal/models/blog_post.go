package models

import (
	"math"
	"regexp"
	"strings"
	"time"
)

// BlogPostStatus represents the status of a blog post.
type BlogPostStatus string

const (
	BlogPostStatusDraft     BlogPostStatus = "draft"
	BlogPostStatusPublished BlogPostStatus = "published"
	BlogPostStatusArchived  BlogPostStatus = "archived"
)

// BlogPost represents a blog post on Solvr.
type BlogPost struct {
	ID               string         `json:"id"`
	Slug             string         `json:"slug"`
	Title            string         `json:"title"`
	Body             string         `json:"body"`
	Excerpt          string         `json:"excerpt,omitempty"`
	Tags             []string       `json:"tags,omitempty"`
	CoverImageURL    string         `json:"cover_image_url,omitempty"`
	PostedByType     AuthorType     `json:"posted_by_type"`
	PostedByID       string         `json:"posted_by_id"`
	Status           BlogPostStatus `json:"status"`
	ViewCount        int            `json:"view_count"`
	Upvotes          int            `json:"upvotes"`
	Downvotes        int            `json:"downvotes"`
	ReadTimeMinutes  int            `json:"read_time_minutes"`
	MetaDescription  string         `json:"meta_description,omitempty"`
	PublishedAt      *time.Time     `json:"published_at,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        *time.Time     `json:"deleted_at,omitempty"`
}

// VoteScore returns the computed vote score (upvotes - downvotes).
func (bp *BlogPost) VoteScore() int {
	return bp.Upvotes - bp.Downvotes
}

// BlogPostAuthor contains author information for a blog post.
type BlogPostAuthor struct {
	Type        AuthorType `json:"type"`
	ID          string     `json:"id"`
	DisplayName string     `json:"display_name"`
	AvatarURL   string     `json:"avatar_url,omitempty"`
}

// BlogPostWithAuthor is a BlogPost with embedded author information.
type BlogPostWithAuthor struct {
	BlogPost
	Author    BlogPostAuthor `json:"author"`
	VoteScore int            `json:"vote_score"`
	UserVote  *string        `json:"user_vote"`
}

// BlogPostListOptions contains options for listing blog posts.
type BlogPostListOptions struct {
	Status     BlogPostStatus
	Tags       []string
	AuthorType AuthorType
	AuthorID   string
	Sort       string
	Page       int
	PerPage    int
	ViewerType AuthorType
	ViewerID   string
}

// BlogTag represents a tag with its usage count in blog posts.
type BlogTag struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// IsValidBlogPostStatus checks if a status string is a valid blog post status.
func IsValidBlogPostStatus(s string) bool {
	switch BlogPostStatus(s) {
	case BlogPostStatusDraft, BlogPostStatusPublished, BlogPostStatusArchived:
		return true
	default:
		return false
	}
}

// CalculateReadTime returns the estimated reading time in minutes.
// Based on 200 words per minute, minimum 1 minute.
func CalculateReadTime(body string) int {
	if body == "" {
		return 1
	}
	words := len(strings.Fields(body))
	minutes := int(math.Ceil(float64(words) / 200.0))
	if minutes < 1 {
		return 1
	}
	return minutes
}

// GenerateExcerpt truncates body to maxLen total characters (including "..." if truncated).
func GenerateExcerpt(body string, maxLen int) string {
	if body == "" {
		return ""
	}
	if len(body) <= maxLen {
		return body
	}
	if maxLen <= 3 {
		return body[:maxLen]
	}
	return body[:maxLen-3] + "..."
}

var slugNonAlphanumeric = regexp.MustCompile(`[^a-z0-9-]+`)
var slugMultipleHyphens = regexp.MustCompile(`-{2,}`)

// GenerateSlug converts a title to a URL-friendly slug.
func GenerateSlug(title string) string {
	slug := strings.ToLower(strings.TrimSpace(title))
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = slugNonAlphanumeric.ReplaceAllString(slug, "")
	slug = slugMultipleHyphens.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	return slug
}
