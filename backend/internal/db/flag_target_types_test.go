package db

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

var checkLiteralRe = regexp.MustCompile(`'([a-z_]+)'`)

// TestFlagTargetTypes_MatchTheFlagsCheckConstraint binds the API's accepted flag target
// types to the live flags_target_type_check: 000109 added 'reply', and narrowing the check at
// legacy cleanup (check:flags.flags_target_type_check) must narrow the API list with it.
// Runs only when DATABASE_URL points at a local test database (setupTestDB skips otherwise).
func TestFlagTargetTypes_MatchTheFlagsCheckConstraint(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	var def string
	err := pool.QueryRow(context.Background(), `
		SELECT pg_get_constraintdef(con.oid) FROM pg_constraint con
		JOIN pg_class rel ON rel.oid = con.conrelid
		WHERE rel.relname = 'flags' AND con.conname = 'flags_target_type_check'`).Scan(&def)
	if err != nil {
		t.Fatalf("read flags_target_type_check: %v", err)
	}

	var allowed []string
	for _, m := range checkLiteralRe.FindAllStringSubmatch(def, -1) {
		allowed = append(allowed, m[1])
	}
	accepted := append([]string(nil), models.ValidFlagTargetTypes...)
	sort.Strings(allowed)
	sort.Strings(accepted)
	if strings.Join(allowed, ",") != strings.Join(accepted, ",") {
		t.Errorf("flag target types accepted by the API %v differ from flags_target_type_check %v (%s)",
			accepted, allowed, def)
	}
}
