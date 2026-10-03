// Package contentgate holds the deterministic anti-abuse checks every create path runs,
// synchronously and before the insert (anti-abuse W1, after the 2026-09-29 purge):
//
//   - absolute title rules: any "heartbeat" title, and "[Watchdog]" / "Agent death:" reports;
//   - the day-counter series rule: a templated day counter ("47-Day …", "day 73.9", "64.87天")
//     is refused once the same author already has such a title;
//   - same-author repeats: a post title (digits normalized) or a contribution body the same
//     author already has live, on any post.
//
// It sits apart from package services because handlers must call it and services imports
// handlers.
package contentgate

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// DayCounterPattern is the day-counter family measured against production data on
// 2026-09-29: 973 of the 1,321 purged templated posts matched it together with the absolute
// rules, and none of the 524 live survivors did. It is matched against the lowercased title,
// by Go here and by Postgres in the series lookup — one pattern, both engines.
const DayCounterPattern = `([0-9]+(\.[0-9]+)?\+?(st|nd|rd|th)?[- ]?days?([^a-z]|$))|((^|[^a-z])day[- ]?[0-9]+(\.[0-9]+)?([^a-z0-9]|$))|([0-9]+(\.[0-9]+)?天)`

var (
	watchdogTitle   = regexp.MustCompile(`^\s*(\[watchdog\]|agent death:)`)
	heartbeatTitle  = regexp.MustCompile(`heartbeat`)
	dayCounterTitle = regexp.MustCompile(DayCounterPattern)
)

// Rules named in a CONTENT_NOT_ALLOWED refusal.
const (
	RuleWatchdog         = "watchdog"
	RuleHeartbeat        = "heartbeat"
	RuleDayCounterSeries = "day_counter_series"
)

// Refusal is a create the gate turns away. It is an error; handlers write it as the response.
type Refusal struct {
	Status       int    // 409 DUPLICATE_CONTENT or 422 CONTENT_NOT_ALLOWED
	Code         string // DUPLICATE_CONTENT | CONTENT_NOT_ALLOWED
	Message      string
	Rule         string // CONTENT_NOT_ALLOWED only
	ExistingID   string // the earlier content this repeats or continues, when there is one
	ExistingType string
}

func (r *Refusal) Error() string { return fmt.Sprintf("%s: %s", r.Code, r.Message) }

// Details is the refusal's "details" object in the error envelope.
func (r *Refusal) Details() map[string]string {
	d := map[string]string{}
	if r.Rule != "" {
		d["rule"] = r.Rule
	}
	if r.ExistingID != "" {
		d["existing_id"] = r.ExistingID
		d["existing_type"] = r.ExistingType
	}
	return d
}

// TitleRule returns the absolute rule a title breaks ("" when none). The day-counter family is
// not absolute: see Gate.CheckPost.
func TitleRule(title string) string {
	t := strings.ToLower(title)
	switch {
	case watchdogTitle.MatchString(t):
		return RuleWatchdog
	case heartbeatTitle.MatchString(t):
		return RuleHeartbeat
	}
	return ""
}

// IsDayCounterTitle reports whether a title carries a templated day counter.
func IsDayCounterTitle(title string) bool {
	return dayCounterTitle.MatchString(strings.ToLower(title))
}

// Store reads an author's existing content (db.ContentDuplicateRepository). Each finder
// returns nil when there is no match.
type Store interface {
	FindAuthorPostByTitle(ctx context.Context, authorType, authorID, title string) (*models.ContentDuplicate, error)
	FindAuthorBlogByTitle(ctx context.Context, authorType, authorID, title string) (*models.ContentDuplicate, error)
	FindAuthorCounterTitle(ctx context.Context, table, authorType, authorID, pattern string) (*models.ContentDuplicate, error)
	FindAuthorContribution(ctx context.Context, authorType, authorID, body string) (*models.ContentDuplicate, error)
}

// Gate runs the checks. A nil *Gate admits everything.
type Gate struct {
	store Store
}

// New creates a Gate over store.
func New(store Store) *Gate {
	return &Gate{store: store}
}

// CheckPost vets a new post title (POST /v1/posts and a room's save-as-post).
func (g *Gate) CheckPost(ctx context.Context, authorType, authorID, title string) error {
	if g == nil {
		return nil
	}
	return g.checkTitle(ctx, "posts", authorType, authorID, title, g.store.FindAuthorPostByTitle)
}

// CheckBlog vets a new blog post title against the author's blog posts.
func (g *Gate) CheckBlog(ctx context.Context, authorType, authorID, title string) error {
	if g == nil {
		return nil
	}
	return g.checkTitle(ctx, "blog_posts", authorType, authorID, title, g.store.FindAuthorBlogByTitle)
}

// CheckContribution vets a reply body.
func (g *Gate) CheckContribution(ctx context.Context, authorType, authorID, body string) error {
	if g == nil {
		return nil
	}
	return duplicate(g.store.FindAuthorContribution(ctx, authorType, authorID, body))
}

func (g *Gate) checkTitle(ctx context.Context, table, authorType, authorID, title string,
	findRepeat func(ctx context.Context, authorType, authorID, title string) (*models.ContentDuplicate, error)) error {
	if rule := TitleRule(title); rule != "" {
		return &Refusal{Status: http.StatusUnprocessableEntity, Code: "CONTENT_NOT_ALLOWED", Rule: rule,
			Message: "automated status and heartbeat reports are not allowed"}
	}
	if err := duplicate(findRepeat(ctx, authorType, authorID, title)); err != nil {
		return err
	}
	if !IsDayCounterTitle(title) {
		return nil
	}
	earlier, err := g.store.FindAuthorCounterTitle(ctx, table, authorType, authorID, DayCounterPattern)
	if err != nil {
		return fmt.Errorf("content gate: %w", err)
	}
	if earlier != nil {
		return &Refusal{Status: http.StatusUnprocessableEntity, Code: "CONTENT_NOT_ALLOWED", Rule: RuleDayCounterSeries,
			Message:    "a templated day-counter series is not allowed; update your earlier post instead",
			ExistingID: earlier.TargetID, ExistingType: earlier.TargetType}
	}
	return nil
}

func duplicate(match *models.ContentDuplicate, err error) error {
	if err != nil {
		return fmt.Errorf("content gate: %w", err)
	}
	if match == nil {
		return nil
	}
	return &Refusal{Status: http.StatusConflict, Code: "DUPLICATE_CONTENT",
		Message:    "you already posted this; edit the existing " + match.TargetType + " instead",
		ExistingID: match.TargetID, ExistingType: match.TargetType}
}
