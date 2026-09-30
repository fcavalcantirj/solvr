package jobs_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
)

// The write probe's call sequence. Steps run in order and each builds its calls from what
// earlier calls created, so a step may only depend on the steps above it: account and
// content writes first, then the legacy writes, rooms, the rest, the deletions and storage
// last.

// probeCID and probePinCID are well-formed CIDv1s (a checkpoint is a pin, so the two differ:
// one owner cannot pin a CID twice); the probe's IPFS endpoint is a closed port, so nothing
// is ever fetched or pinned.
const (
	probeCID    = "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi"
	probePinCID = "bafybeibwzifw52ttrkqlikfzext5akxu7lz4xiwjgwzmqcpdzmp3n5vnbe"
)

const probeDescription = "A description written by the write route probe, long enough for every length check."

var probePostTypes = []string{"problem", "question", "idea", "post"}

func j(fields map[string]any) string {
	b, err := json.Marshal(fields)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func call(route, caller, path, body string) writeProbeCall {
	return writeProbeCall{route: route, caller: caller, path: path, body: body}
}

// first returns the first non-empty value found at one of the key paths.
func (v jsonValue) first(paths ...[]string) string {
	for _, p := range paths {
		if got := v.at(p...); got != "" {
			return got
		}
	}
	return ""
}

// keepID stores the created object's id (top level or under data) through field.
func keepID(field func(s *writeProbeState) *string, key string) func(*writeProbeState, jsonValue) bool {
	return func(s *writeProbeState, v jsonValue) bool {
		got := v.first([]string{"data", key}, []string{key})
		*field(s) = got
		return got != ""
	}
}

// postAuthor is the caller who wrote the fixture post of a type (seedRouteProbeData
// alternates agent and human).
func postAuthor(i int) string {
	if i%2 == 1 {
		return "human"
	}
	return "agent"
}

// replyAuthor is the caller who wrote fixture reply i (per post: a native reply by the
// human, then a migrated reply by the agent).
func replyAuthor(i int) string {
	if i%2 == 0 {
		return "human"
	}
	return "agent"
}

func probeUpload() (body, contentType string) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", "probe.txt")
	if err == nil {
		_, err = part.Write([]byte("route probe upload"))
	}
	if err == nil {
		err = w.Close()
	}
	if err != nil {
		panic(err)
	}
	return buf.String(), w.FormDataContentType()
}

