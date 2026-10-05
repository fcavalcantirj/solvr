package db

import (
	"context"
	"fmt"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// SEOBaseline is what the server alone can measure for the weekly SEO baseline (task
// idx 85, SPEC.md Part 27.6). Impressions, clicks, CTR, position, crawling and indexing
// live in Google Search Console and are joined offline by cmd/seo-report. Nothing here
// names a private room, a non-indexable post or a search text.
type SEOBaseline struct {
	From       time.Time              `json:"from"`
	To         time.Time              `json:"to"`
	Indexable  SEOIndexable           `json:"indexable"`
	Lastmod    models.SitemapLastmods `json:"lastmod"`
	Activation SEOActivation          `json:"activation"`
	Funnel     []SEOFunnelCount       `json:"funnel"`
	Landings   []SEOLanding           `json:"landings"`
	Search     SEOSearch              `json:"search"`
}

// SEOIndexable counts the pages the site offers for indexing now.
type SEOIndexable struct {
	Posts            int `json:"posts"`
	Rooms            int `json:"rooms"`
	RoomHistoryPages int `json:"room_history_pages"`
	Agents           int `json:"agents"`
	// Users are the people's profiles with public content (SPEC.md 27.1).
	Users     int `json:"users"`
	BlogPosts int `json:"blog_posts"`
}

// SEOActivation counts activation milestones recorded in the window.
type SEOActivation struct {
	RoomsActivated       int `json:"rooms_activated"`
	FirstTwoWayExchanges int `json:"first_two_way_exchanges"`
}

// SEOFunnelCount is one connection-funnel step at one entry surface in the window.
type SEOFunnelCount struct {
	Event        string `json:"event"`
	EntrySurface string `json:"entry_surface"`
	Count        int    `json:"count"`
}

// SEOLanding is a public room or indexable post page with the connections (rooms
// created) and activations (first two-way exchanges) attributed to it in the window.
type SEOLanding struct {
	Path        string `json:"path"`
	Kind        string `json:"kind"`
	Connections int    `json:"connections"`
	Activations int    `json:"activations"`
}

// SEOSearch counts searches in the window; their text never leaves the database.
type SEOSearch struct {
	Queries    int `json:"queries"`
	ZeroResult int `json:"zero_result"`
}

// SEOBaselineRepository measures the SEO baseline.
type SEOBaselineRepository struct {
	pool    *Pool
	sitemap *SitemapRepository
}

// NewSEOBaselineRepository creates an SEOBaselineRepository.
func NewSEOBaselineRepository(pool *Pool) *SEOBaselineRepository {
	return &SEOBaselineRepository{pool: pool, sitemap: NewSitemapRepository(pool)}
}

// Measure reads the baseline for [from, to). historyPageSize is the transcript page
// size (handlers.HistoryPageSize), so transcript pages are counted as the site serves them.
func (r *SEOBaselineRepository) Measure(ctx context.Context, from, to time.Time, historyPageSize int) (*SEOBaseline, error) {
	b := &SEOBaseline{From: from, To: to, Funnel: []SEOFunnelCount{}, Landings: []SEOLanding{}}

	counts, err := r.sitemap.GetSitemapCounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("seo baseline counts: %w", err)
	}
	b.Indexable = SEOIndexable{Posts: counts.Posts, Rooms: counts.Rooms, Agents: counts.Agents, Users: counts.Users, BlogPosts: counts.BlogPosts}
	b.Lastmod = counts.Lastmod

	if err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(CEIL(m.max_seq::numeric / $1)), 0)::int
		FROM rooms
		JOIN LATERAL (SELECT MAX(e.sequence) AS max_seq FROM room_entries e WHERE e.room_id = rooms.id) m ON true
		WHERE `+roomIndexablePredicate, historyPageSize).Scan(&b.Indexable.RoomHistoryPages); err != nil {
		return nil, fmt.Errorf("seo baseline history pages: %w", err)
	}

	if err := r.pool.QueryRow(ctx, `
		SELECT
		  (SELECT COUNT(*) FROM room_events WHERE event_type = $3 AND created_at >= $1 AND created_at < $2),
		  (SELECT COUNT(*) FROM funnel_events WHERE event_name = 'first_two_way_exchange' AND occurred_at >= $1 AND occurred_at < $2)`,
		from, to, RoomActivationEventType).Scan(&b.Activation.RoomsActivated, &b.Activation.FirstTwoWayExchanges); err != nil {
		return nil, fmt.Errorf("seo baseline activation: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT event_name, COALESCE(entry_surface, ''), COUNT(*)
		FROM funnel_events WHERE occurred_at >= $1 AND occurred_at < $2
		GROUP BY 1, 2 ORDER BY 1, 2`, from, to)
	if err != nil {
		return nil, fmt.Errorf("seo baseline funnel: %w", err)
	}
	for rows.Next() {
		var f SEOFunnelCount
		if err := rows.Scan(&f.Event, &f.EntrySurface, &f.Count); err != nil {
			rows.Close()
			return nil, err
		}
		b.Funnel = append(b.Funnel, f)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Attribution by page, only for a room that is public and live or a post that is
	// indexable: a private or withdrawn source is never named.
	rows, err = r.pool.Query(ctx, `
		SELECT CASE WHEN f.source_kind = 'room' THEN '/rooms/' || rm.slug ELSE '/posts/' || p.id::text END,
		       f.source_kind,
		       COUNT(*) FILTER (WHERE f.event_name = 'room_created'),
		       COUNT(*) FILTER (WHERE f.event_name = 'first_two_way_exchange')
		FROM funnel_events f
		LEFT JOIN rooms rm ON f.source_kind = 'room' AND rm.id = f.source_id
		     AND rm.is_private = false AND rm.deleted_at IS NULL AND (rm.expires_at IS NULL OR rm.expires_at > NOW())
		LEFT JOIN posts p ON f.source_kind = 'post' AND p.id = f.source_id
		     AND p.id IN (SELECT id FROM posts WHERE `+sitemapPostEligible+`)
		WHERE f.source_id IS NOT NULL AND f.occurred_at >= $1 AND f.occurred_at < $2
		  AND (rm.id IS NOT NULL OR p.id IS NOT NULL)
		GROUP BY 1, 2 ORDER BY 3 DESC, 1 LIMIT 200`, from, to)
	if err != nil {
		return nil, fmt.Errorf("seo baseline landings: %w", err)
	}
	for rows.Next() {
		var l SEOLanding
		if err := rows.Scan(&l.Path, &l.Kind, &l.Connections, &l.Activations); err != nil {
			rows.Close()
			return nil, err
		}
		b.Landings = append(b.Landings, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE results_count = 0)
		FROM search_queries WHERE searched_at >= $1 AND searched_at < $2`, from, to).Scan(&b.Search.Queries, &b.Search.ZeroResult); err != nil {
		return nil, fmt.Errorf("seo baseline search: %w", err)
	}
	return b, nil
}
