package models

import "regexp"

// A person's public name (SPEC.md 2.8). An e-mail address is never served as a name: sign-up
// and PATCH /v1/me refuse one, and a name stored before that rule is served as the username.
// The rule has two forms, this one for names a handler serialises and db.userPublicName for
// names a query reads; both are written with the patterns below.

// EmailAddressPattern finds an e-mail address in a text: a run without white space or "@", an
// "@", and a domain with a dot. It uses only POSIX classes and brackets, no backslash and no
// flag, so Go's regexp and PostgreSQL's ~ read it alike.
const EmailAddressPattern = `[^[:space:]@]+@[^[:space:]@]+[.][^[:space:]@]+`

// VisibleTextPattern finds a character that is not white space: a text it does not match is
// blank.
const VisibleTextPattern = `[^[:space:]]`

var (
	emailAddress = regexp.MustCompile(EmailAddressPattern)
	visibleText  = regexp.MustCompile(VisibleTextPattern)
)

// ContainsEmailAddress reports whether s holds an e-mail address.
func ContainsEmailAddress(s string) bool {
	return emailAddress.MatchString(s)
}

// PublicDisplayName is the name a public answer gives a person: their display name, or their
// username when the display name is blank or contains an e-mail address.
func PublicDisplayName(displayName, username string) string {
	if !visibleText.MatchString(displayName) || ContainsEmailAddress(displayName) {
		return username
	}
	return displayName
}
