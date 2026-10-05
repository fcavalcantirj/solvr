package db

import (
	"fmt"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// userPublicName is models.PublicDisplayName written in SQL for the users row a query aliases
// alias (SPEC.md 2.8): the display name, or the username when the display name is blank or
// contains an e-mail address. Every query that reads a person's name for a public answer reads
// it through this expression; the stored display name is never rewritten. A LEFT JOIN that
// finds no user still reads NULL, so a COALESCE around it falls through as before.
func userPublicName(alias string) string {
	return fmt.Sprintf(`(CASE WHEN %[1]s.display_name !~ '%[2]s' OR %[1]s.display_name ~ '%[3]s'
		THEN %[1]s.username ELSE %[1]s.display_name END)`,
		alias, models.VisibleTextPattern, models.EmailAddressPattern)
}
