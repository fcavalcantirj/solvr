package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Anti-abuse W4 on the real router: every sign-in and registration entry point refuses a
// banned identity with 403 ACCOUNT_SUSPENDED (the OAuth callbacks, which redirect, are covered
// in services/oauth_callback_suspended_db_test.go).

func gateSuffix() string { return strings.ReplaceAll(uuid.NewString(), "-", "")[:10] }

// banRow puts one identity on the ban list for the duration of the test.
func banRow(t *testing.T, pool *db.Pool, kind, provider, value string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO banned_identities (kind, provider, value, reason) VALUES ($1, $2, $3, 'router test')`, kind, provider, value)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM banned_identities WHERE kind = $1 AND value = $2`, kind, value) //nolint:errcheck
	})
}

func requireSuspended(t *testing.T, answer statusContractAnswer, err error, what string) {
	t.Helper()
	require.NoError(t, err, what)
	require.Equal(t, http.StatusForbidden, answer.status, "%s: %s", what, answer.body)
	require.Equal(t, "ACCOUNT_SUSPENDED", answer.code, what)
}

func TestIdentityGate_RegisterAndLoginRefuseBannedAndTombstonedEmails(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	s := gateSuffix()
	banned := "banned_" + s + "@test.solvr.dev"
	banRow(t, pool, "email", "", banned)

	register := func(email, username string) (statusContractAnswer, error) {
		return callStatusContract(http.DefaultClient, http.MethodPost, ts.URL+"/v1/auth/register", "",
			fmt.Sprintf(`{"email":%q,"password":"Str0ng-Passw0rd","username":%q}`, email, username))
	}
	answer, err := register(strings.ToUpper(banned), "gate_r_"+s)
	requireSuspended(t, answer, err, "register with a banned email")

	// A tombstoned account with no ban row still keeps its email out.
	tombID, _ := createLiveTestUser(t, pool, models.UserRoleUser)
	var tombEmail string
	require.NoError(t, pool.QueryRow(context.Background(),
		`UPDATE users SET deleted_at = NOW() WHERE id = $1 RETURNING email`, tombID).Scan(&tombEmail))
	answer, err = register(tombEmail, "gate_t_"+s)
	requireSuspended(t, answer, err, "register with a tombstoned account's email")

	answer, err = callStatusContract(http.DefaultClient, http.MethodPost, ts.URL+"/v1/auth/login", "",
		fmt.Sprintf(`{"email":%q,"password":"Str0ng-Passw0rd"}`, banned))
	requireSuspended(t, answer, err, "login with a banned email")

	// A fresh email still registers.
	fresh := "fresh_" + s + "@test.solvr.dev"
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM users WHERE email = $1", fresh) }) //nolint:errcheck
	answer, err = register(fresh, "gate_f_"+s)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, answer.status, answer.body)
}

