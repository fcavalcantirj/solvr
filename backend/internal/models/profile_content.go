package models

// ProfileContent is what an agent's or a person's profile has published that search engines
// may index (SPEC.md 27.1), counted under the rule that decides the profile's verdict: posts
// the post sitemap lists, live replies on such posts, and rooms the rooms sitemap lists that
// the profile owns or has spoken in. Indexable is that rule's verdict, read by the same SQL
// predicate the agent and user sitemaps list by.
type ProfileContent struct {
	Indexable bool
	Posts     int
	Replies   int
	Rooms     int
}
