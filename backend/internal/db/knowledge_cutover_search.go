package db

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
)

// SearchSampleReport compares the default public search before and after the conversion for
// the most frequent recorded queries (idx 77 step 5), the way the counter steps compare each
// counter with its records. Pairs are (query, post) or (query, contribution). The full-text
// path is compared: the cutover has no embedding service.
type SearchSampleReport struct {
	Queries []string `json:"queries"`
	// Errored are the sampled queries the search rejected before the conversion; they are left
	// out of the comparison. ErroredAfter are those it rejects only after: a failure.
	Errored      []string `json:"errored,omitempty"`
	ErroredAfter []string `json:"errored_after,omitempty"`

	PostsBefore int `json:"posts_before"`
	PostsAfter  int `json:"posts_after"`
	// LostPosts were found before and not after although public search still reads them: a
	// failure. UnsearchablePosts were found before and are no longer searchable (their state
	// changed), so their absence is expected.
	LostPosts         []SearchSampleMiss `json:"lost_posts,omitempty"`
	UnsearchablePosts []SearchSampleMiss `json:"unsearchable_posts,omitempty"`

	// Contributions are the live legacy answers and approaches the legacy contribution search
	// matched. Each is found after as a reply by the answer or approach reply search, or is
	// hidden: its reply or post is one public search does not read (or it has no reply, being
	// on a deleted post). MissingContributions are the rest: a failure.
	Contributions        int                `json:"contributions"`
	ContributionsFound   int                `json:"contributions_found"`
	ContributionsHidden  int                `json:"contributions_hidden"`
	MissingContributions []SearchSampleMiss `json:"missing_contributions,omitempty"`
}

// SearchSampleMiss is one post (Kind "post") or legacy contribution (Kind "answer" or
// "approach") a sampled query found before and does not find after.
type SearchSampleMiss struct {
	Query string `json:"query"`
	ID    string `json:"id"`
	Kind  string `json:"kind"`
}

// searchSamplePageSize is the page the sample reads every result with.
const searchSamplePageSize = 50

// sampledQueriesSQL lists the most frequent recorded queries, ties by text.
const sampledQueriesSQL = `SELECT query_normalized FROM search_queries WHERE btrim(query_normalized) <> ''
	GROUP BY query_normalized ORDER BY count(*) DESC, query_normalized LIMIT $1`

// legacyContributionSearchSQL is the legacy contribution search: live answers by content,
// live approaches by angle, method, outcome and solution.
const legacyContributionSearchSQL = `
	SELECT 'answer', id::text FROM answers WHERE deleted_at IS NULL
		AND to_tsvector('english', content) @@ to_tsquery('english', $1)
	UNION ALL
	SELECT 'approach', id::text FROM approaches WHERE deleted_at IS NULL
		AND to_tsvector('english', COALESCE(angle, '') || ' ' || COALESCE(method, '') || ' ' ||
			COALESCE(outcome, '') || ' ' || COALESCE(solution, '')) @@ to_tsquery('english', $1)`

// contributionRepliesSQL maps legacy contributions to their replies and says whether public
// search reads each: the rule the default search anchors replies with.
const contributionRepliesSQL = `SELECT r.legacy_type, r.legacy_id::text, r.id::text,
		(r.deleted_at IS NULL AND r.author_type <> 'system' AND ` + replyMatchPostRule + `
			AND ` + "p.visibility = 'public'" + `)
	FROM replies r JOIN posts p ON p.id = r.post_id
	WHERE (r.legacy_type, r.legacy_id) IN (SELECT * FROM unnest($1::text[], $2::uuid[]))`

// searchablePostsSQL keeps the given posts public search still finds by their own text.
const searchablePostsSQL = `SELECT p.id::text FROM posts p
	WHERE p.id = ANY($1::uuid[]) AND ` + searchablePostRule + ` AND p.visibility = 'public'`

type legacyContribution struct{ kind, id string }

