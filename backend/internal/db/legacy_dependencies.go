package db

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Legacy-dependency inventory for the move of every dependent relationship and background
// job to the canonical posts/replies model. Discovery finds what still touches the legacy
// contribution tables or the legacy post/target types; LegacyDependencyDispositions records
// the decision for each; LegacyCleanupBlockers says whether schema cleanup may proceed.

// LegacyTables are the contribution tables the canonical Reply model replaces.
var LegacyTables = []string{
	"approaches", "answers", "responses", "comments", "approach_relationships", "progress_notes",
}

// LegacyDependency is one object, query, job or declared relationship tied to the legacy
// model. Key is "kind:name". Owner, when set, is the key of the legacy table the object
// belongs to (an index or constraint on it): it drops with that table and inherits the
// table's disposition unless the registry lists the object on its own.
type LegacyDependency struct {
	Kind   string
	Key    string
	Owner  string
	Detail string
}

// LegacyDependencyAction is the decision for one dependency.
type LegacyDependencyAction string

const (
	// LegacyActionRemap: data is re-pointed at the canonical identity (post or reply).
	LegacyActionRemap LegacyDependencyAction = "remap"
	// LegacyActionRefactor: the object or code is rewritten against posts/replies.
	LegacyActionRefactor LegacyDependencyAction = "refactor"
	// LegacyActionRetire: the behavior has no canonical future and is removed.
	LegacyActionRetire LegacyDependencyAction = "retire"
	// LegacyActionKeep: reviewed; it does not depend on the legacy model and stays.
	LegacyActionKeep LegacyDependencyAction = "keep"
)

// LegacyDependencyDisposition is the recorded decision. Done is set only once the
// canonical replacement (or the retirement) is implemented and verified; keep needs no Done.
type LegacyDependencyDisposition struct {
	Action LegacyDependencyAction
	Done   bool
	Note   string
}

func legacyKeyKind(key string) string {
	kind, _, ok := strings.Cut(key, ":")
	if !ok {
		return ""
	}
	return kind
}

// LegacyCleanupBlockers lists, sorted, every dependency that stops schema cleanup: one with
// no explicit disposition (its own, or its owner's), or one whose non-keep disposition is
// not done. Cleanup is complete only when this is empty.
func LegacyCleanupBlockers(deps []LegacyDependency, registry map[string]LegacyDependencyDisposition) []string {
	seen := map[string]bool{}
	var out []string
	for _, d := range deps {
		if seen[d.Key] {
			continue
		}
		seen[d.Key] = true
		disp, ok := registry[d.Key]
		if !ok && d.Owner != "" {
			disp, ok = registry[d.Owner]
		}
		switch {
		case !ok:
			out = append(out, d.Key+" (no disposition)")
		case disp.Action != LegacyActionKeep && !disp.Done:
			out = append(out, fmt.Sprintf("%s (%s pending)", d.Key, disp.Action))
		}
	}
	sort.Strings(out)
	return out
}

var (
	legacyTableWord = `(approaches|answers|responses|comments|approach_relationships|progress_notes)`
	// A legacy table named in SQL position (not merely in prose or a Go identifier).
	legacySQLTableRe = regexp.MustCompile(`(?i)\b(?:FROM|JOIN|INTO|UPDATE|TABLE)\s+` + legacyTableWord + `\b`)
	// A legacy post type or contribution target type compared in SQL.
	legacySQLTypeRe = regexp.MustCompile(`(?i)\btype\s*(?:=|IN|<>|!=)\s*\(?\s*'(problem|question|idea|approach|answer|response|comment)'`)
	// A quoted legacy type literal inside a catalog definition (checks, partial indexes).
	legacyTypeLiteralRe = regexp.MustCompile(`'(problem|question|idea|approach|answer|response|comment)'`)
	legacyTableRefRe    = regexp.MustCompile(`(?i)\b` + legacyTableWord + `\b`)
	// A column on another table that carries a legacy contribution identity.
	legacyColumnRe = regexp.MustCompile(`(^|_)(approach|approaches|answer|answers|comment|comments)(_|$)|^response_id$`)
	scheduledJobRe = regexp.MustCompile(`\bjobs\.New(\w+Job)\(`)
	listenRe       = regexp.MustCompile(`"LISTEN "\s*\+\s*(\w+)`)
)

func uniqueLower(matches [][]string) []string {
	set := map[string]bool{}
	for _, m := range matches {
		set[strings.ToLower(m[1])] = true
	}
	var out []string
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ScanLegacySourceDependencies walks the Go sources under root (tests excluded) and returns
// every file whose SQL names a legacy table or compares a legacy type, every scheduled job
// constructed as jobs.New<Name>Job, and every LISTEN consumer channel.
func ScanLegacySourceDependencies(root string) ([]LegacyDependency, error) {
	var deps []LegacyDependency
	seen := map[string]bool{}
	add := func(d LegacyDependency) {
		if !seen[d.Key] {
			seen[d.Key] = true
			deps = append(deps, d)
		}
	}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if name := entry.Name(); name == "vendor" || name == "node_modules" || (strings.HasPrefix(name, ".") && path != root) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		text := string(src)
		tables := uniqueLower(legacySQLTableRe.FindAllStringSubmatch(text, -1))
		types := uniqueLower(legacySQLTypeRe.FindAllStringSubmatch(text, -1))
		if len(tables) > 0 || len(types) > 0 {
			add(LegacyDependency{Kind: "code", Key: "code:" + rel,
				Detail: fmt.Sprintf("tables=%s types=%s", strings.Join(tables, ","), strings.Join(types, ","))})
		}
		for _, m := range scheduledJobRe.FindAllStringSubmatch(text, -1) {
			add(LegacyDependency{Kind: "job", Key: "job:" + m[1], Detail: "scheduled in " + rel})
		}
		for _, m := range listenRe.FindAllStringSubmatch(text, -1) {
			add(LegacyDependency{Kind: "consumer", Key: "consumer:" + m[1], Detail: "LISTEN in " + rel})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan legacy source dependencies: %w", err)
	}
	return deps, nil
}

// declaredLegacyRelations are the relationships the task requires remapping (step 2);
// declaredLegacyFeatures are the workers and features it requires refactoring (step 3).
var (
	declaredLegacyRelations = []string{
		"votes", "bookmarks", "reports", "notifications", "accepted-answer-provenance",
		"approach-relationships", "progress-notes", "verification-records", "translations",
		"archived-cids",
	}
	declaredLegacyFeatures = []string{
		"briefing", "badges", "leaderboards", "reputation", "crystallization", "forgetting",
		"moderation", "duplicate-detection", "embedding-workers",
	}
)

// DeclaredLegacyDependencies returns the relationships and features the migration must
// account for whether or not a query happens to match them.
func DeclaredLegacyDependencies() []LegacyDependency {
	var deps []LegacyDependency
	for _, r := range declaredLegacyRelations {
		deps = append(deps, LegacyDependency{Kind: "relation", Key: "relation:" + r})
	}
	for _, f := range declaredLegacyFeatures {
		deps = append(deps, LegacyDependency{Kind: "feature", Key: "feature:" + f})
	}
	return deps
}
