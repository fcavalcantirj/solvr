package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// The signed links of Solvr's emails (SPEC.md 16.6). POST is RFC 8058's one-click unsubscribe:
// it accepts any form body, needs no credential, is idempotent and answers 200, and a bad token
// answers 403. GET never writes: it answers 303 to the solvr.dev page, which asks before it
// POSTs. The token is HMAC-SHA256(email, JWT_SECRET), so every link already sent keeps working.

// mockUnsubscribeRepo records UnsubscribeByEmail calls.
type mockUnsubscribeRepo struct {
	err   error
	calls []string
}

func (m *mockUnsubscribeRepo) UnsubscribeByEmail(ctx context.Context, email string) error {
	m.calls = append(m.calls, email)
	return m.err
}

const unsubTestKey = "test-secret-key"

func unsubscribeQuery(email, token string) string {
	return "?email=" + url.QueryEscape(email) + "&token=" + url.QueryEscape(token)
}

func oneClickPost(h *UnsubscribeHandler, query string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/email/unsubscribe"+query,
		strings.NewReader("List-Unsubscribe=One-Click"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.Unsubscribe(rr, req)
	return rr
}

func pageGet(h *UnsubscribeHandler, query string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	h.ShowPage(rr, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/email/unsubscribe"+query, http.NoBody))
	return rr
}

func TestUnsubscribe_OneClickPostUnsubscribes(t *testing.T) {
	repo := &mockUnsubscribeRepo{}
	h := NewUnsubscribeHandler(repo, unsubTestKey)
	email := "alice@example.com"

	rr := oneClickPost(h, unsubscribeQuery(email, GenerateUnsubscribeToken(email, unsubTestKey)))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "unsubscribed" {
		t.Errorf("status = %q, want unsubscribed", resp["status"])
	}
	if len(repo.calls) != 1 || repo.calls[0] != email {
		t.Errorf("repo calls = %v, want [%s]", repo.calls, email)
	}
}

func TestUnsubscribe_OneClickPostIsIdempotent(t *testing.T) {
	repo := &mockUnsubscribeRepo{}
	h := NewUnsubscribeHandler(repo, unsubTestKey)
	email := "alice@example.com"
	q := unsubscribeQuery(email, GenerateUnsubscribeToken(email, unsubTestKey))

	for i := 0; i < 2; i++ {
		if rr := oneClickPost(h, q); rr.Code != http.StatusOK {
			t.Fatalf("call %d: status = %d, want 200", i+1, rr.Code)
		}
	}
}

func TestUnsubscribe_OneClickPostAcceptsAnyFormBody(t *testing.T) {
	repo := &mockUnsubscribeRepo{}
	h := NewUnsubscribeHandler(repo, unsubTestKey)
	email := "alice@example.com"
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost,
		"/v1/email/unsubscribe"+unsubscribeQuery(email, GenerateUnsubscribeToken(email, unsubTestKey)), http.NoBody)
	rr := httptest.NewRecorder()
	h.Unsubscribe(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("an empty body: status = %d, want 200", rr.Code)
	}
}

func TestUnsubscribe_OneClickPostRefusesABadToken(t *testing.T) {
	repo := &mockUnsubscribeRepo{}
	h := NewUnsubscribeHandler(repo, unsubTestKey)

	rr := oneClickPost(h, unsubscribeQuery("alice@example.com", "bad-token"))

	if rr.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rr.Code)
	}
	if len(repo.calls) != 0 {
		t.Error("a bad token must not unsubscribe anyone")
	}
}

func TestUnsubscribe_OneClickPostRefusesATokenOfAnotherKey(t *testing.T) {
	repo := &mockUnsubscribeRepo{}
	h := NewUnsubscribeHandler(repo, "production-key")
	email := "alice@example.com"

	rr := oneClickPost(h, unsubscribeQuery(email, GenerateUnsubscribeToken(email, "different-key")))

	if rr.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rr.Code)
	}
}

func TestUnsubscribe_AnEncodedPlusSurvives(t *testing.T) {
	repo := &mockUnsubscribeRepo{}
	h := NewUnsubscribeHandler(repo, unsubTestKey)
	email := "alice+tag@example.com"

	rr := oneClickPost(h, unsubscribeQuery(email, GenerateUnsubscribeToken(email, unsubTestKey)))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	if len(repo.calls) != 1 || repo.calls[0] != email {
		t.Errorf("repo calls = %v, want [%s]", repo.calls, email)
	}
}