// searchSampler holds what the sample found before the conversion.
type searchSampler struct {
	pool    *Pool
	search  *SearchRepository
	rep     *SearchSampleReport
	posts   map[string][]string
	matched map[string][]legacyContribution
}

func newSearchSampler(pool *Pool) *searchSampler {
	return &searchSampler{pool: pool, search: NewSearchRepository(pool), rep: &SearchSampleReport{},
		posts: map[string][]string{}, matched: map[string][]legacyContribution{}}
}

// before samples the queries and records, for each, every post the default search finds and
// every contribution the legacy contribution search matches.
func (s *searchSampler) before(ctx context.Context, n int) (any, error) {
	rows, err := s.pool.Query(ctx, sampledQueriesSQL, n)
	if err != nil {
		return nil, fmt.Errorf("sample queries: %w", err)
	}
	if s.rep.Queries, err = pgx.CollectRows(rows, pgx.RowTo[string]); err != nil {
		return nil, fmt.Errorf("sample queries: %w", err)
	}
	for _, q := range s.rep.Queries {
		ids, err := s.allResults(ctx, q, "")
		if err != nil {
			s.rep.Errored = append(s.rep.Errored, q)
			continue
		}
		s.posts[q] = ids
		s.rep.PostsBefore += len(ids)
		if s.matched[q], err = s.legacyMatches(ctx, q); err != nil {
			return nil, err
		}
		s.rep.Contributions += len(s.matched[q])
	}
	return s.counts(), nil
}

// after runs the same queries on the converted model and compares.
func (s *searchSampler) after(ctx context.Context) (any, error) {
	after := map[string][]string{}
	var lost []string
	for _, q := range s.rep.Queries {
		before, ok := s.posts[q]
		if !ok {
			continue
		}
		ids, err := s.allResults(ctx, q, "")
		if err != nil {
			s.rep.ErroredAfter = append(s.rep.ErroredAfter, q)
			continue
		}
		after[q] = ids
		s.rep.PostsAfter += len(ids)
		lost = append(lost, missingFrom(before, ids)...)
		if err := s.compareContributions(ctx, q); err != nil {
			return nil, err
		}
	}
	searchable, err := s.idSet(ctx, searchablePostsSQL, lost)
	if err != nil {
		return nil, err
	}
	s.rep.LostPosts, s.rep.UnsearchablePosts = compareSampledPosts(s.rep.Queries, s.posts, after, searchable)
	return s.counts(), s.rep.failure()
}

