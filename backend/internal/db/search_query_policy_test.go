package db

import "testing"

// The public-query filtering policy decides whether a RECORDED search term may
// be published on a public page. These are pure-function tests: no database,
// no skipping, they run everywhere.

func TestPublicQueryText_ScopeGates(t *testing.T) {
	yes, no := true, false

	if got := PublicQueryText("postgres connection pool", nil); got != RejectedUnknownScope {
		t.Fatalf("a search recorded before scope was instrumented must be unknown_scope, got %q", got)
	}
	if got := PublicQueryText("postgres connection pool", &no); got != RejectedProtectedScope {
		t.Fatalf("a family-scoped search may have matched protected content, got %q", got)
	}
	if got := PublicQueryText("postgres connection pool", &yes); got != PublicQueryOK {
		t.Fatalf("an ordinary public-scope search must publish, got %q", got)
	}
}

func TestPublicQueryText_RejectsCredentialShapedText(t *testing.T) {
	yes := true
	cases := map[string]string{
		"openai key":      "sk-abcd1234abcd1234abcd1234",
		"solvr agent key": "solvr_sk_9f2c1ab4d77e4f0a91bb",
		"aws access key":  "AKIAIOSFODNN7EXAMPLE",
		"github token":    "ghp_16C7e42F292c6912E7710c838347Ae178B4a",
		"slack token":     "xoxb-1234567890-abcdefghij",
		"jwt":             "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NSJ9.abcdef",
		"private key":     "-----BEGIN RSA PRIVATE KEY-----",
		"assignment":      "DATABASE_PASSWORD=hunter2",
		"spaced assign":   "api_key : 9f2c1ab4d77e",
		"bearer":          "Authorization Bearer abcdefghijklmnopqrstuvwxyz012345",
		"url credentials": "postgres://solvr:s3cr3t@db.internal:5432/solvr",
		"long opaque run": "a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f6a7b8",
	}
	for name, q := range cases {
		if got := PublicQueryText(q, &yes); got != RejectedCredential {
			t.Errorf("%s: expected credential rejection for %q, got %q", name, q, got)
		}
	}
}

func TestPublicQueryText_KeepsOrdinaryTechnicalTerms(t *testing.T) {
	yes := true
	// The risk is a PASTED SECRET, not the topic. Blocking the words would
	// delete the knowledge base's most common searches.
	keep := []string{
		"how to rotate an api key",
		"jwt token expired error",
		"reset postgres password",
		"bearer auth middleware 401",
		"aws s3 access denied",
		"connection pool exhausted",
	}
	for _, q := range keep {
		if got := PublicQueryText(q, &yes); got != PublicQueryOK {
			t.Errorf("expected %q to stay publishable, got %q", q, got)
		}
	}
}

func TestPublicQueryText_RejectsPersonalData(t *testing.T) {
	yes := true
	cases := map[string]string{
		"email":       "why does felipe.cavalcanti.rj@gmail.com bounce",
		"phone":       "call +55 21 99876 5432 about the outage",
		"long digits": "order 12345678901234 failed",
		"card":        "4111 1111 1111 1111 declined",
	}
	for name, q := range cases {
		if got := PublicQueryText(q, &yes); got != RejectedPersonal {
			t.Errorf("%s: expected personal rejection for %q, got %q", name, q, got)
		}
	}
}

func TestPublicQueryText_KeepsOrdinaryNumbers(t *testing.T) {
	yes := true
	keep := []string{
		"error 500 on deploy",
		"postgres 17 upgrade",
		"http 429 rate limit",
		"go 1.23 generics",
	}
	for _, q := range keep {
		if got := PublicQueryText(q, &yes); got != PublicQueryOK {
			t.Errorf("expected %q to stay publishable, got %q", q, got)
		}
	}
}

func TestPublicQueryText_RejectsModeratedTerms(t *testing.T) {
	yes := true
	for _, q := range []string{"free porn sites", "XXX videos download"} {
		if got := PublicQueryText(q, &yes); got != RejectedModerated {
			t.Errorf("expected moderated rejection for %q, got %q", q, got)
		}
	}
	// Word boundaries: a moderated term must not swallow a legitimate word.
	if got := PublicQueryText("pornography detection model", &yes); got != RejectedModerated {
		t.Errorf("expected the prefixed form to be moderated too, got %q", got)
	}
	if got := PublicQueryText("xxhash collision rate", &yes); got != PublicQueryOK {
		t.Errorf("xxhash must not match the xxx term, got %q", got)
	}
}

func TestPublicQueryText_RejectsEmptyAndOverlongTerms(t *testing.T) {
	yes := true
	if got := PublicQueryText("   ", &yes); got != RejectedUnpublishable {
		t.Errorf("blank term must not be published, got %q", got)
	}
	long := make([]byte, 160)
	for i := range long {
		long[i] = 'a'
		if i%8 == 7 {
			long[i] = ' '
		}
	}
	if got := PublicQueryText(string(long), &yes); got != RejectedUnpublishable {
		t.Errorf("an overlong term is a paste, not a search term, got %q", got)
	}
}

func TestModerationBlocklistIsExtendable(t *testing.T) {
	// The operator owns the list; the default is a floor, not the whole policy.
	terms := moderationTerms([]string{"porn", "xxx"}, "acme-internal, Widget ")
	if !containsTerm(terms, "acme-internal") || !containsTerm(terms, "widget") {
		t.Fatalf("configured terms must be normalised and added: %v", terms)
	}
	if !containsTerm(terms, "porn") {
		t.Fatalf("configured terms must extend the default, not replace it: %v", terms)
	}
}

func containsTerm(terms []string, want string) bool {
	for _, t := range terms {
		if t == want {
			return true
		}
	}
	return false
}
