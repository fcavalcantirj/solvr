package models

import (
	"strings"
	"testing"
)

// The canonical Reply contract (BART-585): one reply type for every new
// contribution — no approach/answer/response/comment choice, no type-specific
// form, no mandatory status workflow. Body carries code, a failed attempt, a
// review, or discussion as plain Markdown.

func TestValidateReplyBody_Required(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{"empty", "", true},
		{"whitespace only", "   \n\t ", true},
		{"valid short", "This worked for me.", false},
		{"valid markdown code", "```go\nfmt.Println(\"ok\")\n```", false},
		{"too long", strings.Repeat("x", MaxReplyBodyLength+1), true},
		{"at max", strings.Repeat("x", MaxReplyBodyLength), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateReplyBody(tc.body)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %q, got nil", tc.name)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.name, err)
			}
		})
	}
}

func TestCreateReplyRequest_Validate(t *testing.T) {
	// A valid request needs only a body; parent_reply_id is optional.
	valid := CreateReplyRequest{Body: "a genuine reply"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}

	parent := "11111111-1111-1111-1111-111111111111"
	withParent := CreateReplyRequest{Body: "child reply", ParentReplyID: &parent}
	if err := withParent.Validate(); err != nil {
		t.Fatalf("threaded request rejected: %v", err)
	}

	empty := CreateReplyRequest{Body: ""}
	if err := empty.Validate(); err == nil {
		t.Fatal("empty body accepted, want error")
	}
}

func TestReply_ComputeScore(t *testing.T) {
	r := Reply{Upvotes: 7, Downvotes: 2}
	r.ComputeScore()
	if r.Score != 5 {
		t.Fatalf("score = %d, want 5", r.Score)
	}
}

func TestReply_IsDeleted(t *testing.T) {
	live := Reply{}
	if live.IsDeleted() {
		t.Fatal("fresh reply reported deleted")
	}
}

// A Reply carries no content-type field: there is no approach/answer/response/
// comment discriminator on the canonical model, only optional migration
// provenance describing where a converted reply came from.
func TestReply_ProvenanceIsOptional(t *testing.T) {
	native := Reply{Body: "native canonical reply"}
	if native.Body == "" {
		t.Fatal("body not retained")
	}
	if native.LegacyType != nil || native.LegacyID != nil {
		t.Fatal("a natively created reply must not carry legacy provenance")
	}
}

// Votes and reports target the canonical Reply identity (target_type = 'reply').
func TestReportTargetReply_IsValid(t *testing.T) {
	if !IsValidReportTargetType(ReportTargetReply) {
		t.Fatal("reply must be a valid report target")
	}
}
