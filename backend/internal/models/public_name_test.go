package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// publicNameCases pins the public-name rule (SPEC.md 2.8): a stored display name, the
// username, and the name a public answer serves. The SQL form of the rule (db.userPublicName)
// is checked against PublicDisplayName itself in the db package, so the two cannot drift.
var publicNameCases = []struct {
	DisplayName, Username, Want string
}{
	{"Ana Lima", "ana", "Ana Lima"},
	{"felipe@example.com", "felipe", "felipe"},
	{"  Felipe.C@Example.Com.br  ", "felipe", "felipe"},
	{"Felipe <felipe@example.com>", "felipe", "felipe"},
	{"", "ana", "ana"},
	{"   ", "ana", "ana"},
	{"\t\n", "ana", "ana"},
	{"ana@home", "ana", "ana@home"},               // no domain dot: not an address
	{"@ana", "ana", "@ana"},                       // a handle, not an address
	{"Ana @ Solvr.dev", "ana", "Ana @ Solvr.dev"}, // spaces around the @
	{"Ação Café", "acao", "Ação Café"},
}

func TestPublicDisplayName_ServesTheUsernameForAnEmailOrABlankName(t *testing.T) {
	for _, c := range publicNameCases {
		assert.Equal(t, c.Want, PublicDisplayName(c.DisplayName, c.Username), "display name %q", c.DisplayName)
	}
}

func TestContainsEmailAddress(t *testing.T) {
	assert.True(t, ContainsEmailAddress("felipe@example.com"))
	assert.True(t, ContainsEmailAddress("write to a.b+c@mail.example.org today"))
	assert.False(t, ContainsEmailAddress("Felipe Cavalcanti"))
	assert.False(t, ContainsEmailAddress("ana@home"))
	assert.False(t, ContainsEmailAddress(""))
}

// The agent as anyone may read it (SPEC.md 2.7): its contact e-mail is for the agent itself
// and the human who claimed it, never a public answer. Nothing else is removed.
func TestAgent_Public_DropsOnlyTheEmail(t *testing.T) {
	human := "human-1"
	a := Agent{ID: "agent_one", DisplayName: "Agent One", Email: "owner@example.com", HumanID: &human, Bio: "bio", Model: "m"}
	p := a.Public()
	assert.Empty(t, p.Email)
	assert.Equal(t, "owner@example.com", a.Email, "the agent itself is unchanged")
	p.Email = a.Email
	assert.Equal(t, a, p, "every other field is kept")
}
