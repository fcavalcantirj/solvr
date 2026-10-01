package api

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// idx 52 steps 1, 2 and 4: references/api.md and references/examples.md are installed with the
// skill (install.sh) and published at solvr.dev/references/. They are the direct-HTTP reference
// for an agent that calls the API without solvr.sh, and they documented and curled the legacy
// write routes (now 410 ENDPOINT_RETIRED) and typed creates. Every knowledge call they teach
// must be a route of a family the API keeps, a create must send no type, and every retired
// write must be named with its canonical replacement.

var publishedAPIReferences = []string{
	"../../../skill/references/api.md",
	"../../../frontend/public/references/api.md",
}

var publishedExampleReferences = []string{
	"../../../skill/references/examples.md",
	"../../../frontend/public/references/examples.md",
}

func readPublishedReference(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err, "the published reference must exist, or this check protects nothing")
	require.Greater(t, len(raw), 5000, "%s: an empty file would pass every check below", path)
	return string(raw)
}

// referenceCall is one API call a reference teaches: a curl command in a bash block, or an
// endpoint heading of api.md (its paths are relative to /v1 and name parameters :like_this).
type referenceCall struct {
	where, method, path, body string
}

var (
	bashBlock       = regexp.MustCompile("(?ms)^```(?:bash|sh)\n(.*?)^```")
	curlURL         = regexp.MustCompile(`"https://api\.solvr\.dev(/[^"?\s]*)`)
	curlMethod      = regexp.MustCompile(`-X\s+([A-Z]+)`)
	curlBody        = regexp.MustCompile(`(?s)-d\s+'(.*?)'`)
	endpointHeading = regexp.MustCompile(`(?m)^### (GET|POST|PUT|PATCH|DELETE) (/\S+)`)
	pathParameter   = regexp.MustCompile(`:\w+`)
	knowledgePath   = regexp.MustCompile(`^/v1/(posts|replies|problems|questions|ideas|approaches|answers|responses|comments)(/|$)`)
)

func referenceCalls(path, doc string) []referenceCall {
	var calls []referenceCall
	for _, block := range bashBlock.FindAllStringSubmatch(doc, -1) {
		for _, command := range strings.Split(block[1], "curl ")[1:] {
			url := curlURL.FindStringSubmatch(command)
			if url == nil {
				continue
			}
			method, body := "GET", ""
			if b := curlBody.FindStringSubmatch(command); b != nil {
				method, body = "POST", b[1]
			}
			if m := curlMethod.FindStringSubmatch(command); m != nil {
				method = m[1]
			}
			calls = append(calls, referenceCall{path + ": curl -X " + method + " " + url[1], method, url[1], body})
		}
	}
	for _, h := range endpointHeading.FindAllStringSubmatch(doc, -1) {
		p := pathParameter.ReplaceAllString(h[2], "X")
		if !strings.HasPrefix(p, "/r/") && !strings.HasPrefix(p, "/health") {
			p = "/v1" + p
		}
		calls = append(calls, referenceCall{path + ": ### " + h[1] + " " + h[2], h[1], p, ""})
	}
	return calls
}

// familyRouteOf finds the RouteFamilies route a concrete call hits. A static segment beats a
// parameter, as in the router, so the route with the fewest parameters wins.
func familyRouteOf(method, path string) (RouteFamily, string, bool) {
	var found RouteFamily
	var foundRoute string
	best := -1
	for _, f := range RouteFamilies {
		for _, route := range f.Routes {
			m, template, _ := strings.Cut(route, " ")
			if m != method {
				continue
			}
			ts, ps := strings.Split(template, "/"), strings.Split(path, "/")
			if len(ts) != len(ps) {
				continue
			}
			params, match := 0, true
			for i := range ts {
				if strings.HasPrefix(ts[i], "{") {
					params++
				} else if ts[i] != ps[i] {
					match = false
					break
				}
			}
			if match && (best < 0 || params < best) {
				found, foundRoute, best = f, route, params
			}
		}
	}
	return found, foundRoute, best >= 0
}

func retirementOf(route string) (LegacyRouteRetirement, bool) {
	for _, ret := range LegacyWriteRetirements {
		if ret.Route == route {
			return ret, true
		}
	}
	return LegacyRouteRetirement{}, false
}