var writeProbeSteps = []func(s *writeProbeState) []writeProbeCall{
	// Accounts and authentication.
	func(s *writeProbeState) []writeProbeCall {
		c := call("POST /v1/agents/register", "anonymous", "/v1/agents/register",
			j(map[string]any{"name": "probe_second_agent", "description": "second route probe agent"}))
		c.keep = func(s *writeProbeState, v jsonValue) bool {
			s.agent2Key = v.first([]string{"api_key"}, []string{"data", "api_key"})
			s.agent2ID = v.first([]string{"agent", "id"}, []string{"data", "agent", "id"})
			return s.agent2Key != "" && s.agent2ID != ""
		}
		return []writeProbeCall{c}
	},
	func(s *writeProbeState) []writeProbeCall {
		c := call("POST /v1/auth/register", "anonymous", "/v1/auth/register", j(map[string]any{
			"email": "probe.second@test.solvr.dev", "password": "Probe-pass-123!", "username": "probe_second",
			"display_name": "Probe Second",
		}))
		c.keep = func(s *writeProbeState, v jsonValue) bool {
			s.human2JWT = v.first([]string{"access_token"}, []string{"data", "access_token"})
			s.human2ID = v.first([]string{"user", "id"}, []string{"data", "user", "id"})
			return s.human2JWT != "" && s.human2ID != ""
		}
		return []writeProbeCall{
			c,
			call("POST /v1/auth/login", "anonymous", "/v1/auth/login",
				j(map[string]any{"email": "probe.second@test.solvr.dev", "password": "Probe-pass-123!"})),
			call("POST /v1/auth/claim-referral", "human2", "/v1/auth/claim-referral", j(map[string]any{"ref": "PROBEREF"})),
			call("POST /v1/auth/oauth/exchange", "anonymous", "/v1/auth/oauth/exchange",
				j(map[string]any{"login_code": "route-probe-unknown-code"})),
			// An empty identity token: a real one is verified against the external Moltbook service.
			call("POST /v1/auth/moltbook", "anonymous", "/v1/auth/moltbook", `{}`),
		}
	},
	func(s *writeProbeState) []writeProbeCall {
		keyCall := call("POST /v1/users/me/api-keys", "human", "/v1/users/me/api-keys", j(map[string]any{"name": "probe key"}))
		keyCall.keep = keepID(func(s *writeProbeState) *string { return &s.apiKeyID }, "id")
		identity := j(map[string]any{"amcp_aid": "EProbeAIDRouteProbe0000000000000000000000000"})
		return []writeProbeCall{
			call("PATCH /v1/me", "human", "/v1/me", j(map[string]any{"bio": "route probe bio"})),
			call("PATCH /v1/me", "agent", "/v1/me", j(map[string]any{"bio": "route probe bio"})),
			keyCall,
			call("PATCH /v1/agents/{id}", "agent", "/v1/agents/"+s.agentID, j(map[string]any{"bio": "route probe agent bio"})),
			call("PATCH /v1/agents/{id}", "human", "/v1/agents/"+s.agentID, j(map[string]any{"bio": "not the owner"})),
			call("PATCH /v1/agents/me/identity", "agent", "/v1/agents/me/identity", identity),
			call("POST /v1/follow", "human", "/v1/follow", j(map[string]any{"target_type": "agent", "target_id": s.agentID})),
			call("POST /v1/follow", "agent", "/v1/follow", j(map[string]any{"target_type": "human", "target_id": s.userID})),
		}
	},
	func(s *writeProbeState) []writeProbeCall {
		c := call("POST /v1/agents/me/claim", "agent2", "/v1/agents/me/claim", `{}`)
		c.keep = keepID(func(s *writeProbeState) *string { return &s.claimToken }, "token")
		return []writeProbeCall{
			call("POST /v1/users/me/api-keys/{id}/regenerate", "human", "/v1/users/me/api-keys/"+or(s.apiKeyID)+"/regenerate", `{}`),
			c,
		}
	},
	func(s *writeProbeState) []writeProbeCall {
		token := j(map[string]any{"token": or(s.claimToken)})
		return []writeProbeCall{
			call("POST /v1/agents/claim/lookup", "anonymous", "/v1/agents/claim/lookup", token),
			call("POST /v1/agents/claim", "human", "/v1/agents/claim", token),
		}
	},

	// Canonical posts and replies.
	func(s *writeProbeState) []writeProbeCall {
		var calls []writeProbeCall
		for _, caller := range []string{"human", "agent"} {
			for _, typ := range probePostTypes {
				body := map[string]any{"title": "Route probe write " + typ, "description": probeDescription, "tags": []string{"probe"}}
				if typ != "post" {
					body["type"] = typ
				}
				c := call("POST /v1/posts", caller, "/v1/posts", j(body))
				key := caller + " " + typ
				c.keep = func(s *writeProbeState, v jsonValue) bool {
					s.created[key] = v.first([]string{"data", "id"}, []string{"id"})
					return s.created[key] != ""
				}
				calls = append(calls, c)
			}
		}
		for i, typ := range probePostTypes {
			id := s.posts[typ]
			calls = append(calls,
				call("PATCH /v1/posts/{id}", postAuthor(i), "/v1/posts/"+id, j(map[string]any{"title": "Route probe edited " + typ})),
				call("PATCH /v1/posts/{id}", postAuthor(i+1), "/v1/posts/"+id, j(map[string]any{"title": "Not the author's edit"})))
		}
		// The problem holds a reply migrated from a succeeded approach, so it may be solved.
		calls = append(calls, call("PATCH /v1/posts/{id}", "agent", "/v1/posts/"+s.posts["problem"], j(map[string]any{"status": "solved"})))
		for _, typ := range probePostTypes {
			id := s.posts[typ]
			for _, caller := range []string{"human", "agent"} {
				calls = append(calls,
					call("POST /v1/posts/{id}/vote", caller, "/v1/posts/"+id+"/vote", j(map[string]any{"direction": "up"})),
					call("POST /v1/posts/{id}/replies", caller, "/v1/posts/"+id+"/replies",
						j(map[string]any{"body": "A reply the write route probe posts."})),
					call("POST /v1/users/me/bookmarks", caller, "/v1/users/me/bookmarks", j(map[string]any{"post_id": id})))
			}
			calls = append(calls,
				call("POST /v1/posts/{id}/view", "anonymous", "/v1/posts/"+id+"/view", `{}`),
				call("POST /v1/posts/{id}/view", "human", "/v1/posts/"+id+"/view", `{}`))
		}
		for i, id := range s.replies {
			calls = append(calls,
				call("POST /v1/posts/{id}/replies", "agent", "/v1/posts/"+s.posts[probePostTypes[i/2]]+"/replies",
					j(map[string]any{"body": "A threaded probe reply.", "parent_reply_id": id})),
				call("PATCH /v1/replies/{id}", replyAuthor(i), "/v1/replies/"+id, j(map[string]any{"body": "An edited probe reply."})),
				call("POST /v1/replies/{id}/vote", replyAuthor(i+1), "/v1/replies/"+id+"/vote", j(map[string]any{"direction": "up"})))
		}
		for _, typ := range probePostTypes {
			for _, caller := range []string{"human", "agent"} {
				calls = append(calls, call("DELETE /v1/users/me/bookmarks/{id}", caller, "/v1/users/me/bookmarks/"+s.posts[typ], ""))
			}
		}
		targets := [][2]string{
			{"post", s.posts["post"]}, {"reply", s.replies[0]}, {"reply", s.replies[1]}, {"approach", s.replies[1]},
			{"answer", s.replies[3]}, {"response", s.replies[5]}, {"comment", s.replies[7]},
		}
		for _, target := range targets {
			calls = append(calls, call("POST /v1/reports", "human", "/v1/reports",
				j(map[string]any{"target_type": target[0], "target_id": target[1], "reason": "spam", "details": "route probe"})))
		}
		return calls
	},
	func(s *writeProbeState) []writeProbeCall {
		rpc := func(method string, params map[string]any) string {
			return j(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
		}
		calls := []writeProbeCall{
			call("POST /v1/mcp", "anonymous", "/v1/mcp", rpc("initialize", map[string]any{})),
			call("POST /v1/mcp", "anonymous", "/v1/mcp", rpc("tools/list", map[string]any{})),
			call("POST /v1/mcp", "agent", "/v1/mcp", rpc("tools/call", map[string]any{
				"name": "solvr_search", "arguments": map[string]any{"query": "probe"}})),
			call("POST /v1/mcp", "agent", "/v1/mcp", rpc("tools/call", map[string]any{
				"name": "solvr_search", "arguments": map[string]any{"query": "probe", "type": "problem"}})),
			call("POST /v1/analytics/funnel", "anonymous", "/v1/analytics/funnel",
				j(map[string]any{"flow_id": "route-probe-flow", "event": "room_viewed"})),
			call("POST /v1/analytics/funnel", "human", "/v1/analytics/funnel",
				j(map[string]any{"flow_id": "route-probe-flow", "event": "starter_prompt_copied"})),
		}
		for _, typ := range probePostTypes {
			calls = append(calls, call("POST /v1/mcp", "agent", "/v1/mcp", rpc("tools/call", map[string]any{
				"name": "solvr_get", "arguments": map[string]any{"id": s.posts[typ]}})))
		}
		return calls
	},

	// Legacy writes: served by the legacy repositories until idx 52 retires them.
	func(s *writeProbeState) []writeProbeCall {
		approach := call("POST /v1/problems/{id}/approaches", "agent", "/v1/problems/"+s.posts["problem"]+"/approaches",
			j(map[string]any{"angle": "A legacy probe angle", "method": "A legacy probe method"}))
		approach.keep = keepID(func(s *writeProbeState) *string { return &s.approachID }, "id")
		answer := call("POST /v1/questions/{id}/answers", "agent", "/v1/questions/"+s.posts["question"]+"/answers",
			j(map[string]any{"content": "A legacy probe answer, long enough to be accepted by the legacy route."}))
		answer.keep = keepID(func(s *writeProbeState) *string { return &s.answerID }, "id")
		response := call("POST /v1/ideas/{id}/responses", "human", "/v1/ideas/"+s.posts["idea"]+"/responses",
			j(map[string]any{"content": "A legacy probe response to the idea.", "response_type": "build"}))
		response.keep = keepID(func(s *writeProbeState) *string { return &s.responseID }, "id")
		calls := []writeProbeCall{approach, answer, response}
		for _, typ := range []string{"problem", "question", "idea"} {
			calls = append(calls, call("POST /v1/"+typ+"s", "agent", "/v1/"+typ+"s",
				j(map[string]any{"title": "Legacy probe " + typ, "description": probeDescription})))
		}
		return calls
	},
	func(s *writeProbeState) []writeProbeCall {
		comment := call("POST /v1/answers/{id}/comments", "human", "/v1/answers/"+or(s.answerID)+"/comments",
			j(map[string]any{"content": "A legacy probe comment."}))
		comment.keep = keepID(func(s *writeProbeState) *string { return &s.commentID }, "id")
		return []writeProbeCall{
			call("POST /v1/approaches/{id}/progress", "agent", "/v1/approaches/"+or(s.approachID)+"/progress",
				j(map[string]any{"content": "Legacy probe progress."})),
			call("PATCH /v1/approaches/{id}", "agent", "/v1/approaches/"+or(s.approachID), j(map[string]any{"status": "working"})),
			call("POST /v1/approaches/{id}/verify", "agent", "/v1/approaches/"+or(s.approachID)+"/verify", `{}`),
			call("PATCH /v1/answers/{id}", "agent", "/v1/answers/"+or(s.answerID),
				j(map[string]any{"content": "An edited legacy probe answer, still long enough."})),
			call("POST /v1/answers/{id}/vote", "human", "/v1/answers/"+or(s.answerID)+"/vote", j(map[string]any{"direction": "up"})),
			comment,
			call("POST /v1/approaches/{id}/comments", "human", "/v1/approaches/"+or(s.approachID)+"/comments",
				j(map[string]any{"content": "A legacy probe comment."})),
			call("POST /v1/responses/{id}/comments", "agent", "/v1/responses/"+or(s.responseID)+"/comments",
				j(map[string]any{"content": "A legacy probe comment."})),
			call("POST /v1/posts/{id}/comments", "agent", "/v1/posts/"+s.posts["post"]+"/comments",
				j(map[string]any{"content": "A legacy probe comment."})),
			call("POST /v1/questions/{id}/accept/{aid}", "human", "/v1/questions/"+s.posts["question"]+"/accept/"+or(s.answerID), `{}`),
			call("POST /v1/ideas/{id}/evolve", "agent", "/v1/ideas/"+s.posts["idea"]+"/evolve",
				j(map[string]any{"evolved_post_id": or(s.created["agent problem"])})),
		}
	},
	func(s *writeProbeState) []writeProbeCall {
		return []writeProbeCall{
			call("DELETE /v1/comments/{id}", "human", "/v1/comments/"+or(s.commentID), ""),
			call("DELETE /v1/answers/{id}", "agent", "/v1/answers/"+or(s.answerID), ""),
		}
	},

	// Rooms, the room transport and its adapters.
	func(s *writeProbeState) []writeProbeCall {
		room := call("POST /v1/rooms", "human", "/v1/rooms", j(map[string]any{"display_name": "Probe write room"}))
		room.keep = keepID(func(s *writeProbeState) *string { return &s.roomSlug2 }, "slug")
		handshake := call("POST /v1/rooms/{slug}/handshake", "agent", "/v1/rooms/"+s.roomSlug+"/handshake", `{}`)
		handshake.keep = keepID(func(s *writeProbeState) *string { return &s.roomToken }, "room_token")
		return []writeProbeCall{
			room,
			call("POST /v1/rooms", "agent", "/v1/rooms", j(map[string]any{"display_name": "Probe agent room"})),
			call("PATCH /v1/rooms/{slug}", "human", "/v1/rooms/"+s.roomSlug, j(map[string]any{"description": "route probe room"})),
			call("POST /v1/rooms/{slug}/members", "human", "/v1/rooms/"+s.roomSlug+"/members", j(map[string]any{"agent_id": or(s.agent2ID)})),
			handshake,
		}
	},
	func(s *writeProbeState) []writeProbeCall {
		r := "/r/" + s.roomSlug
		me := j(map[string]any{"agent_name": s.agentID})
		message := call("POST /r/{slug}/message", "room", r+"/message", j(map[string]any{"agent_name": s.agentID, "content": "A probe message."}))
		message.keep = keepID(func(s *writeProbeState) *string { return &s.messageID }, "id")
		claim := j(map[string]any{"key": "probe-task", "agent": s.agentID, "ttl_seconds": 60})
		return []writeProbeCall{
			call("POST /r/{slug}/join", "room", r+"/join", me),
			call("POST /r/{slug}/heartbeat", "room", r+"/heartbeat", me),
			message,
			call("POST /r/{slug}/claim", "room", r+"/claim", claim),
			call("POST /r/{slug}/claim/renew", "room", r+"/claim/renew", claim),
			call("POST /r/{slug}/claim/release", "room", r+"/claim/release", claim),
			call("POST /r/{slug}/events", "room", r+"/events",
				j(map[string]any{"type": "status", "actor": s.agentID, "payload": map[string]any{"state": "probe"}})),
			call("POST /v1/rooms/{slug}/entries", "human", "/v1/rooms/"+s.roomSlug+"/entries", j(map[string]any{"body": "A probe entry."})),
			call("POST /v1/rooms/{slug}/entries", "agent", "/v1/rooms/"+s.roomSlug+"/entries", j(map[string]any{"body": "A probe entry."})),
			call("POST /v1/rooms/{slug}/messages", "human", "/v1/rooms/"+s.roomSlug+"/messages", j(map[string]any{"content": "A probe message."})),
			call("POST /v1/rooms/{slug}/stream-ticket", "human", "/v1/rooms/"+s.roomSlug+"/stream-ticket", `{}`),
			call("POST /v1/rooms/{slug}/stream-ticket", "agent", "/v1/rooms/"+s.roomSlug+"/stream-ticket", `{}`),
		}
	},
	func(s *writeProbeState) []writeProbeCall {
		r := "/r/" + s.roomSlug
		save := call("POST /v1/rooms/{slug}/save-as-post", "human", "/v1/rooms/"+s.roomSlug+"/save-as-post",
			j(map[string]any{"title": "Route probe room outcome", "summary": probeDescription}))
		save.keep = keepID(func(s *writeProbeState) *string { return &s.savedPostID }, "id")
		return []writeProbeCall{
			call("POST /r/{slug}/messages/{id}/pin", "room", r+"/messages/"+or(s.messageID)+"/pin", `{}`),
			call("DELETE /r/{slug}/messages/{id}/pin", "room", r+"/messages/"+or(s.messageID)+"/pin", ""),
			save,
		}
	},
	func(s *writeProbeState) []writeProbeCall {
		room := "/v1/rooms/" + s.roomSlug
		return []writeProbeCall{
			call("POST /v1/rooms/{slug}/posts/{postID}/publish", "human", room+"/posts/"+or(s.savedPostID)+"/publish", `{}`),
			call("POST /r/{slug}/leave", "room", "/r/"+s.roomSlug+"/leave", j(map[string]any{"agent_name": s.agentID})),
			call("POST /v1/rooms/{slug}/archive", "human", room+"/archive", `{}`),
			call("POST /v1/rooms/{slug}/reopen", "human", room+"/reopen", `{}`),
			call("DELETE /v1/rooms/{slug}/members/{agent_id}/token", "human", room+"/members/"+s.agentID+"/token", ""),
			call("DELETE /v1/rooms/{slug}/members/{agent_id}", "human", room+"/members/"+or(s.agent2ID), ""),
			call("DELETE /v1/rooms/{slug}", "human", "/v1/rooms/"+or(s.roomSlug2), ""),
		}
	},

	// Blog, storage and operator routes.
	func(s *writeProbeState) []writeProbeCall {
		blog := call("POST /v1/blog", "human", "/v1/blog", j(map[string]any{
			"title": "Route probe blog post", "body": probeDescription, "status": "published", "tags": []string{"probe"}}))
		blog.keep = keepID(func(s *writeProbeState) *string { return &s.blogSlug }, "slug")
		incident := "/admin/incidents/route-probe-incident"
		return []writeProbeCall{
			blog,
			call("POST /admin/query", "admin", "/admin/query", j(map[string]any{"query": "SELECT COUNT(*) FROM posts"})),
			call("POST /admin/jobs/translation/run", "admin", "/admin/jobs/translation/run", `{}`),
			call("POST /admin/email/broadcast", "admin", "/admin/email/broadcast",
				j(map[string]any{"subject": "route probe", "body_html": "<p>route probe</p>", "dry_run": true})),
			call("POST /admin/incidents", "admin", "/admin/incidents", j(map[string]any{
				"id": "route-probe-incident", "title": "Route probe incident", "severity": "minor", "affected_services": []string{"api"}})),
			call("PATCH /admin/incidents/{id}", "admin", incident, j(map[string]any{"status": "monitoring"})),
			call("POST /admin/incidents/{id}/updates", "admin", incident+"/updates",
				j(map[string]any{"status": "resolved", "message": "route probe update"})),
			// Anti-abuse operator routes: dry runs, so the probe bans and unpins nothing.
			call("POST /admin/bans", "admin", "/admin/bans", j(map[string]any{
				"account_type": "agent", "account_id": s.agent2ID, "reason": "route probe", "dry_run": true})),
			call("POST /admin/ipfs/unpin", "admin", "/admin/ipfs/unpin", j(map[string]any{
				"cids": []string{"QmbrPqJC7j1mVmPsjbjyzhU2qq8FFeYyMxox7N5Ztsdzip"}, "dry_run": true})),
			call("POST /admin/ipfs/gc", "admin", "/admin/ipfs/gc", `{}`),
		}
	},
	func(s *writeProbeState) []writeProbeCall {
		blog := "/v1/blog/" + or(s.blogSlug)
		return []writeProbeCall{
			call("PATCH /v1/blog/{slug}", "human", blog, j(map[string]any{"excerpt": "route probe excerpt"})),
			call("POST /v1/blog/{slug}/vote", "agent", blog+"/vote", j(map[string]any{"direction": "up"})),
			call("POST /v1/blog/{slug}/view", "anonymous", blog+"/view", `{}`),
			call("DELETE /v1/blog/{slug}", "human", blog, ""),
		}
	},

	// Notifications, then the deletions.
	func(s *writeProbeState) []writeProbeCall {
		return []writeProbeCall{
			call("POST /v1/notifications/{id}/read", "human", "/v1/notifications/"+s.humanNotification+"/read", `{}`),
			call("POST /v1/notifications/{id}/read", "agent", "/v1/notifications/"+s.agentNotification+"/read", `{}`),
			call("POST /v1/notifications/read-all", "human", "/v1/notifications/read-all", `{}`),
			call("POST /v1/notifications/read-all", "agent", "/v1/notifications/read-all", `{}`),
			call("DELETE /v1/notifications/{id}", "human", "/v1/notifications/"+s.humanNotification, ""),
			call("DELETE /v1/notifications/{id}", "agent", "/v1/notifications/"+s.agentNotification, ""),
			call("DELETE /v1/notifications", "human", "/v1/notifications", ""),
			call("DELETE /v1/notifications", "agent", "/v1/notifications", ""),
			call("DELETE /v1/follow", "human", "/v1/follow", j(map[string]any{"target_type": "agent", "target_id": s.agentID})),
			call("DELETE /v1/follow", "agent", "/v1/follow", j(map[string]any{"target_type": "human", "target_id": s.userID})),
		}
	},
	func(s *writeProbeState) []writeProbeCall {
		var calls []writeProbeCall
		for i, id := range s.replies {
			calls = append(calls, call("DELETE /v1/replies/{id}", replyAuthor(i), "/v1/replies/"+id, ""))
		}
		for i, typ := range probePostTypes {
			calls = append(calls, call("DELETE /v1/posts/{id}", postAuthor(i), "/v1/posts/"+s.posts[typ], ""))
			for _, caller := range []string{"human", "agent"} {
				calls = append(calls, call("DELETE /v1/posts/{id}", caller, "/v1/posts/"+or(s.created[caller+" "+typ]), ""))
			}
		}
		rotate := call("POST /v1/agents/{id}/api-key", "human", "/v1/agents/"+or(s.agent2ID)+"/api-key", `{}`)
		rotate.keep = func(s *writeProbeState, v jsonValue) bool {
			s.agent2Key = v.first([]string{"data", "api_key"}, []string{"api_key"})
			return s.agent2Key != ""
		}
		return append(calls,
			call("DELETE /v1/users/me/api-keys/{id}", "human", "/v1/users/me/api-keys/"+or(s.apiKeyID), ""),
			rotate)
	},
	// Storage runs last: a pin finishes in the background seconds later (the closed IPFS
	// port is retried), and its database work must land in the drain or a canonical call's
	// window, never in a legacy-family call's, where a missing table passes as expected.
	func(s *writeProbeState) []writeProbeCall {
		pin := call("POST /v1/pins", "agent", "/v1/pins", j(map[string]any{"cid": probePinCID, "name": "probe pin"}))
		pin.keep = keepID(func(s *writeProbeState) *string { return &s.pinID }, "requestid")
		upload, uploadType := probeUpload()
		add := call("POST /v1/add", "agent", "/v1/add", upload)
		add.contentType = uploadType
		return []writeProbeCall{
			call("POST /v1/agents/me/checkpoints", "agent", "/v1/agents/me/checkpoints",
				j(map[string]any{"cid": probeCID, "name": "probe checkpoint"})),
			pin, add,
			call("POST /v1/pins", "human", "/v1/pins", j(map[string]any{"cid": probePinCID, "name": "probe pin"})),
		}
	},
	func(s *writeProbeState) []writeProbeCall {
		return []writeProbeCall{
			call("DELETE /v1/pins/{requestid}", "agent", "/v1/pins/"+or(s.pinID), ""),
			call("DELETE /v1/agents/me", "agent2", "/v1/agents/me", ""),
			call("DELETE /admin/agents/{id}", "admin", "/admin/agents/"+or(s.agent2ID), ""),
			call("DELETE /v1/me", "human2", "/v1/me", ""),
			call("DELETE /admin/users/{id}", "admin", "/admin/users/"+or(s.human2ID), ""),
		}
	},
}
