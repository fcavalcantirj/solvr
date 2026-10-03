package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// gscRow is one row of a Google Search Console Performance export (Pages or Queries).
type gscRow struct {
	Key         string
	Clicks      int
	Impressions int
	Position    float64
}

// parseGSC reads a Search Console CSV export: a key column (Top pages / Top queries),
// then Clicks, Impressions, CTR and Position. CTR is recomputed, never trusted.
func parseGSC(r io.Reader) ([]gscRow, error) {
	records, err := csv.NewReader(r).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("empty export")
	}
	col := map[string]int{}
	for i, h := range records[0] {
		col[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, need := range []string{"clicks", "impressions", "position"} {
		if _, ok := col[need]; !ok {
			return nil, fmt.Errorf("export has no %s column", need)
		}
	}
	rows := []gscRow{}
	for _, rec := range records[1:] {
		clicks, _ := strconv.Atoi(strings.TrimSpace(rec[col["clicks"]]))
		impressions, _ := strconv.Atoi(strings.TrimSpace(rec[col["impressions"]]))
		position, _ := strconv.ParseFloat(strings.TrimSpace(rec[col["position"]]), 64)
		rows = append(rows, gscRow{Key: strings.TrimSpace(rec[0]), Clicks: clicks, Impressions: impressions, Position: position})
	}
	return rows, nil
}

// querySegment separates brand queries from the rest.
func querySegment(query string, brands []string) string {
	q := strings.ToLower(query)
	for _, b := range brands {
		if b != "" && strings.Contains(q, strings.ToLower(b)) {
			return "brand"
		}
	}
	return "non-brand"
}

// pageSegment puts a landing page in the segment the SEO work is measured by.
func pageSegment(raw string) string {
	path := raw
	if u, err := url.Parse(raw); err == nil {
		path = u.Path
	}
	switch {
	case path == "" || path == "/":
		return "home"
	case strings.HasPrefix(path, "/docs/guides/"):
		return "guides"
	case path == "/docs" || strings.HasPrefix(path, "/docs/"):
		return "docs"
	case strings.HasPrefix(path, "/rooms/") && strings.Contains(path, "/history/"):
		return "room transcripts"
	case strings.HasPrefix(path, "/rooms/"):
		return "rooms"
	case strings.HasPrefix(path, "/posts/"):
		return "posts"
	}
	return "other"
}

// reviewDates are the migration reviews: one, four and eight weeks after release.
func reviewDates(release time.Time) []string {
	out := []string{}
	for _, weeks := range []int{1, 4, 8} {
		out = append(out, release.AddDate(0, 0, 7*weeks).Format("2006-01-02"))
	}
	return out
}

// baseline is what GET /admin/seo/baseline answers.
type baseline struct {
	Data struct {
		Window    string `json:"window"`
		Indexable struct {
			Posts            int `json:"posts"`
			Rooms            int `json:"rooms"`
			RoomHistoryPages int `json:"room_history_pages"`
			Agents           int `json:"agents"`
			BlogPosts        int `json:"blog_posts"`
		} `json:"indexable"`
		Activation struct {
			RoomsActivated       int `json:"rooms_activated"`
			FirstTwoWayExchanges int `json:"first_two_way_exchanges"`
		} `json:"activation"`
		Landings []struct {
			Path        string `json:"path"`
			Connections int    `json:"connections"`
			Activations int    `json:"activations"`
		} `json:"landings"`
		Search struct {
			Queries    int `json:"queries"`
			ZeroResult int `json:"zero_result"`
		} `json:"search"`
	} `json:"data"`
}

type options struct {
	baseline, pages, queries, brands, release string
}

type totals struct{ clicks, impressions int }

func (t totals) ctr() float64 {
	if t.impressions == 0 {
		return 0
	}
	return 100 * float64(t.clicks) / float64(t.impressions)
}

func readGSC(path string) ([]gscRow, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseGSC(f)
}

func line(w io.Writer, name, status, value string) {
	fmt.Fprintf(w, "  %-22s %-12s %s\n", name, status, value)
}

// run writes the weekly report. Without a Search Console export, every milestone only
// Search Console can measure is UNAVAILABLE, never zero.
func run(w io.Writer, o options) error {
	raw, err := os.ReadFile(o.baseline)
	if err != nil {
		return err
	}
	var b baseline
	if err := json.Unmarshal(raw, &b); err != nil {
		return fmt.Errorf("baseline: %w", err)
	}
	pages, err := readGSC(o.pages)
	if err != nil {
		return fmt.Errorf("pages export: %w", err)
	}
	queries, err := readGSC(o.queries)
	if err != nil {
		return fmt.Errorf("queries export: %w", err)
	}
	release, err := time.Parse("2006-01-02", o.release)
	if err != nil {
		return fmt.Errorf("release date: %w", err)
	}
	d := b.Data
	published := d.Indexable.Posts + d.Indexable.Rooms + d.Indexable.RoomHistoryPages + d.Indexable.Agents + d.Indexable.BlogPosts

	fmt.Fprintf(w, "SEO REPORT (server window %s)\n\nMILESTONES (each measured on its own)\n", d.Window)
	line(w, "publication", "measured", fmt.Sprintf("%d indexable content pages", published))
	line(w, "sitemap acceptance", "UNAVAILABLE", "Google Search Console: Sitemaps")
	line(w, "crawling", "UNAVAILABLE", "Google Search Console: Crawl stats")
	var site totals
	for _, p := range pages {
		site.clicks += p.Clicks
		site.impressions += p.Impressions
	}
	if pages == nil {
		line(w, "indexing", "UNAVAILABLE", "no Search Console export given (Page indexing report)")
		line(w, "traffic", "UNAVAILABLE", "no Search Console export given (Performance report)")
	} else {
		line(w, "indexing", "PARTIAL", fmt.Sprintf("%d pages with impressions in the export; the full status is GSC Page indexing", len(pages)))
		line(w, "traffic", "measured", fmt.Sprintf("%d clicks, %d impressions, CTR %.2f%%", site.clicks, site.impressions, site.ctr()))
	}
	line(w, "core web vitals", "UNAVAILABLE", "Google Search Console: Core Web Vitals")
	line(w, "activation", "measured", fmt.Sprintf("%d rooms activated in the window, %d first two-way exchanges", d.Activation.RoomsActivated, d.Activation.FirstTwoWayExchanges))
	fmt.Fprintf(w, "  searches: %d, zero-result: %d\n", d.Search.Queries, d.Search.ZeroResult)

	segments := func(title string, rows []gscRow, seg func(string) string) {
		fmt.Fprintf(w, "\n%s\n", title)
		if rows == nil {
			fmt.Fprintf(w, "  UNAVAILABLE: no Search Console export given\n")
			return
		}
		by := map[string]totals{}
		for _, r := range rows {
			t := by[seg(r.Key)]
			t.clicks += r.Clicks
			t.impressions += r.Impressions
			by[seg(r.Key)] = t
		}
		names := make([]string, 0, len(by))
		for n := range by {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			t := by[n]
			fmt.Fprintf(w, "  %-22s %d clicks  %d impressions  CTR %.2f%%\n", n, t.clicks, t.impressions, t.ctr())
		}
	}
	brands := strings.Split(o.brands, ",")
	segments("QUERY SEGMENTS", queries, func(q string) string { return querySegment(q, brands) })
	segments("LANDING SEGMENTS", pages, pageSegment)

	fmt.Fprintf(w, "\nLANDING PAGE -> SERVER ACTIVATION (public rooms and posts)\n")
	clicks := map[string]int{}
	for _, p := range pages {
		if u, err := url.Parse(p.Key); err == nil {
			clicks[u.Path] += p.Clicks
		}
	}
	for _, l := range d.Landings {
		c := "?"
		if pages != nil {
			c = strconv.Itoa(clicks[l.Path])
		}
		fmt.Fprintf(w, "  %-22s %s clicks  %d connections  %d activations\n", l.Path, c, l.Connections, l.Activations)
	}

	fmt.Fprintf(w, "\nREVIEWS of redirects and canonicals (seo-verify legacy and sample): %s\n", strings.Join(reviewDates(release), ", "))
	fmt.Fprintf(w, "Publishing pages does not certify crawling, indexing, traffic or activation; each is read above on its own.\n")
	return nil
}
