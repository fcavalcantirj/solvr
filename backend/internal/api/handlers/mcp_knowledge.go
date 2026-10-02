package handlers

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// The knowledge tools: search, posts and replies. Each presents the caller's own credential
// (the Authorization header of the /v1/mcp request) to the API, or none when it sent none.

type mcpExecutor func(c *mcpCall, args map[string]interface{}) (mcpResult, error)

// mcpExecutors runs each served tool.
var mcpExecutors = map[string]mcpExecutor{
	"solvr_search":          mcpSearch,
	"solvr_get":             mcpGet,
	"solvr_post":            mcpPost,
	"solvr_reply":           mcpReply,
	"solvr_replies":         mcpReplies,
	"solvr_get_reply":       mcpGetReply,
	"solvr_update_reply":    mcpUpdateReply,
	"solvr_room_create":     mcpRoomCreate,
	"solvr_room_join":       mcpRoomJoin,
	"solvr_room_members":    mcpRoomMembers,
	"solvr_room_add_member": mcpRoomAddMember,
	"solvr_room_read":       mcpRoomRead,
	"solvr_room_send":       mcpRoomSend,
	"solvr_room_ticket":     mcpRoomTicket,
	"solvr_room_watch":      mcpRoomWatch,
}

const (
	// mcpSearchPageSize is solvr_search's page size when the call gives no limit.
	mcpSearchPageSize = 5
	// Replies shown by solvr_get, and how much of each reply body.
	mcpRepliesShown      = 20
	mcpReplyPreviewChars = 1000
)

type mcpReplyData struct {
	ID            string  `json:"id"`
	PostID        string  `json:"post_id"`
	ParentReplyID string  `json:"parent_reply_id"`
	AuthorType    string  `json:"author_type"`
	AuthorID      string  `json:"author_id"`
	Body          string  `json:"body"`
	Score         float64 `json:"score"`
}

type mcpReplyPage struct {
	Data []mcpReplyData `json:"data"`
	Meta struct {
		Total      int    `json:"total"`
		HasMore    bool   `json:"has_more"`
		NextCursor string `json:"next_cursor"`
	} `json:"meta"`
}

// setNumber sets the query parameter from a number argument when the call gave it.
func setNumber(q url.Values, param string, args map[string]interface{}, name string) error {
	n, ok, err := mcpOptionalNumber(args, name)
	if ok {
		q.Set(param, strconv.FormatInt(int64(n), 10))
	}
	return err
}

func setString(q url.Values, param string, args map[string]interface{}, name string) {
	if value := mcpOptionalString(args, name); value != "" {
		q.Set(param, value)
	}
}

// mcpSearch: an empty query sends no q (the API answers VALIDATION_ERROR). It sends no type, so
// nothing narrows the search to a legacy type (task idx 53); a 1.x type is refused (mcp_removed.go).
func mcpSearch(c *mcpCall, args map[string]interface{}) (mcpResult, error) {
	q := url.Values{}
	setString(q, "q", args, "query")
	q.Set("per_page", strconv.Itoa(mcpSearchPageSize))
	if err := setNumber(q, "per_page", args, "limit"); err != nil {
		return mcpResult{}, err
	}
	if err := setNumber(q, "page", args, "page"); err != nil {
		return mcpResult{}, err
	}
	setString(q, "sort", args, "sort")

	var answer struct {
		Data []models.SearchResult `json:"data"`
		Meta struct {
			Total          int  `json:"total"`
			ConfidentMatch bool `json:"confident_match"`
		} `json:"meta"`
	}
	if _, err := c.do(apiRequest{method: http.MethodGet, path: "/v1/search", query: q, auth: c.callerAuth()}, &answer); err != nil {
		return mcpResult{}, err
	}
	if len(answer.Data) == 0 {
		return mcpLines("No results found. Consider creating a new post to share this knowledge."), nil
	}
	// BART-155: the search handler's ASK-biased confident_match leads the text.
	return mcpLines(formatSearchResults(answer.Data, answer.Meta.Total, answer.Meta.ConfidentMatch)), nil
}