// Every knowledge call a reference teaches is a route of a family the API keeps: never a
// retired write (410), never a legacy read on its way out, never a route the API does not serve.
func TestPublishedReferences_TeachOnlyKeptKnowledgeRoutes(t *testing.T) {
	for _, path := range append(append([]string{}, publishedAPIReferences...), publishedExampleReferences...) {
		doc := readPublishedReference(t, path)
		taught := map[string]bool{}
		for _, call := range referenceCalls(path, doc) {
			if !knowledgePath.MatchString(call.path) {
				continue
			}
			family, route, ok := familyRouteOf(call.method, call.path)
			switch ret, retired := retirementOf(route); {
			case !ok:
				t.Errorf("%s: the API serves no %s %s", call.where, call.method, call.path)
			case retired:
				t.Errorf("%s teaches %s, which answers 410 ENDPOINT_RETIRED; replacement: %q", call.where, route, ret.Replacement)
			case family.Disposition != DispositionKeep:
				t.Errorf("%s teaches %s of family %q (%s), not a kept route", call.where, route, family.Name, family.Disposition)
			default:
				taught[route] = true
			}
		}
		for _, route := range []string{"POST /v1/posts", "POST /v1/posts/{id}/replies"} {
			if !taught[route] {
				t.Errorf("%s must teach %s", path, route)
			}
		}
	}
}

// A create sends no type: the canonical post has no legacy type (idx 52 step 2).
func TestPublishedReferences_CreatePostsWithNoType(t *testing.T) {
	for _, path := range append(append([]string{}, publishedAPIReferences...), publishedExampleReferences...) {
		doc := readPublishedReference(t, path)
		creates := 0
		for _, call := range referenceCalls(path, doc) {
			if call.method != "POST" || call.path != "/v1/posts" || call.body == "" {
				continue
			}
			creates++
			if strings.Contains(call.body, `"type"`) {
				t.Errorf("%s creates a typed post: %s", call.where, strings.Join(strings.Fields(call.body), " "))
			}
		}
		require.Greater(t, creates, 0, "%s: no create example found, so the check above protected nothing", path)
	}
	for _, path := range publishedAPIReferences {
		doc := readPublishedReference(t, path)
		start := strings.Index(doc, "\n### POST /posts\n")
		require.GreaterOrEqual(t, start, 0, "%s documents no POST /posts", path)
		section := doc[start+1:]
		section = section[:strings.Index(section[4:], "\n### ")+4]
		require.False(t, strings.Contains(section, `"type"`), "%s: POST /posts documents a type field", path)
	}
}

// Every retired write is documented with its canonical replacement, the way the API answers it
// (idx 52 step 4), and the canonical reply family is documented endpoint by endpoint.
func TestPublishedAPIReference_NamesEveryRetiredWriteWithItsReplacement(t *testing.T) {
	for _, path := range publishedAPIReferences {
		doc := readPublishedReference(t, path)
		require.True(t, strings.Contains(doc, "410 ENDPOINT_RETIRED"), "%s must name the retired routes' answer", path)
		lines := strings.Split(doc, "\n")
		for _, ret := range LegacyWriteRetirements {
			var row string
			for _, line := range lines {
				if strings.HasPrefix(line, "|") && strings.Contains(line, "`"+ret.Route+"`") {
					row = line
					break
				}
			}
			switch {
			case row == "":
				t.Errorf("%s: no table row names the retired route %s", path, ret.Route)
			case ret.Replacement == "" && !strings.Contains(row, "no canonical equivalent"):
				t.Errorf("%s: %s must say it has no canonical equivalent: %s", path, ret.Route, row)
			case ret.Replacement != "" && !strings.Contains(row, "`"+ret.Replacement+"`"):
				t.Errorf("%s: %s must name its replacement %s: %s", path, ret.Route, ret.Replacement, row)
			}
		}

		documented := map[string]bool{}
		for _, call := range referenceCalls(path, doc) {
			if strings.Contains(call.where, ": ### ") {
				if _, route, ok := familyRouteOf(call.method, call.path); ok {
					documented[route] = true
				}
			}
		}
		for _, f := range RouteFamilies {
			if f.Name == "canonical-replies" {
				for _, route := range f.Routes {
					if !documented[route] {
						t.Errorf("%s has no endpoint heading for %s", path, route)
					}
				}
			}
		}
	}
}

// The examples teach the reply body and threading, which replace approaches, progress notes,
// answers, responses and comments.
func TestPublishedExamples_TeachRepliesAndThreading(t *testing.T) {
	for _, path := range publishedExampleReferences {
		doc := readPublishedReference(t, path)
		replies, threaded := 0, 0
		for _, call := range referenceCalls(path, doc) {
			if _, route, ok := familyRouteOf(call.method, call.path); !ok || route != "POST /v1/posts/{id}/replies" {
				continue
			}
			if !strings.Contains(call.body, `"body"`) {
				t.Errorf("%s: a reply sends its text as body", call.where)
			}
			replies++
			if strings.Contains(call.body, `"parent_reply_id"`) {
				threaded++
			}
		}
		require.Greater(t, replies, 0, "%s must show a reply", path)
		require.Greater(t, threaded, 0, "%s must show a threaded reply (parent_reply_id)", path)
	}
}
