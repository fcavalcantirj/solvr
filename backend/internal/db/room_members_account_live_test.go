package db_test

import (
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// idx 75 step 3: an established stream is re-authorized against the account behind it. A
// soft-deleted account no longer authenticates, so it must not keep reading a room through a
// stream that was authorized before it was deleted (SPEC Part 20.2 / 20.3).
func TestRoomMemberRepository_AccountLive(t *testing.T) {
	ctx, pool := openAuthorityPool(t)
	repo := db.NewRoomMemberRepository(pool)

	insertTestAgentForMembers(ctx, t, pool, "agent_account_live")
	user := insertAuthorityUser(ctx, t, pool, "accountlive")

	cases := []struct {
		name      string
		actorType string
		id        string
		want      bool
	}{
		{"a live agent", "agent", "agent_account_live", true},
		{"a live human", "human", user.String(), true},
		{"an agent that never existed", "agent", "agent_account_never", false},
		{"a human that never existed", "human", "0f6a5d5e-8a52-4c8e-9b7a-000000000000", false},
		{"a malformed human id", "human", "not-a-uuid", false},
		{"an unknown actor type", "robot", "agent_account_live", false},
	}
	for _, tc := range cases {
		got, err := repo.AccountLive(ctx, tc.actorType, tc.id)
		if err != nil {
			t.Fatalf("%s: AccountLive: %v", tc.name, err)
		}
		if got != tc.want {
			t.Errorf("%s: AccountLive = %v, want %v", tc.name, got, tc.want)
		}
	}

	softDeleteAgent(ctx, t, pool, "agent_account_live")
	softDeleteUser(ctx, t, pool, user)
	for name, c := range map[string][2]string{"a deleted agent": {"agent", "agent_account_live"}, "a deleted human": {"human", user.String()}} {
		got, err := repo.AccountLive(ctx, c[0], c[1])
		if err != nil || got {
			t.Errorf("%s: AccountLive = %v, %v, want false", name, got, err)
		}
	}
}
