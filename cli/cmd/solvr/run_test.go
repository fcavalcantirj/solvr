package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestRun_UsageErrorsPointAtHelpAndPrintNoUsageDump(t *testing.T) {
	isolateHome(t)
	res := runCLI(t, "replies")
	if res.code != 1 {
		t.Errorf("exit %d, want 1", res.code)
	}
	if !strings.Contains(res.stderr, "accepts 1 arg") || !strings.Contains(res.stderr, "Run 'solvr replies --help' for usage.") {
		t.Errorf("stderr %q", res.stderr)
	}
	if res.stdout != "" {
		t.Errorf("stdout %q, want nothing", res.stdout)
	}
}

func TestRun_AnAPIErrorIsReportedOnceWithItsRequestIDAndNoUsage(t *testing.T) {
	srv := newRoomServer(t, func(w http.ResponseWriter, r *http.Request) {
		answerJSON(w, 404, `{"error":{"code":"NOT_FOUND","message":"post not found","request_id":"req-42"}}`)
	})
	writeConfig(t, srv.URL+"/v1", "")

	res := runCLI(t, "get", "missing")
	if res.code != 1 || res.stdout != "" {
		t.Fatalf("exit %d, stdout %q", res.code, res.stdout)
	}
	if strings.Count(res.stderr, "NOT_FOUND: post not found") != 1 || !strings.Contains(res.stderr, "request id: req-42") {
		t.Errorf("stderr %q", res.stderr)
	}
	if strings.Contains(res.stderr, "Usage:") {
		t.Errorf("an API error printed the usage: %q", res.stderr)
	}
}

func TestRun_AJSONErrorThatIsNotTheAPIsAnswerIsStillJSON(t *testing.T) {
	srv := newRoomServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(502)
		_, _ = w.Write([]byte("<html>bad gateway</html>"))
	})
	writeConfig(t, srv.URL+"/v1", "")

	human := runCLI(t, "get", "p1")
	if human.code != 1 || !strings.Contains(human.stderr, "API returned status 502") {
		t.Errorf("exit %d, stderr %q", human.code, human.stderr)
	}
	asJSON := runCLI(t, "get", "p1", "--json")
	if asJSON.code != 1 || jsonDiff([]byte(`{"error":{"code":"HTTP_502","message":"API returned status 502"}}`), []byte(asJSON.stderr), true) != "" {
		t.Errorf("--json exit %d, stderr %q", asJSON.code, asJSON.stderr)
	}
}

func TestSearch_SendsOnlyTheOptionsGivenAndNoEmptyQuery(t *testing.T) {
	srv := newRoomServer(t, func(w http.ResponseWriter, r *http.Request) {
		answerJSON(w, 400, `{"error":{"code":"VALIDATION_ERROR","message":"search query 'q' is required"}}`)
	})
	writeConfig(t, srv.URL+"/v1", "")

	runCLI(t, "search", "")
	runCLI(t, "search", "race", "--page", "2", "--sort", "votes")
	sent := srv.requests()
	if len(sent[0].query) != 0 {
		t.Errorf("an empty query sent %v, want no parameter", sent[0].query)
	}
	if q := sent[1].query; len(q) != 3 || q.Get("q") != "race" || q.Get("page") != "2" || q.Get("sort") != "votes" {
		t.Errorf("search sent %v", q)
	}
}

func TestPathParametersAreEscaped(t *testing.T) {
	srv := newRoomServer(t, func(w http.ResponseWriter, r *http.Request) {
		answerJSON(w, 200, `{"data":{"id":"x"},"meta":{"total":0,"has_more":false}}`)
	})
	writeConfig(t, srv.URL+"/v1", "solvr_agent_key")

	runCLI(t, "get", "a b/c")
	runCLI(t, "replies", "a b/c")
	runCLI(t, "reply", "a b/c", "--body", "hi")
	runCLI(t, "get-reply", "a b/c")
	runCLI(t, "update-reply", "a b/c", "--if-match", `"1"`, "--body", "hi")
	want := []string{"/v1/posts/a%20b%2Fc", "/v1/posts/a%20b%2Fc/replies", "/v1/posts/a%20b%2Fc/replies", "/v1/replies/a%20b%2Fc", "/v1/replies/a%20b%2Fc"}
	sent := srv.requests()
	if len(sent) != len(want) {
		t.Fatalf("sent %d requests, want %d", len(sent), len(want))
	}
	for i, path := range want {
		if sent[i].path != path {
			t.Errorf("request %d: path %s, want %s", i, sent[i].path, path)
		}
	}
}

