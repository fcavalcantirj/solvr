package api

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/growth"
)

// /llms.txt (llmstxt.org) is the index an LLM reads first. Like skill.md it is authored in
// skill/ and copied into frontend/public by scripts/sync-skill.sh (the frontend prebuild),
// and the copy is committed. This test keeps it current: the two copies are identical, it
// has the llms.txt shape, and every link resolves to a GET route this API serves (as
// api.RouteFamilies lists them) or a page or file the site serves.

var llmsTxtLinkRe = regexp.MustCompile(`\]\((https://(?:api\.)?solvr\.dev[^)\s]*)\)`)

func TestPublishedLLMsTxt_IsTheSyncedIndexAndEveryLinkResolves(t *testing.T) {
	src := repoFile(t, "skill/llms.txt")
	require.Equal(t, src, repoFile(t, "frontend/public/llms.txt"),
		"frontend/public/llms.txt must be the synced copy of skill/llms.txt (scripts/sync-skill.sh)")

	lines := strings.Split(src, "\n")
	require.Equal(t, "# Solvr", lines[0], "an llms.txt opens with the site's name as H1")
	require.True(t, len(lines) > 2 && strings.HasPrefix(lines[2], "> "), "then a one-paragraph blockquote summary")
	for _, h := range []string{"## Connect your agents", "## Agent skill", "## API", "## Docs"} {
		require.Contains(t, src, "\n"+h+"\n")
	}
	for _, want := range []string{
		"https://api.solvr.dev/v1/connect", "https://solvr.dev/connect", "https://solvr.dev/skill.md",
		"https://api.solvr.dev/v1/openapi.json", "https://api.solvr.dev/v1/search?q=",
		"https://solvr.dev/docs/guides", "https://solvr.dev/rooms/" + growth.PublicDemoRoomSlug,
	} {
		require.Contains(t, src, want)
	}

	served := map[string]bool{}
	for _, f := range RouteFamilies {
		for _, r := range f.Routes {
			served[r] = true
		}
	}
	links := llmsTxtLinkRe.FindAllStringSubmatch(src, -1)
	require.NotEmpty(t, links)
	for _, m := range links {
		u, err := url.Parse(m[1])
		require.NoError(t, err, m[1])
		if u.Host == "api.solvr.dev" {
			require.True(t, served["GET "+u.Path], "%s is not a GET route the API serves", m[1])
			continue
		}
		require.True(t, sitePageExists(u.Path), "%s is not a page or file the site serves", m[1])
	}
}

// sitePageExists reports whether the Next.js app serves path: a public file, a page, or a
// page under a dynamic segment ([slug]).
func sitePageExists(path string) bool {
	root := filepath.Join("..", "..", "..", "frontend")
	exists := func(p string) bool { _, err := os.Stat(p); return err == nil }
	clean := strings.Trim(path, "/")
	if clean != "" && exists(filepath.Join(root, "public", clean)) {
		return true
	}
	if exists(filepath.Join(root, "app", clean, "page.tsx")) {
		return true
	}
	dynamic, _ := filepath.Glob(filepath.Join(root, "app", filepath.Dir(clean), "[[]*]", "page.tsx"))
	return clean != "" && len(dynamic) > 0
}
