package db

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Step 6 of the legacy-dependency migration, enforced where schema cleanup is written: an up
// migration that removes legacy runtime storage is allowed only once LegacyCleanupBlockers
// is empty for the real inventory.

// LegacySchemaCleanup is one up-migration statement that removes legacy runtime storage.
type LegacySchemaCleanup struct {
	Migration string
	Line      int
	What      string
}

func (c LegacySchemaCleanup) String() string {
	return fmt.Sprintf("%s:%d %s", c.Migration, c.Line, c.What)
}

var (
	sqlLineCommentRe  = regexp.MustCompile(`--[^\n]*`)
	sqlBlockCommentRe = regexp.MustCompile(`(?s)/\*.*?\*/`)
	dropTableRe       = regexp.MustCompile(`(?is)\bDROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?(.+)`)
	dropFunctionRe    = regexp.MustCompile(`(?is)\bDROP\s+FUNCTION\s+(?:IF\s+EXISTS\s+)?(.+)`)
	alterTableRe      = regexp.MustCompile(`(?is)\bALTER\s+TABLE\s+(?:IF\s+EXISTS\s+)?(?:ONLY\s+)?([\w."]+)\s+(.+)`)
	renameTableRe     = regexp.MustCompile(`(?i)^RENAME\s+TO\b`)
	setSchemaRe       = regexp.MustCompile(`(?i)^SET\s+SCHEMA\s+"?(\w+)"?`)
	dropColumnRe      = regexp.MustCompile(`(?i)\bDROP\s+(?:COLUMN\s+)?(?:IF\s+EXISTS\s+)?"?(\w+)"?`)
	checkDefinitionRe = regexp.MustCompile(`(?i)\bCONSTRAINT\s+"?(\w+)"?\s+CHECK\s*\(`)
	dropBehaviorRe    = regexp.MustCompile(`(?i)\s+(CASCADE|RESTRICT)\s*$`)
	// A name carrying a legacy table as an underscore-delimited part (hybrid_search_answers).
	legacyNamePartRe = regexp.MustCompile(`(^|_)` + legacyTableWord + `(_|$)`)
)

// Words that follow DROP inside ALTER TABLE without naming a column.
var dropNonColumn = map[string]bool{
	"constraint": true, "default": true, "not": true, "identity": true, "expression": true,
}

// sqlStatement is one ;-terminated statement and the line it starts on.
type sqlStatement struct {
	line int
	text string
}

// splitSQL blanks comments (keeping line breaks) and splits on semicolons. A function body
// is split too; its parts are still scanned, which only matters for removals written there.
func splitSQL(src string) []sqlStatement {
	blank := func(s string) string {
		return strings.Map(func(r rune) rune {
			if r == '\n' {
				return r
			}
			return ' '
		}, s)
	}
	src = sqlBlockCommentRe.ReplaceAllStringFunc(src, blank)
	src = sqlLineCommentRe.ReplaceAllStringFunc(src, blank)
	var out []sqlStatement
	line := 1
	for part := range strings.SplitSeq(src, ";") {
		lead := len(part) - len(strings.TrimLeft(part, " \t\r\n"))
		if text := strings.TrimSpace(part); text != "" {
			out = append(out, sqlStatement{line: line + strings.Count(part[:lead], "\n"), text: text})
		}
		line += strings.Count(part, "\n")
	}
	return out
}

// sqlName strips the schema, quotes and case from an identifier.
func sqlName(s string) string {
	s = strings.Trim(strings.TrimSpace(s), `"`)
	if i := strings.LastIndex(s, "."); i >= 0 {
		s = strings.Trim(s[i+1:], `"`)
	}
	return strings.ToLower(s)
}

// sqlNameList splits "a, public.b CASCADE" or "f(int, text), g(int)" into names.
func sqlNameList(s string) []string {
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch {
		case r == '(':
			depth++
		case r == ')':
			depth--
		case depth == 0:
			b.WriteRune(r)
		}
	}
	var names []string
	for n := range strings.SplitSeq(dropBehaviorRe.ReplaceAllString(b.String(), ""), ",") {
		if n = sqlName(n); n != "" {
			names = append(names, n)
		}
	}
	return names
}

// parenBody returns the text inside the parenthesis that opens at s[open].
func parenBody(s string, open int) string {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[open+1 : i]
			}
		}
	}
	return s[open+1:]
}

func legacyLiterals(s string) map[string]bool {
	set := map[string]bool{}
	for _, m := range legacyTypeLiteralRe.FindAllStringSubmatch(s, -1) {
		set[strings.ToLower(m[1])] = true
	}
	return set
}

