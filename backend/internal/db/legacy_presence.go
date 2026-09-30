package db

import (
	"context"
	"fmt"
)

// legacyContributionTablesPresent reports whether the legacy contribution tables still exist.
// The anti-abuse checks (repeat gate, create limit, contribution moderation) read them only
// while they do: legacy cleanup drops them (LegacyTables), and the canonical routes that run
// these checks must keep working after it, without a hidden legacy dependency.
func legacyContributionTablesPresent(ctx context.Context, pool *Pool) (bool, error) {
	var present bool
	err := pool.QueryRow(ctx, `
		SELECT to_regclass('answers') IS NOT NULL AND to_regclass('approaches') IS NOT NULL
		   AND to_regclass('responses') IS NOT NULL AND to_regclass('comments') IS NOT NULL
		   AND to_regclass('progress_notes') IS NOT NULL`).Scan(&present)
	if err != nil {
		return false, fmt.Errorf("check legacy tables: %w", err)
	}
	return present, nil
}
