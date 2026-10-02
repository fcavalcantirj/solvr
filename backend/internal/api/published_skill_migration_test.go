package api

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// skillVersion is the version solvr.sh refuses a removed 3.x choice with (SOLVR_SKILL_VERSION in
// skill/scripts/solvr-migrating.sh).
func skillVersion(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile("../../../skill/scripts/solvr-migrating.sh")
	require.NoError(t, err, "solvr-migrating.sh holds the skill's version and its removed choices")
	m := regexp.MustCompile(`(?m)^SOLVR_SKILL_VERSION="([^"]+)"$`).FindSubmatch(src)
	require.NotNil(t, m, "solvr-migrating.sh must declare SOLVR_SKILL_VERSION")
	return string(m[1])
}

// retiredRoutes maps every retired legacy route, with its parameters unnamed, to its replacement.
func retiredRoutes(t *testing.T) map[string]string {
	t.Helper()
	retired := map[string]string{}
	for _, ret := range append(append([]LegacyRouteRetirement{}, LegacyWriteRetirements...), LegacyReadRetirements...) {
		retired[unnamedRoute(ret.Route)] = ret.Replacement
	}
	require.NotEmpty(t, retired)
	return retired
}

// unnamedRoute writes the parameters of a "METHOD /path" template (":id" or "{id}") as "{}".
func unnamedRoute(route string) string {
	return regexp.MustCompile(`:[A-Za-z_]+|\{[A-Za-z_]+\}`).ReplaceAllString(route, "{}")
}

// skillJSONRoutes reads every "METHOD /path" a skill.json endpoint names: "GET/POST /v1/posts (...)"
// names GET /v1/posts and POST /v1/posts.
func skillJSONRoutes(endpoint string) []string {
	var routes []string
	pattern := regexp.MustCompile(`\b((?:GET|POST|PUT|PATCH|DELETE)(?:/(?:GET|POST|PUT|PATCH|DELETE))*) (/[^\s,?()]+)`)
	for _, m := range pattern.FindAllStringSubmatch(endpoint, -1) {
		for _, method := range strings.Split(m[1], "/") {
			routes = append(routes, method+" "+m[2])
		}
	}
	return routes
}

// idx 78 step 5: the skill moved to 4.0.0, removing the 3.x choices of the legacy knowledge model.
// skill.json (published at solvr.dev/skill.json) is what an agent reads for the skill's version and
// the endpoints behind it. It published 3.7.16 and named POST /v1/problems/:id/approaches and
// POST /v1/questions/:id/answers, which answer every caller 410 ENDPOINT_RETIRED. Both copies must
// publish the version solvr.sh refuses with, name no retired route, and name the reply routes.
func TestPublishedSkillJSON_PublishesTheSkillVersionAndNoRetiredRoute(t *testing.T) {
	version := skillVersion(t)
	retired := retiredRoutes(t)

	var copies [][]byte
	for _, path := range []string{"../../../skill/skill.json", "../../../frontend/public/skill.json"} {
		raw, err := os.ReadFile(path)
		require.NoError(t, err, "the published document must exist, or this check protects nothing")
		copies = append(copies, raw)
		var doc struct {
			Version   string            `json:"version"`
			Endpoints map[string]string `json:"endpoints"`
		}
		require.NoError(t, json.Unmarshal(raw, &doc), path)
		require.Equal(t, version, doc.Version, "%s must publish the version solvr.sh refuses with", path)
		require.Greater(t, len(doc.Endpoints), 10, "%s: an empty endpoint list would pass every check below", path)

		named := map[string]bool{}
		for key, endpoint := range doc.Endpoints {
			routes := skillJSONRoutes(endpoint)
			require.NotEmpty(t, routes, "%s endpoint %q names no route: %s", path, key, endpoint)
			for _, route := range routes {
				named[unnamedRoute(route)] = true
				replacement, isRetired := retired[unnamedRoute(route)]
				require.False(t, isRetired, "%s endpoint %q names %s, which answers 410 ENDPOINT_RETIRED (use %q)",
					path, key, route, replacement)
			}
		}
		for _, route := range []string{"GET /v1/posts/{}/replies", "POST /v1/posts/{}/replies", "GET /v1/replies/{}",
			"PATCH /v1/replies/{}"} {
			require.True(t, named[route], "%s must name %s: answers and approaches are replies", path, route)
		}
	}
	require.Equal(t, string(copies[0]), string(copies[1]), "solvr.dev/skill.json must be skill/skill.json")
}

// idx 78 step 5: SKILL.md (and its copy at solvr.dev/skill.md) carries the skill's notes for moving
// from 3.x to the current version. They say which routes a 3.x skill's removed commands call and
// that those answer 410 ENDPOINT_RETIRED naming the replacement; every such claim must be true of
// the API, or the notes send an agent after a route that is not there.
func TestPublishedSkill_MigrationNotesNameTheRetiredRoutesTruly(t *testing.T) {
	version := skillVersion(t)
	retired := retiredRoutes(t)
	heading := "## Migrating from 3.x to " + version

	for _, path := range []string{"../../../skill/SKILL.md", "../../../frontend/public/skill.md"} {
		raw, err := os.ReadFile(path)
		require.NoError(t, err, "the published document must exist, or this check protects nothing")
		doc := string(raw)
		start := strings.Index(doc, "\n"+heading+"\n")
		require.GreaterOrEqual(t, start, 0, "%s must carry %q", path, heading)
		notes := doc[start+1:]
		if end := strings.Index(notes[len(heading):], "\n## "); end >= 0 {
			notes = notes[:len(heading)+end]
		}

		require.Contains(t, notes, "410 ENDPOINT_RETIRED", path)
		require.Contains(t, notes, "error.details.replacement", path)
		for legacy, replacement := range map[string]string{
			"POST /v1/questions/{id}/answers":   "POST /v1/posts/{id}/replies",
			"POST /v1/problems/{id}/approaches": "POST /v1/posts/{id}/replies",
		} {
			require.Contains(t, notes, "`"+legacy+"`", "%s notes must name the route a 3.x skill calls", path)
			got, isRetired := retired[unnamedRoute(legacy)]
			require.True(t, isRetired, "the notes say %s is retired; the API must retire it", legacy)
			require.Equal(t, replacement, got, "the API must name the replacement the notes give for %s", legacy)
			require.Contains(t, notes, "`"+replacement+"`", "%s notes must name the replacement of %s", path, legacy)
		}
		for _, removed := range []string{"`solvr post <type>`", "`solvr answer`", "`solvr approach`", "`--include`", "`--type`"} {
			require.Contains(t, notes, removed, "%s notes must name every removed 3.x choice", path)
		}
	}
}