// ScanLegacySchemaCleanup reads the up migrations under dir in order and returns every
// statement that removes legacy runtime storage: a legacy table dropped, renamed away or moved
// out of public (ALTER TABLE ... SET SCHEMA), a column dropped from a legacy table or carrying
// a legacy identity, a function whose name carries a legacy table or is in legacyFunctions
// dropped, and a named check constraint redefined so it no longer admits a legacy type its
// previous definition admitted.
func ScanLegacySchemaCleanup(dir string, legacyFunctions []string) ([]LegacySchemaCleanup, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return nil, fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(files)
	isLegacyTable := map[string]bool{}
	for _, t := range LegacyTables {
		isLegacyTable[t] = true
	}
	isLegacyFunction := map[string]bool{}
	for _, f := range legacyFunctions {
		isLegacyFunction[strings.ToLower(f)] = true
	}
	checks := map[string]map[string]bool{}
	var out []LegacySchemaCleanup
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read migration: %w", err)
		}
		name := filepath.Base(file)
		for _, st := range splitSQL(string(src)) {
			add := func(format string, args ...any) {
				out = append(out, LegacySchemaCleanup{Migration: name, Line: st.line, What: fmt.Sprintf(format, args...)})
			}
			if m := dropTableRe.FindStringSubmatch(st.text); m != nil {
				for _, t := range sqlNameList(m[1]) {
					if isLegacyTable[t] {
						add("drops table %s", t)
					}
				}
			}
			if m := dropFunctionRe.FindStringSubmatch(st.text); m != nil {
				for _, f := range sqlNameList(m[1]) {
					if isLegacyFunction[f] || legacyNamePartRe.MatchString(f) {
						add("drops function %s", f)
					}
				}
			}
			if m := alterTableRe.FindStringSubmatch(st.text); m != nil {
				table, actions := sqlName(m[1]), strings.TrimSpace(m[2])
				if isLegacyTable[table] && renameTableRe.MatchString(actions) {
					add("renames table %s", table)
				}
				if m := setSchemaRe.FindStringSubmatch(actions); m != nil && isLegacyTable[table] && !strings.EqualFold(m[1], "public") {
					add("moves table %s out of public", table)
				}
				for _, c := range dropColumnRe.FindAllStringSubmatch(actions, -1) {
					col := strings.ToLower(c[1])
					if !dropNonColumn[col] && (isLegacyTable[table] || legacyColumnRe.MatchString(col)) {
						add("drops column %s.%s", table, col)
					}
				}
			}
			for _, loc := range checkDefinitionRe.FindAllStringSubmatchIndex(st.text, -1) {
				constraint := strings.ToLower(st.text[loc[2]:loc[3]])
				admitted := legacyLiterals(parenBody(st.text, loc[1]-1))
				var lost []string
				for lit := range checks[constraint] {
					if !admitted[lit] {
						lost = append(lost, "'"+lit+"'")
					}
				}
				if len(lost) > 0 {
					sort.Strings(lost)
					add("narrows %s (no longer admits %s)", constraint, strings.Join(lost, ", "))
				}
				checks[constraint] = admitted
			}
		}
	}
	return out, nil
}

// registryDependencies lists every registry key as a dependency, so a pending disposition
// blocks cleanup even when no scan rediscovers its object (the catalog needs a database).
func registryDependencies(registry map[string]LegacyDependencyDisposition) []LegacyDependency {
	var deps []LegacyDependency
	for key := range registry {
		deps = append(deps, LegacyDependency{Kind: legacyKeyKind(key), Key: key})
	}
	sort.Slice(deps, func(i, j int) bool { return deps[i].Key < deps[j].Key })
	return deps
}

// LegacySchemaCleanupGate returns an error naming each cleanup statement and each blocker
// when the up migrations under dir remove legacy runtime storage while LegacyCleanupBlockers
// over deps and every registry entry is not empty; nil otherwise.
func LegacySchemaCleanupGate(dir string, deps []LegacyDependency, registry map[string]LegacyDependencyDisposition) error {
	var functions []string
	for key := range registry {
		if legacyKeyKind(key) == "function" {
			functions = append(functions, strings.TrimPrefix(key, "function:"))
		}
	}
	cleanups, err := ScanLegacySchemaCleanup(dir, functions)
	if err != nil {
		return err
	}
	if len(cleanups) == 0 {
		return nil
	}
	all := append(append([]LegacyDependency{}, deps...), registryDependencies(registry)...)
	blockers := LegacyCleanupBlockers(all, registry)
	if len(blockers) == 0 {
		return nil
	}
	lines := []string{"legacy schema cleanup before every dependency has a done disposition", "cleanup:"}
	for _, c := range cleanups {
		lines = append(lines, "  "+c.String())
	}
	lines = append(lines, "blockers:")
	for _, k := range blockers {
		lines = append(lines, "  "+k)
	}
	return errors.New(strings.Join(lines, "\n"))
}
