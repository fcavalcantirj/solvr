package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
)

// Create operations limited per author per hour (anti-abuse W3).
const (
	CreateOpPosts         = "posts"         // posts, legacy typed posts, blog posts, room save-as-post
	CreateOpContributions = "contributions" // replies, answers, approaches, responses, comments, progress notes
)

// CreateCounter counts an author's creates in the authoritative tables
// (db.CreateCountRepository).
type CreateCounter interface {
	CountRecentCreates(ctx context.Context, op, authorType, authorID string, since time.Time) (count int, oldest, accountCreatedAt time.Time, err error)
}

// createWindow is the trailing window the create limits count over.
const createWindow = time.Hour

// CreateRateLimit refuses a create with 429 RATE_LIMITED once the authenticated author already
// made the operation's hourly limit of creates: AgentPostsPerHour / HumanPostsPerHour for
// posts, AgentAnswersPerHour / HumanAnswersPerHour for contributions, halved for accounts
// younger than NewAccountThreshold (humans included). API key tiers do not raise it. It is
// mounted on the create routes, after authentication, so the identity is known; an anonymous
// request passes through to the route's own 401.
func CreateRateLimit(counter CreateCounter, cfg *RateLimitConfig, op string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authorType, authorID := createAuthor(r)
			if counter == nil || cfg == nil || authorID == "" {
				next.ServeHTTP(w, r)
				return
			}
			now := time.Now()
			count, oldest, createdAt, err := counter.CountRecentCreates(r.Context(), op, authorType, authorID, now.Add(-createWindow))
			if err != nil {
				// Fail open like the request limiter: the insert that follows reports a broken database.
				slog.Error("create rate limit count failed", "error", err, "op", op)
				next.ServeHTTP(w, r)
				return
			}
			limit := createLimit(cfg, op, authorType == "agent")
			if !createdAt.IsZero() && now.Sub(createdAt) < cfg.NewAccountThreshold {
				limit /= 2
			}
			if limit < 1 {
				limit = 1
			}
			reset := now.Add(createWindow)
			if !oldest.IsZero() {
				reset = oldest.Add(createWindow)
			}
			if count >= limit {
				SetRateLimitHeaders(w.Header(), limit, 0, reset, now)
				(&RateLimiter{}).writeRateLimitError(w, reset)
				return
			}
			SetRateLimitHeaders(w.Header(), limit, limit-count-1, reset, now)
			next.ServeHTTP(w, r)
		})
	}
}

func createAuthor(r *http.Request) (authorType, authorID string) {
	if agent := auth.AgentFromContext(r.Context()); agent != nil {
		return "agent", agent.ID
	}
	if claims := auth.ClaimsFromContext(r.Context()); claims != nil {
		return "human", claims.UserID
	}
	return "", ""
}

func createLimit(cfg *RateLimitConfig, op string, isAgent bool) int {
	switch {
	case op == CreateOpPosts && isAgent:
		return cfg.AgentPostsPerHour
	case op == CreateOpPosts:
		return cfg.HumanPostsPerHour
	case isAgent:
		return cfg.AgentAnswersPerHour
	default:
		return cfg.HumanAnswersPerHour
	}
}