// Links sent before this release put the address in the query unencoded, so a '+' in it reads
// as a space. The token is the HMAC of the real address, so the handler accepts the '+' form
// only when its token matches: an old link of a plus address still works, and nothing else
// gets through.
func TestUnsubscribe_AnOldLinkOfAPlusAddressStillWorks(t *testing.T) {
	repo := &mockUnsubscribeRepo{}
	h := NewUnsubscribeHandler(repo, unsubTestKey)
	email := "alice+tag@example.com"
	oldLink := "?email=" + email + "&token=" + GenerateUnsubscribeToken(email, unsubTestKey)

	rr := oneClickPost(h, oldLink)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	if len(repo.calls) != 1 || repo.calls[0] != email {
		t.Errorf("repo calls = %v, want [%s]", repo.calls, email)
	}

	other := &mockUnsubscribeRepo{}
	h = NewUnsubscribeHandler(other, unsubTestKey)
	if rr := oneClickPost(h, "?email=alice%20tag@example.com&token=bad"); rr.Code != http.StatusForbidden {
		t.Errorf("a space with a bad token: status = %d, want 403", rr.Code)
	}
}

func TestUnsubscribe_MissingParams(t *testing.T) {
	h := NewUnsubscribeHandler(&mockUnsubscribeRepo{}, unsubTestKey)
	for _, q := range []string{"", "?email=a@b.com", "?token=t"} {
		if rr := oneClickPost(h, q); rr.Code != http.StatusBadRequest {
			t.Errorf("POST %q: status = %d, want 400", q, rr.Code)
		}
		if rr := pageGet(h, q); rr.Code != http.StatusBadRequest {
			t.Errorf("GET %q: status = %d, want 400", q, rr.Code)
		}
	}
}

func TestUnsubscribe_RepoError(t *testing.T) {
	h := NewUnsubscribeHandler(&mockUnsubscribeRepo{err: fmt.Errorf("db error")}, unsubTestKey)
	email := "alice@example.com"

	rr := oneClickPost(h, unsubscribeQuery(email, GenerateUnsubscribeToken(email, unsubTestKey)))

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}

// GET answers 303 to the page with the address and token encoded and the address masked for
// display, and writes nothing: a link scanner or a prefetch never unsubscribes anyone.
func TestUnsubscribe_GetRedirectsToThePageAndWritesNothing(t *testing.T) {
	repo := &mockUnsubscribeRepo{}
	h := NewUnsubscribeHandler(repo, unsubTestKey)
	email := "alice+tag@example.com"
	token := GenerateUnsubscribeToken(email, unsubTestKey)

	for _, tok := range []string{token, "bad"} {
		rr := pageGet(h, unsubscribeQuery(email, tok))
		if rr.Code != http.StatusSeeOther {
			t.Fatalf("token %q: status = %d, want 303", tok, rr.Code)
		}
		loc := rr.Header().Get("Location")
		if !strings.HasPrefix(loc, "https://solvr.dev/email/unsubscribe?email=alice%2Btag%40example.com&") {
			t.Errorf("Location = %q, want the solvr.dev page with the address encoded first", loc)
		}
		u, err := url.Parse(loc)
		if err != nil {
			t.Fatalf("Location %q: %v", loc, err)
		}
		if got := u.Query().Get("token"); got != tok {
			t.Errorf("token = %q, want %q", got, tok)
		}
		if got := u.Query().Get("masked"); got != "a***@example.com" {
			t.Errorf("masked = %q, want a***@example.com", got)
		}
	}
	if len(repo.calls) != 0 {
		t.Errorf("GET wrote: repo calls = %v", repo.calls)
	}
}

func TestUnsubscribeURLs_EncodeTheAddress(t *testing.T) {
	email := "alice+tag@example.com"
	token := GenerateUnsubscribeToken(email, unsubTestKey)

	page := unsubscribePageURL(email, token)
	if !strings.HasPrefix(page, "https://solvr.dev/email/unsubscribe?email=alice%2Btag%40example.com&") {
		t.Errorf("page URL = %q", page)
	}
	oneClick := oneClickUnsubscribeURL(email, token)
	if want := "https://api.solvr.dev/v1/email/unsubscribe?email=alice%2Btag%40example.com&token=" + token; oneClick != want {
		t.Errorf("one-click URL = %q, want %q", oneClick, want)
	}
}

func TestMaskEmail(t *testing.T) {
	for in, want := range map[string]string{
		"alice@example.com": "a***@example.com",
		"a@b.c":             "a***@b.c",
		"no-at-sign":        "***",
		"@example.com":      "***@example.com",
	} {
		if got := maskEmail(in); got != want {
			t.Errorf("maskEmail(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGenerateUnsubscribeToken_Deterministic(t *testing.T) {
	token1 := GenerateUnsubscribeToken("alice@example.com", "key")
	token2 := GenerateUnsubscribeToken("alice@example.com", "key")
	if token1 != token2 {
		t.Error("same email+key should produce same token")
	}

	token3 := GenerateUnsubscribeToken("bob@example.com", "key")
	if token1 == token3 {
		t.Error("different emails should produce different tokens")
	}
}
