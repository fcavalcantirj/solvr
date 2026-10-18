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
	// Traffic is what the counted_* views left out of the counts above (SPEC.md 27.6).
	Traffic SEOTraffic `json:"traffic"`
	// PageViews are the consent-free page views by kind of page and class.
	PageViews []PageViewCount `json:"pageviews"`
}

// SEOTraffic is what the baseline's counts left out, and why.
type SEOTraffic struct {
	Excluded []SEOExcludedCount `json:"excluded"`
}

// SEOExcludedCount is the rows of one table left out of the counts for one reason: a funnel
// step or a search classed crawler or automation, or a search a link brought.
type SEOExcludedCount struct {
	Table  string `json:"table"`
	Event  string `json:"event,omitempty"`
	Class  string `json:"traffic_class"`
	Reason string `json:"traffic_reason"`
	Entry  string `json:"entry,omitempty"`
	Count  int    `json:"count"`
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
	b := &SEOBaseline{From: from, To: to, Funnel: []SEOFunnelCount{}, Landings: []SEOLanding{},
		Traffic: SEOTraffic{Excluded: []SEOExcludedCount{}}, PageViews: []PageViewCount{}}

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
		  (SELECT COUNT(*) FROM room_events ev JOIN rooms r ON r.id = ev.room_id
		    WHERE ev.event_type = $3 AND ev.created_at >= $1 AND ev.created_at < $2 AND r.deleted_at IS NULL),
		  (SELECT COUNT(*) FROM counted_funnel_events WHERE event_name = 'first_two_way_exchange' AND occurred_at >= $1 AND occurred_at < $2)`,
		from, to, RoomActivationEventType).Scan(&b.Activation.RoomsActivated, &b.Activation.FirstTwoWayExchanges); err != nil {
		return nil, fmt.Errorf("seo baseline activation: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT event_name, COALESCE(entry_surface, ''), COUNT(*)
		FROM counted_funnel_events WHERE occurred_at >= $1 AND occurred_at < $2
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
		FROM counted_funnel_events f
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
		FROM counted_search_queries WHERE searched_at >= $1 AND searched_at < $2`, from, to).Scan(&b.Search.Queries, &b.Search.ZeroResult); err != nil {
		return nil, fmt.Errorf("seo baseline search: %w", err)
	}
	if err := r.measureExcluded(ctx, from, to, b); err != nil {
		return nil, err
	}
	if err := r.measurePageViews(ctx, from, to, b); err != nil {
		return nil, err
	}
	return b, nil
}

// measureExcluded reads what the counted_* views left out of the window, by table, step,
// class, reason and entry. It reads the raw tables on purpose: it is the account of the rows
// the counts above do not contain.
func (r *SEOBaselineRepository) measureExcluded(ctx context.Context, from, to time.Time, b *SEOBaseline) error {
	rows, err := r.pool.Query(ctx, `
		SELECT 'funnel_events', event_name, traffic_class, COALESCE(traffic_reason, ''), '', count(*)::int
		  FROM funnel_events
		 WHERE occurred_at >= $1 AND occurred_at < $2 AND traffic_class IN ('crawler', 'automation')
		 GROUP BY 2, 3, 4
		UNION ALL
		SELECT 'search_queries', '', traffic_class, COALESCE(traffic_reason, ''), COALESCE(entry, ''), count(*)::int
		  FROM search_queries
		 WHERE searched_at >= $1 AND searched_at < $2
		   AND (traffic_class IN ('crawler', 'automation') OR entry = 'link')
		 GROUP BY 3, 4, 5
		 ORDER BY 1, 2, 3, 4, 5`, from, to)
	if err != nil {
		return fmt.Errorf("seo baseline excluded traffic: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var c SEOExcludedCount
		if err := rows.Scan(&c.Table, &c.Event, &c.Class, &c.Reason, &c.Entry, &c.Count); err != nil {
			return err
		}
		b.Traffic.Excluded = append(b.Traffic.Excluded, c)
	}
	return rows.Err()
}

// measurePageViews reads the window's consent-free page views by kind of page and class.
func (r *SEOBaselineRepository) measurePageViews(ctx context.Context, from, to time.Time, b *SEOBaseline) error {
	rows, err := r.pool.Query(ctx, `
		SELECT content_group, traffic_class, count(*)::int FROM page_view_events
		 WHERE occurred_at >= $1 AND occurred_at < $2 GROUP BY 1, 2 ORDER BY 1, 2`, from, to)
	if err != nil {
		return fmt.Errorf("seo baseline page views: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var c PageViewCount
		if err := rows.Scan(&c.ContentGroup, &c.Class, &c.Count); err != nil {
			return err
		}
		b.PageViews = append(b.PageViews, c)
	}
	return rows.Err()
}