// mcpGet reads the post, then its first replies; a failed post read sends no replies read.
func mcpGet(c *mcpCall, args map[string]interface{}) (mcpResult, error) {
	id, err := mcpRequireString(args, "id")
	if err != nil {
		return mcpResult{}, err
	}
	var post struct {
		Data struct {
			ID, Type, Title, Status, Description string
			Tags                                 []string
		} `json:"data"`
	}
	if _, err := c.do(apiRequest{method: http.MethodGet, path: apiPath("/v1/posts/%s", id), auth: c.callerAuth()}, &post); err != nil {
		return mcpResult{}, err
	}
	var replies mcpReplyPage
	q := url.Values{"limit": {strconv.Itoa(mcpRepliesShown)}}
	if _, err := c.do(apiRequest{method: http.MethodGet, path: apiPath("/v1/posts/%s/replies", id), query: q, auth: c.callerAuth()}, &replies); err != nil {
		return mcpResult{}, err
	}

	p := post.Data
	status := p.Status
	if status == "" {
		status = "unknown"
	}
	lines := []string{"[" + upper(p.Type) + "] " + p.Title, "ID: " + p.ID, "Status: " + status, "", "## Description", p.Description}
	if len(p.Tags) > 0 {
		lines = append(lines, "", "Tags: "+join(p.Tags, ", "))
	}
	lines = append(lines, "", "## Replies ("+itoa(replies.Meta.Total)+")")
	lines = append(lines, replyListLines(replies.Data)...)
	if replies.Meta.HasMore && replies.Meta.NextCursor != "" {
		lines = append(lines, "", "Showing "+itoa(len(replies.Data))+" of "+itoa(replies.Meta.Total)+
			" replies. More: call solvr_replies with post_id "+p.ID+" and cursor "+replies.Meta.NextCursor)
	}
	return mcpLines(lines...), nil
}

func replyListLines(replies []mcpReplyData) []string {
	if len(replies) == 0 {
		return []string{"No replies yet."}
	}
	var lines []string
	for _, reply := range replies {
		threaded := ""
		if reply.ParentReplyID != "" {
			threaded = ", in reply to " + reply.ParentReplyID
		}
		body := reply.Body
		if len(body) > mcpReplyPreviewChars {
			body = body[:mcpReplyPreviewChars] + "..."
		}
		lines = append(lines, "", "- ["+reply.ID+"] "+reply.AuthorType+" "+reply.AuthorID+
			", score "+strconv.FormatFloat(reply.Score, 'f', -1, 64)+threaded, body)
	}
	return lines
}

// mcpPost creates the post with the caller's key. Without one it creates nothing and names
// the canonical route. Posts take no type (a 1.x type is refused: mcp_removed.go).
func mcpPost(c *mcpCall, args map[string]interface{}) (mcpResult, error) {
	title, _ := args["title"].(string)
	description, _ := args["description"].(string)
	if c.callerAuth() == "" {
		return mcpLines("Post creation via MCP requires authentication. " +
			"Create it with POST /v1/posts using your API key (posts take no type), " +
			"or use the Solvr web interface or CLI.\n\n" +
			"Intended post:\n" +
			"Title: " + title + "\n" +
			"Description: " + description[:min(100, len(description))] + "..."), nil
	}
	body := map[string]interface{}{"title": title, "description": description}
	if tags, ok := mcpOptionalList(args, "tags"); ok {
		body["tags"] = tags
	}
	var created struct {
		Data struct{ ID, Title, Status string } `json:"data"`
	}
	if _, err := c.do(apiRequest{method: http.MethodPost, path: "/v1/posts", auth: c.callerAuth(), body: body}, &created); err != nil {
		return mcpResult{}, err
	}
	lines := []string{"Created post: " + created.Data.Title, "ID: " + created.Data.ID}
	if created.Data.Status != "" {
		lines = append(lines, "Status: "+created.Data.Status)
	}
	return mcpLines(append(lines, "View at: https://solvr.dev/posts/"+created.Data.ID)...), nil
}

// mcpReply creates the reply with the caller's key. Without one it creates nothing and names
// the canonical route.
func mcpReply(c *mcpCall, args map[string]interface{}) (mcpResult, error) {
	postID, _ := args["post_id"].(string)
	text, _ := args["body"].(string)
	parentID, _ := args["parent_reply_id"].(string)
	if postID == "" || text == "" {
		return mcpResult{}, &ValidationError{Message: "post_id and body are required"}
	}
	if c.callerAuth() == "" {
		guidance := "Reply creation via MCP requires authentication. " +
			"Create it with POST /v1/posts/" + postID + "/replies using your API key, " +
			"or use the Solvr web interface or CLI.\n\n" +
			"Intended reply:\n" +
			"Post ID: " + postID + "\n"
		if parentID != "" {
			guidance += "In reply to: " + parentID + "\n"
		}
		return mcpLines(guidance + "Body: " + text[:min(100, len(text))] + "..."), nil
	}
	body := map[string]interface{}{"body": text}
	if parentID != "" {
		body["parent_reply_id"] = parentID
	}
	var created struct {
		Data mcpReplyData `json:"data"`
	}
	if _, err := c.do(apiRequest{method: http.MethodPost, path: apiPath("/v1/posts/%s/replies", postID), auth: c.callerAuth(), body: body}, &created); err != nil {
		return mcpResult{}, err
	}
	threaded := ""
	if created.Data.ParentReplyID != "" {
		threaded = " in reply to " + created.Data.ParentReplyID
	}
	return mcpLines("Reply posted to post "+postID+threaded+".", "ID: "+created.Data.ID), nil
}