func TestGetReply_ShowsTheETagAndHowToEdit(t *testing.T) {
	srv := newRoomServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"17"`)
		answerJSON(w, 200, `{"data":{"id":"r1","post_id":"p1","author_type":"agent","author_id":"agent_a","body":"Use a mutex"}}`)
	})
	writeConfig(t, srv.URL+"/v1", "")

	res := runCLI(t, "get-reply", "r1")
	for _, want := range []string{"r1", "agent agent_a", "on post p1", "Use a mutex", `ETag: "17"`, `solvr update-reply r1 --if-match '"17"' --body`} {
		if !strings.Contains(res.stdout, want) {
			t.Errorf("output %q, want %q", res.stdout, want)
		}
	}
	if auth := srv.requests()[0].header.Get("Authorization"); auth != "" {
		t.Errorf("an anonymous read presented %q", auth)
	}
}

func TestUpdateReply_AnEmptyIfMatchSendsNoHeaderAndSurfacesTheAPIsAnswer(t *testing.T) {
	srv := newRoomServer(t, func(w http.ResponseWriter, r *http.Request) {
		answerJSON(w, 428, `{"error":{"code":"PRECONDITION_REQUIRED","message":"send If-Match","request_id":"req-7"}}`)
	})
	writeConfig(t, srv.URL+"/v1", "solvr_agent_key")

	res := runCLI(t, "update-reply", "r1", "--if-match", "", "--body", "new")
	if _, sent := srv.requests()[0].header["If-Match"]; sent {
		t.Error("an empty --if-match sent an If-Match header")
	}
	if res.code != 1 || !strings.Contains(res.stderr, "PRECONDITION_REQUIRED: send If-Match") || !strings.Contains(res.stderr, "req-7") {
		t.Errorf("exit %d, stderr %q", res.code, res.stderr)
	}
	if body := string(srv.requests()[0].body); body != `{"body":"new"}` {
		t.Errorf("update-reply sent %s", body)
	}
}

func TestUpdateReply_RequiresIfMatchAndBody(t *testing.T) {
	srv := newRoomServer(t, nil)
	writeConfig(t, srv.URL+"/v1", "solvr_agent_key")
	for _, args := range [][]string{
		{"update-reply", "r1", "--body", "new"},
		{"update-reply", "r1", "--if-match", `"1"`},
	} {
		if res := runCLI(t, args...); res.code != 1 || !strings.Contains(res.stderr, "required flag") {
			t.Errorf("solvr %s: exit %d, stderr %q", strings.Join(args, " "), res.code, res.stderr)
		}
	}
	if n := len(srv.requests()); n != 0 {
		t.Errorf("sent %d requests, want 0", n)
	}
}

func TestConfigGet_MasksRoomTokens(t *testing.T) {
	home := writeConfig(t, "http://127.0.0.1:9/v1", "")
	if res := runCLI(t, "config", "set", "room-token.plan-room", "solvr_rt_0123456789abcdef"); res.code != 0 {
		t.Fatalf("config set: %s", res.stderr)
	}
	res := runCLI(t, "config", "get")
	if strings.Contains(res.stdout, "solvr_rt_0123456789abcdef") || !strings.Contains(res.stdout, "room-token.plan-room=solvr_****cdef") {
		t.Errorf("config get printed %q", res.stdout)
	}
	if !strings.Contains(readConfigFile(t, home), "solvr_rt_0123456789abcdef") {
		t.Error("the token was not saved")
	}
}
