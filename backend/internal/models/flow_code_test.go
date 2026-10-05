package models

import (
	"strings"
	"testing"
)

// The flow code is the connection-funnel flow id in a form short enough to ride on the
// skill link of the copied sentence (SPEC.md 25.6). ValidFlowCode is the one format check
// the connect contract, the funnel ingest and room creation all use.

func TestFlowCode_TheAlphabetHasNoLookAlikeCharacters(t *testing.T) {
	if got := len(FlowCodeAlphabet); got != 31 {
		t.Fatalf("alphabet has %d characters, want 31", got)
	}
	for _, banned := range "ilo01" {
		if strings.ContainsRune(FlowCodeAlphabet, banned) {
			t.Errorf("alphabet contains the look-alike %q", banned)
		}
	}
	seen := map[rune]bool{}
	for _, c := range FlowCodeAlphabet {
		if seen[c] {
			t.Errorf("alphabet repeats %q", c)
		}
		seen[c] = true
		if (c < 'a' || c > 'z') && (c < '2' || c > '9') {
			t.Errorf("alphabet holds %q, which is not a lowercase letter or a digit 2-9", c)
		}
	}
	if FlowCodeLength != 8 {
		t.Fatalf("a flow code is %d characters, want 8", FlowCodeLength)
	}
}

// The format check and the alphabet are one definition: every character of the alphabet
// is accepted in every position, and nothing outside it is.
func TestValidFlowCode_AcceptsExactlyTheAlphabet(t *testing.T) {
	for _, c := range FlowCodeAlphabet {
		code := strings.Repeat(string(c), FlowCodeLength)
		if !ValidFlowCode(code) {
			t.Errorf("ValidFlowCode(%q) = false, want true", code)
		}
	}
	for c := rune(0); c < 256; c++ {
		if strings.ContainsRune(FlowCodeAlphabet, c) {
			continue
		}
		code := "abcdefg" + string(c)
		if ValidFlowCode(code) {
			t.Errorf("ValidFlowCode(%q) = true for a character outside the alphabet", code)
		}
	}
}

func TestValidFlowCode_RefusesEverythingElse(t *testing.T) {
	for _, bad := range []string{
		"",                           // absent
		"abcdefg",                    // too short
		"abcdefghj",                  // too long
		"ABCDEFGH",                   // upper case
		"abcdefgi",                   // i
		"abcdefgl",                   // l
		"abcdefgo",                   // o
		"abcdefg0",                   // 0
		"abcdefg1",                   // 1
		"abcdefgh\n",                 // a trailing newline is not part of a code
		" abcdefgh",                  // nor is a space
		"abcd-fgh",                   // nor punctuation
		"abcdefgh?x=1",               // nor a query
		"f_0123456789abcdef01234567", // the former flow id format
		"k7m2p9xq k7m2p9xq",
	} {
		if ValidFlowCode(bad) {
			t.Errorf("ValidFlowCode(%q) = true, want false", bad)
		}
	}
	if !ValidFlowCode("k7m2p9xq") {
		t.Error(`ValidFlowCode("k7m2p9xq") = false, want true`)
	}
}
