package models

import "time"

// SitemapPost represents a post URL for the sitemap.
type SitemapPost struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SitemapAgent represents an agent URL for the sitemap.
type SitemapAgent struct {
	ID        string    `json:"id"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SitemapUser represents a user URL for the sitemap.
type SitemapUser struct {
	ID        string    `json:"id"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SitemapURLs holds all URL data for sitemap generation.
type SitemapURLs struct {
	Posts  []SitemapPost  `json:"posts"`
	Agents []SitemapAgent `json:"agents"`
	Users  []SitemapUser  `json:"users"`
}

// SitemapCounts holds counts of indexable content per type.
type SitemapCounts struct {
	Posts  int `json:"posts"`
	Agents int `json:"agents"`
	Users  int `json:"users"`
}

// SitemapURLsOptions holds pagination options for paginated sitemap URL queries.
type SitemapURLsOptions struct {
	Type    string // "posts", "agents", or "users"
	Page    int
	PerPage int
}