// compareContributions finds each contribution q matched before among the replies the answer
// and approach reply searches find after.
func (s *searchSampler) compareContributions(ctx context.Context, q string) error {
	matched := s.matched[q]
	if len(matched) == 0 {
		return nil
	}
	found := map[string]bool{}
	for _, contentType := range []string{"answers", "approaches"} {
		ids, err := s.allResults(ctx, q, contentType)
		if err != nil {
			return fmt.Errorf("%s search %q: %w", contentType, q, err)
		}
		for _, id := range ids {
			found[id] = true
		}
	}
	kinds, ids := make([]string, len(matched)), make([]string, len(matched))
	for i, c := range matched {
		kinds[i], ids[i] = c.kind, c.id
	}
	rows, err := s.pool.Query(ctx, contributionRepliesSQL, kinds, ids)
	if err != nil {
		return fmt.Errorf("contribution replies: %w", err)
	}
	defer rows.Close()
	type reply struct {
		id       string
		readable bool
	}
	replies := map[legacyContribution]reply{}
	for rows.Next() {
		var c legacyContribution
		var r reply
		if err := rows.Scan(&c.kind, &c.id, &r.id, &r.readable); err != nil {
			return err
		}
		replies[c] = r
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, c := range matched {
		r, ok := replies[c]
		switch {
		case !ok || !r.readable:
			s.rep.ContributionsHidden++
		case found[r.id]:
			s.rep.ContributionsFound++
		default:
			s.rep.MissingContributions = append(s.rep.MissingContributions, SearchSampleMiss{Query: q, ID: c.id, Kind: c.kind})
		}
	}
	return nil
}

// allResults reads every page of the default search (contentType "") or of one reply search.
func (s *searchSampler) allResults(ctx context.Context, q, contentType string) ([]string, error) {
	var ids []string
	for page := 1; ; page++ {
		opts := models.SearchOptions{Page: page, PerPage: searchSamplePageSize}
		if contentType != "" {
			opts.ContentTypes = []string{contentType}
		}
		results, total, _, _, err := s.search.Search(ctx, q, opts)
		if err != nil {
			return nil, err
		}
		for _, r := range results {
			ids = append(ids, r.ID)
		}
		if len(results) == 0 || len(ids) >= total {
			return ids, nil
		}
	}
}

func (s *searchSampler) legacyMatches(ctx context.Context, q string) ([]legacyContribution, error) {
	tsquery := buildTsQuery(q)
	if tsquery == "" {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, legacyContributionSearchSQL, tsquery)
	if err != nil {
		return nil, fmt.Errorf("legacy contribution search %q: %w", q, err)
	}
	defer rows.Close()
	var out []legacyContribution
	for rows.Next() {
		var c legacyContribution
		if err := rows.Scan(&c.kind, &c.id); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *searchSampler) idSet(ctx context.Context, sql string, ids []string) (map[string]bool, error) {
	set := map[string]bool{}
	if len(ids) == 0 {
		return set, nil
	}
	rows, err := s.pool.Query(ctx, sql, ids)
	if err != nil {
		return nil, err
	}
	found, err := pgx.CollectRows(rows, pgx.RowTo[string])
	for _, id := range found {
		set[id] = true
	}
	return set, err
}

// counts is the step's ledger result.
func (s *searchSampler) counts() map[string]int {
	r := s.rep
	return map[string]int{"queries": len(r.Queries), "errored": len(r.Errored), "errored_after": len(r.ErroredAfter),
		"posts_before": r.PostsBefore, "posts_after": r.PostsAfter, "lost_posts": len(r.LostPosts),
		"unsearchable_posts": len(r.UnsearchablePosts), "contributions": r.Contributions,
		"contributions_found": r.ContributionsFound, "contributions_hidden": r.ContributionsHidden,
		"missing_contributions": len(r.MissingContributions)}
}

// compareSampledPosts lists, per query in order, the posts found before and not after: lost
// when public search still reads them, unsearchable when it no longer does.
func compareSampledPosts(queries []string, before, after map[string][]string, searchable map[string]bool) (lost, unsearchable []SearchSampleMiss) {
	for _, q := range queries {
		a, ok := after[q]
		if !ok {
			continue
		}
		for _, id := range missingFrom(before[q], a) {
			miss := SearchSampleMiss{Query: q, ID: id, Kind: "post"}
			if searchable[id] {
				lost = append(lost, miss)
			} else {
				unsearchable = append(unsearchable, miss)
			}
		}
	}
	return lost, unsearchable
}

// missingFrom lists the ids of before that after does not hold, in before's order.
func missingFrom(before, after []string) []string {
	have := make(map[string]bool, len(after))
	for _, id := range after {
		have[id] = true
	}
	var out []string
	for _, id := range before {
		if !have[id] {
			out = append(out, id)
		}
	}
	return out
}

// failure is nil unless the conversion lost something public search still reads.
func (r *SearchSampleReport) failure() error {
	var parts []string
	if n := len(r.LostPosts); n > 0 {
		parts = append(parts, fmt.Sprintf("%d post(s) found before are not found after while still searchable", n))
	}
	if n := len(r.MissingContributions); n > 0 {
		parts = append(parts, fmt.Sprintf("%d matched contribution(s) are not found as a reply", n))
	}
	if n := len(r.ErroredAfter); n > 0 {
		parts = append(parts, fmt.Sprintf("%d query(ies) fail only after the conversion", n))
	}
	if len(parts) == 0 {
		return nil
	}
	return errors.New(strings.Join(parts, "; "))
}