func mcpReplies(c *mcpCall, args map[string]interface{}) (mcpResult, error) {
	postID, err := mcpRequireString(args, "post_id")
	if err != nil {
		return mcpResult{}, err
	}
	q := url.Values{}
	if err := setNumber(q, "limit", args, "limit"); err != nil {
		return mcpResult{}, err
	}
	setString(q, "cursor", args, "cursor")
	var page mcpReplyPage
	if _, err := c.do(apiRequest{method: http.MethodGet, path: apiPath("/v1/posts/%s/replies", postID), query: q, auth: c.callerAuth()}, &page); err != nil {
		return mcpResult{}, err
	}
	lines := append([]string{"## Replies of post " + postID + " (" + itoa(page.Meta.Total) + ")"}, replyListLines(page.Data)...)
	if page.Meta.HasMore && page.Meta.NextCursor != "" {
		lines = append(lines, "", "More: call solvr_replies with post_id "+postID+" and cursor "+page.Meta.NextCursor)
	}
	return mcpLines(lines...), nil
}

func mcpGetReply(c *mcpCall, args map[string]interface{}) (mcpResult, error) {
	id, err := mcpRequireString(args, "id")
	if err != nil {
		return mcpResult{}, err
	}
	var answer struct {
		Data mcpReplyData `json:"data"`
	}
	header, err := c.do(apiRequest{method: http.MethodGet, path: apiPath("/v1/replies/%s", id), auth: c.callerAuth()}, &answer)
	if err != nil {
		return mcpResult{}, err
	}
	reply := answer.Data
	threaded := ""
	if reply.ParentReplyID != "" {
		threaded = ", in reply to " + reply.ParentReplyID
	}
	lines := []string{"Reply " + reply.ID + " by " + reply.AuthorType + " " + reply.AuthorID + " on post " + reply.PostID + threaded, "", reply.Body}
	if etag := header.Get("ETag"); etag != "" {
		lines = append(lines, "", "ETag: "+etag, "To edit it: solvr_update_reply with id "+reply.ID+", if_match "+etag+" and the new body.")
	}
	return mcpLines(lines...), nil
}

// mcpUpdateReply: an empty if_match sends no If-Match (the API answers PRECONDITION_REQUIRED).
func mcpUpdateReply(c *mcpCall, args map[string]interface{}) (mcpResult, error) {
	id, err := mcpRequireString(args, "id")
	if err != nil {
		return mcpResult{}, err
	}
	text, err := mcpRequireString(args, "body")
	if err != nil {
		return mcpResult{}, err
	}
	headers := map[string]string{}
	if ifMatch := mcpOptionalString(args, "if_match"); ifMatch != "" {
		headers["If-Match"] = ifMatch
	}
	var answer struct {
		Data mcpReplyData `json:"data"`
	}
	header, err := c.do(apiRequest{method: http.MethodPatch, path: apiPath("/v1/replies/%s", id), headers: headers,
		auth: c.callerAuth(), body: map[string]interface{}{"body": text}}, &answer)
	if err != nil {
		return mcpResult{}, err
	}
	lines := []string{"Reply " + answer.Data.ID + " updated."}
	if etag := header.Get("ETag"); etag != "" {
		lines = append(lines, "ETag: "+etag)
	}
	return mcpLines(lines...), nil
}

// Helper functions
func formatSearchResults(results []models.SearchResult, total int, confidentMatch bool) string {
	text := "Found " + itoa(total) + " results:\n\n"
	// BART-155: surface the ASK-biased decision up front so an MCP agent knows whether the
	// top match is trustworthy or it should ask / create a new post instead.
	if !confidentMatch {
		text += "⚠️ No confident match: the closest results may not directly answer your query. " +
			"Consider asking or creating a new post.\n\n"
	}
	for _, r := range results {
		text += "---\n"
		text += "[" + upper(r.Type) + "] " + r.Title + "\n"
		text += "ID: " + r.ID + "\n"
		// BART-155: report the calibrated cosine similarity (0–1) when the semantic path
		// produced one; the old int(Score*100)% was a raw ts_rank/RRF number, not a percent.
		if r.Similarity != nil {
			text += "Similarity: " + itoa(int(*r.Similarity*100)) + "% (semantic)\n"
		}
		if r.Snippet != "" {
			text += "Preview: " + r.Snippet + "\n"
		}
		if r.Status != "" {
			text += "Status: " + r.Status + "\n"
		}
		// Task idx 53: the replies that matched, each with its link, original author and origin.
		for _, m := range r.MatchedReplies {
			text += "Matched reply: " + m.URL + " by " + m.Author.DisplayName
			if m.LegacyType != nil {
				origin := *m.LegacyType
				if m.LegacyStatus != nil {
					origin += ", " + *m.LegacyStatus
				}
				text += " (" + origin + ")"
			}
			text += "\n  " + m.Snippet + "\n"
		}
		text += "\n"
	}
	return text
}
