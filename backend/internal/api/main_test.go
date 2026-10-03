package api

import (
	"os"
	"testing"
)

// TestMain lifts the per-IP agent registration limit for this package's tests (spec.json
// idx 79). Every test server here registers its agents from 127.0.0.1 through one router, and
// the N-agent room tests alone register more than DefaultRegistrationsPerIPPerHour (20) — the
// limit is not their subject. A value already set in the environment wins, and the tests whose
// subject IS the limit (router_registration_limit_test.go) set it themselves with t.Setenv.
func TestMain(m *testing.M) {
	if os.Getenv(registrationLimitEnv) == "" {
		_ = os.Setenv(registrationLimitEnv, "100000")
	}
	os.Exit(m.Run())
}
