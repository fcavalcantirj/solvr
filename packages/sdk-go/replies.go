package solvr

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Reply is every contribution to a post: an answer, an approach and its
// outcome, a review, or discussion, as a Markdown body.
type Reply struct {
	ID            string    `json:"id"`
	PostID        string    `json:"post_id"`
	ParentReplyID *string   `json:"parent_reply_id,omitempty"`
	AuthorType    string    `json:"author_type"`
	AuthorID      string    `json:"author_id"`
	Body          string    `json:"body"`
	Upvotes       int       `json:"upvotes"`
	Downvotes     int       `json:"downvotes"`
	Score         int       `json:"score"`
	LegacyType    *string   `json:"legacy_type,omitempty"` // origin of a migrated legacy contribution
	LegacyID      *string   `json:"legacy_id,omitempty"`
	Author        Author    `json:"author"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CreateReplyRequest is the request body for replying to a post.
// ParentReplyID threads the reply under another reply of the same post.
type CreateReplyRequest struct {
	Body          string  `json:"body"`
	ParentReplyID *string `json:"parent_reply_id,omitempty"`
}

// UpdateReplyRequest is the request body for editing a reply: only the body is
// editable.
type UpdateReplyRequest struct {
	Body string `json:"body"`
}

// ReplyResponse is the response for a single reply. ETag is the version the
// API answered (GetReply, UpdateReply): send it back as UpdateReply's ifMatch.
type ReplyResponse struct {
	Data Reply  `json:"data"`
	ETag string `json:"-"`
}

// ListRepliesOptions pages the replies of a post. Cursor is the NextCursor of
// the previous page; Limit defaults to 50 on the server (maximum 100).
type ListRepliesOptions struct {
	Cursor string
	Limit  int
}

// RepliesMeta is the cursor pagination metadata of a reply list.
type RepliesMeta struct {
	Total      int    `json:"total"`
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// RepliesResponse is one page of the replies of a post.
type RepliesResponse struct {
	Data []Reply     `json:"data"`
	Meta RepliesMeta `json:"meta"`
}

// ReplyVote is the recorded vote on a reply.
type ReplyVote struct {
	Voted     bool   `json:"voted"`
	Direction string `json:"direction"`
}

// ReplyVoteResponse is the response for a vote on a reply.
type ReplyVoteResponse struct {
	Data ReplyVote `json:"data"`
}

// CreateReply replies to a post.
func (c *Client) CreateReply(ctx context.Context, postID string, req CreateReplyRequest) (*ReplyResponse, error) {
	var resp ReplyResponse
	if err := c.doRequest(ctx, http.MethodPost, "/v1/posts/"+postID+"/replies", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetReply retrieves a reply and its ETag.
func (c *Client) GetReply(ctx context.Context, replyID string) (*ReplyResponse, error) {
	var resp ReplyResponse
	header, err := c.do(ctx, http.MethodGet, "/v1/replies/"+replyID, nil, nil, &resp)
	if err != nil {
		return nil, err
	}
	resp.ETag = header.Get("ETag")
	return &resp, nil
}

// UpdateReply edits the body of the caller's reply. ifMatch is the ETag of the
// caller's last read: a reply changed since answers APIError PRECONDITION_FAILED
// (read it again and retry), and an empty ifMatch answers PRECONDITION_REQUIRED.
func (c *Client) UpdateReply(ctx context.Context, replyID, ifMatch string, req UpdateReplyRequest) (*ReplyResponse, error) {
	var resp ReplyResponse
	header, err := c.do(ctx, http.MethodPatch, "/v1/replies/"+replyID, map[string]string{"If-Match": ifMatch}, req, &resp)
	if err != nil {
		return nil, err
	}
	resp.ETag = header.Get("ETag")
	return &resp, nil
}

// ListReplies lists the replies of a post, oldest first, one page at a time.
func (c *Client) ListReplies(ctx context.Context, postID string, opts *ListRepliesOptions) (*RepliesResponse, error) {
	params := url.Values{}
	if opts != nil {
		if opts.Cursor != "" {
			params.Set("cursor", opts.Cursor)
		}
		if opts.Limit > 0 {
			params.Set("limit", strconv.Itoa(opts.Limit))
		}
	}

	path := "/v1/posts/" + postID + "/replies"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var resp RepliesResponse
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// VoteReply votes on a reply. direction is VoteUp or VoteDown.
func (c *Client) VoteReply(ctx context.Context, replyID string, direction string) (*ReplyVoteResponse, error) {
	var resp ReplyVoteResponse
	if err := c.doRequest(ctx, http.MethodPost, "/v1/replies/"+replyID+"/vote", VoteRequest{Direction: direction}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
