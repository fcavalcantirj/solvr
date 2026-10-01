package models

// Search document kinds: the rows whose stored vector (posts.embedding, replies.embedding) is
// their semantic search document (migration 000122).
const (
	SearchDocumentPost  = "post"
	SearchDocumentReply = "reply"
)

// SearchDocument is a live post or non-system reply without a stored vector (a row
// search_document_drift() lists), with the text the vector must describe.
type SearchDocument struct {
	Kind        string
	ID          string
	Title       string // posts
	Description string // posts
	Body        string // replies
}

// Text is what is embedded: a post's title and description (as at creation), a reply's body.
func (d SearchDocument) Text() string {
	if d.Kind == SearchDocumentPost {
		return d.Title + " " + d.Description
	}
	return d.Body
}

// Key is the document's position in the drift list's order.
func (d SearchDocument) Key() SearchDocumentKey {
	return SearchDocumentKey{Kind: d.Kind, ID: d.ID}
}

// SearchDocumentKey orders search documents by kind, then id. The zero key comes before every
// document.
type SearchDocumentKey struct {
	Kind string
	ID   string
}

// Less reports whether k comes before o. Ids are canonical lowercase UUIDs, whose string
// order is PostgreSQL's uuid order.
func (k SearchDocumentKey) Less(o SearchDocumentKey) bool {
	if k.Kind != o.Kind {
		return k.Kind < o.Kind
	}
	return k.ID < o.ID
}

// IsZero reports whether k is the zero key.
func (k SearchDocumentKey) IsZero() bool {
	return k == SearchDocumentKey{}
}