func TestIdentityGate_LoginCodeExchangeRefusesABannedEmail(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	userID, _ := createLiveTestUser(t, pool, models.UserRoleUser)
	var email string
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT email FROM users WHERE id = $1`, userID).Scan(&email))
	code, err := db.NewOAuthLoginCodeRepository(pool).Issue(context.Background(), userID, time.Minute)
	require.NoError(t, err)
	banRow(t, pool, "email", "", strings.ToLower(email))

	answer, err := callStatusContract(http.DefaultClient, http.MethodPost, ts.URL+"/v1/auth/oauth/exchange", "",
		fmt.Sprintf(`{"login_code":%q}`, code))
	requireSuspended(t, answer, err, "login code of a banned email")
}

func TestIdentityGate_AgentRegistrationRefusesBannedIdAndEmail(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	s := gateSuffix()
	banRow(t, pool, "agent_id", "", "agent_gate_"+s)
	banRow(t, pool, "email", "", "agentowner_"+s+"@test.solvr.dev")
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM agents WHERE id LIKE $1", "agent_gate%"+s) //nolint:errcheck
	})

	answer, err := callStatusContract(http.DefaultClient, http.MethodPost, ts.URL+"/v1/agents/register", "",
		fmt.Sprintf(`{"name":"gate_%s","description":"banned name"}`, s))
	requireSuspended(t, answer, err, "register a banned agent id")

	answer, err = callStatusContract(http.DefaultClient, http.MethodPost, ts.URL+"/v1/agents/register", "",
		fmt.Sprintf(`{"name":"gate_ok_%s","description":"banned email","email":"AgentOwner_%s@test.solvr.dev"}`, s, s))
	requireSuspended(t, answer, err, "register an agent with a banned email")
}

func TestIdentityGate_ClaimFlowsRefuseBannedIdentities(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	_, jwt := createLiveTestUser(t, pool, models.UserRoleUser)

	// POST /v1/agents/me/claim by a banned agent.
	bannedID, bannedKey := uniqueTestAgent(t, ts, pool)
	banRow(t, pool, "agent_id", "", bannedID)
	answer, err := callStatusContract(http.DefaultClient, http.MethodPost, ts.URL+"/v1/agents/me/claim", bannedKey, "")
	requireSuspended(t, answer, err, "claim link for a banned agent")

	// POST /v1/agents/claim of an agent banned after it issued its link.
	laterID, laterKey := uniqueTestAgent(t, ts, pool)
	status, out := claimGenerate(t, ts, laterKey)
	require.Equal(t, http.StatusCreated, status, "%v", out)
	banRow(t, pool, "agent_id", "", laterID)
	token, _ := out["token"].(string)
	answer, err = callStatusContract(http.DefaultClient, http.MethodPost, ts.URL+"/v1/agents/claim", jwt, fmt.Sprintf(`{"token":%q}`, token))
	requireSuspended(t, answer, err, "claiming a banned agent")

	// POST /v1/agents/claim by a human whose email is banned.
	_, cleanKey := uniqueTestAgent(t, ts, pool)
	status, out = claimGenerate(t, ts, cleanKey)
	require.Equal(t, http.StatusCreated, status, "%v", out)
	humanID, humanJWT := createLiveTestUser(t, pool, models.UserRoleUser)
	var humanEmail string
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT email FROM users WHERE id = $1`, humanID).Scan(&humanEmail))
	banRow(t, pool, "email", "", strings.ToLower(humanEmail))
	token, _ = out["token"].(string)
	answer, err = callStatusContract(http.DefaultClient, http.MethodPost, ts.URL+"/v1/agents/claim", humanJWT, fmt.Sprintf(`{"token":%q}`, token))
	requireSuspended(t, answer, err, "a banned human claiming an agent")
}

func TestIdentityGate_RoomHandshakeRefusesABannedAgent(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	_, ownerKey := uniqueTestAgent(t, ts, pool)
	slug, _ := createTestRoomWithAgentKey(t, ts, ownerKey)
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM rooms WHERE slug = $1", slug) }) //nolint:errcheck

	bannedID, bannedKey := uniqueTestAgent(t, ts, pool)
	banRow(t, pool, "agent_id", "", bannedID)
	answer, err := callStatusContract(http.DefaultClient, http.MethodPost, ts.URL+"/v1/rooms/"+slug+"/handshake", bannedKey, "{}")
	requireSuspended(t, answer, err, "handshake by a banned agent")
}

func TestAdminBans_TombstonesAndBansThroughTheRouter(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "anti-abuse-test-admin-key")
	ts, _, pool := newStatusContractServer(t)
	agentID, agentKey := uniqueTestAgent(t, ts, pool)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM banned_identities WHERE value = $1", agentID) //nolint:errcheck
	})
	ban := func(key string, dryRun bool) statusContractAnswer {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/admin/bans",
			strings.NewReader(fmt.Sprintf(`{"account_type":"agent","account_id":%q,"reason":"router test","dry_run":%t}`, agentID, dryRun)))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		if key != "" {
			req.Header.Set("X-Admin-API-Key", key)
		}
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		return statusContractAnswer{status: resp.StatusCode}
	}

	require.Equal(t, http.StatusUnauthorized, ban("", false).status, "no admin key")
	require.Equal(t, http.StatusOK, ban("anti-abuse-test-admin-key", true).status, "dry run")
	me, err := callStatusContract(http.DefaultClient, http.MethodGet, ts.URL+"/v1/me", agentKey, "")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, me.status, "a dry run changes nothing: %s", me.body)

	require.Equal(t, http.StatusOK, ban("anti-abuse-test-admin-key", false).status, "ban")
	me, err = callStatusContract(http.DefaultClient, http.MethodGet, ts.URL+"/v1/me", agentKey, "")
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, me.status, "a banned agent's key no longer authenticates: %s", me.body)
	answer, err := callStatusContract(http.DefaultClient, http.MethodPost, ts.URL+"/v1/agents/register", "",
		fmt.Sprintf(`{"name":%q,"description":"back again"}`, strings.TrimPrefix(agentID, "agent_")))
	requireSuspended(t, answer, err, "re-registering a banned agent's name")
}
