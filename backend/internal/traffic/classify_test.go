package traffic

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Who a counted request came from (SPEC.md 27.6). These tests need no database: the
// decision is a pure function of what the request said about itself.

// User agents measured on production between 2026-10-05 and 2026-10-17.
const (
	// The tag-walking crawler: real desktop Chrome headers, 100% single tag words.
	crawlerChromeMac = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36"
	crawlerChromeWin = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"
	// An iPhone that cannot exist: iOS 11 never shipped Safari 26.
	impossibleIPhone = "Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/26.5 Mobile/15A372 Safari/604.1"
	// iOS 26 freezes the system version in its user agent at 18_6: a real current iPhone.
	currentIPhone = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.0 Mobile/15E148 Safari/604.1"
	googlebot     = "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; Googlebot/2.1; +http://www.google.com/bot.html) Chrome/141.0.7390.122 Safari/537.36"
	headless      = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/141.0.0.0 Safari/537.36"
	curlUA        = "curl/7.76.1"
	chatGPTUser   = "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko); compatible; ChatGPT-User/1.0; +https://openai.com/bot"
)

func TestClassify_TheOrderedRules(t *testing.T) {
	for _, c := range []struct {
		name   string
		class  Class
		reason Reason
		s      Signals
	}{
		// 1. A declared bot is a crawler, whatever else the request says.
		{"googlebot on a page", Crawler, ReasonDeclaredBot, Signals{Surface: SurfacePage, UserAgent: googlebot, FetchMode: "navigate", HeadersKnown: true}},
		{"googlebot on the API", Crawler, ReasonDeclaredBot, Signals{Surface: SurfaceAPI, UserAgent: googlebot, HeadersKnown: true}},
		// 2. A headless browser names itself in its user agent or its client hints.
		{"headless user agent", Automation, ReasonHeadlessMarker, Signals{Surface: SurfaceBrowserEvent, UserAgent: headless, FetchMode: "cors", HeadersKnown: true}},
		{"headless client hint", Automation, ReasonHeadlessMarker, Signals{Surface: SurfaceBrowserEvent, UserAgent: crawlerChromeMac, ClientHints: `"Chromium";v="141", "Not?A_Brand";v="8", "HeadlessChrome";v="141"`, FetchMode: "cors", HeadersKnown: true}},
		{"playwright", Automation, ReasonHeadlessMarker, Signals{Surface: SurfacePage, UserAgent: "Mozilla/5.0 Playwright/1.48", FetchMode: "navigate", HeadersKnown: true}},
		// 3. An iPhone that cannot exist.
		{"iOS 11 with Safari 26", Automation, ReasonImpossibleUA, Signals{Surface: SurfaceAPI, UserAgent: impossibleIPhone, FetchMode: "cors", HeadersKnown: true}},
		{"iOS 26 (frozen at 18_6) with Safari 26", Human, ReasonBrowser, Signals{Surface: SurfaceAPI, UserAgent: currentIPhone, FetchMode: "cors", HeadersKnown: true}},
		// 4. An agent's credential, or an assistant fetching for its person.
		{"agent key", Agent, ReasonAgentCredential, Signals{Surface: SurfaceAPI, UserAgent: curlUA, AgentCredential: true, HeadersKnown: true}},
		{"assistant on a page", Agent, ReasonAssistantUA, Signals{Surface: SurfacePage, UserAgent: chatGPTUser, HeadersKnown: true}},
		{"claude-code", Agent, ReasonAssistantUA, Signals{Surface: SurfaceSkillFetch, UserAgent: "claude-code/2.0.14", HeadersKnown: true}},
		// 5. A page view or a browser event without fetch headers is not a browser.
		{"page view without Sec-Fetch-Mode", Automation, ReasonNoBrowserHeaders, Signals{Surface: SurfacePage, UserAgent: crawlerChromeMac, HeadersKnown: true}},
		{"browser event without Sec-Fetch-Mode", Automation, ReasonNoBrowserHeaders, Signals{Surface: SurfaceBrowserEvent, UserAgent: crawlerChromeWin, HeadersKnown: true}},
		{"curl on a page", Automation, ReasonNoBrowserHeaders, Signals{Surface: SurfacePage, UserAgent: curlUA, HeadersKnown: true}},
		// 6. A browser with fetch headers is a person. The tag walker sends real ones (a stated limit).
		{"browser page view", Human, ReasonBrowser, Signals{Surface: SurfacePage, UserAgent: crawlerChromeMac, FetchMode: "navigate", HeadersKnown: true}},
		{"browser search", Human, ReasonBrowser, Signals{Surface: SurfaceAPI, UserAgent: crawlerChromeWin, FetchMode: "cors", HeadersKnown: true}},
		{"browser opening the skill link", Human, ReasonBrowser, Signals{Surface: SurfaceSkillFetch, UserAgent: crawlerChromeMac, FetchMode: "navigate", HeadersKnown: true}},
		// 7. Anything else on a skill fetch or the API is an agent's HTTP client.
		{"curl on a skill fetch", Agent, ReasonHTTPClient, Signals{Surface: SurfaceSkillFetch, UserAgent: curlUA, HeadersKnown: true}},
		{"no user agent on the API", Agent, ReasonHTTPClient, Signals{Surface: SurfaceAPI, HeadersKnown: true}},
		{"a browser user agent without fetch headers on the API", Agent, ReasonHTTPClient, Signals{Surface: SurfaceAPI, UserAgent: crawlerChromeMac, HeadersKnown: true}},
	} {
		t.Run(c.name, func(t *testing.T) {
			class, reason := Classify(c.s)
			assert.Equal(t, c.class, class)
			assert.Equal(t, c.reason, reason)
		})
	}
}

// A stored row kept no fetch headers. Without them a browser user agent alone reads as a
// person, and the rule that needs the headers is not applied.
func TestClassify_AStoredRowWithoutItsHeaders(t *testing.T) {
	for _, c := range []struct {
		name   string
		class  Class
		reason Reason
		s      Signals
	}{
		{"browser search", Human, ReasonBrowser, Signals{Surface: SurfaceAPI, UserAgent: crawlerChromeMac}},
		{"impossible iPhone", Automation, ReasonImpossibleUA, Signals{Surface: SurfaceAPI, UserAgent: impossibleIPhone}},
		{"curl", Agent, ReasonHTTPClient, Signals{Surface: SurfaceAPI, UserAgent: curlUA}},
		{"an agent's search", Agent, ReasonAgentCredential, Signals{Surface: SurfaceAPI, UserAgent: crawlerChromeMac, AgentCredential: true}},
		{"a browser event", Human, ReasonBrowser, Signals{Surface: SurfaceBrowserEvent, UserAgent: crawlerChromeMac}},
	} {
		t.Run(c.name, func(t *testing.T) {
			class, reason := Classify(c.s)
			assert.Equal(t, c.class, class)
			assert.Equal(t, c.reason, reason)
		})
	}
}

func TestUAClass(t *testing.T) {
	for _, c := range []struct{ ua, hints, class string }{
		{"", "", UAClassNone},
		{"   ", "", UAClassNone},
		{googlebot, "", UAClassCrawler},
		{headless, "", UAClassHeadless},
		{crawlerChromeMac, `"HeadlessChrome";v="141"`, UAClassHeadless},
		{"solvr-web/1.0 (ssr)", "", UAClassWebServer},
		{"solvr-web/1.0 (skill-fetch report)", "", UAClassWebServer},
		{crawlerChromeMac, "", UAClassBrowser},
		{impossibleIPhone, "", UAClassBrowser},
		{curlUA, "", UAClassHTTPClient},
		{"Go-http-client/2.0", "", UAClassHTTPClient},
		{chatGPTUser, "", UAClassHTTPClient},
		{"claude-code/2.0.14", "", UAClassHTTPClient},
	} {
		assert.Equal(t, c.class, UAClass(c.ua, c.hints), "%q %q", c.ua, c.hints)
	}
	for _, class := range []string{UAClassBrowser, UAClassWebServer, UAClassHTTPClient, UAClassCrawler, UAClassHeadless, UAClassNone} {
		assert.Contains(t, UAClasses, class)
	}
	assert.Len(t, UAClasses, 6, "the closed list of 000146's api_request_events_ua_class_check")
}

// The ONE list of what a link preview or a crawler calls itself (moved from the skill fetch,
// SPEC.md 25.7): a declared bot in any letter case, never an agent's tool.
func TestIsDeclaredBot(t *testing.T) {
	require.Len(t, BotTokens, 31)
	for _, token := range BotTokens {
		assert.True(t, IsDeclaredBot("Mozilla/5.0 (compatible; "+strings.ToUpper(token)+"/1.0)"), token)
		assert.True(t, IsDeclaredBot(strings.ToLower(token)), token)
	}
	for _, tool := range []string{"ChatGPT-User/1.0", "Claude-User", "claude-code/2.0", "curl/8.7.1", "python-requests/2.32", "node", "Go-http-client/2.0", "axios/1.7", "undici", "okhttp/4.12", "", crawlerChromeMac} {
		assert.False(t, IsDeclaredBot(tool), tool)
	}
}

// Every class and reason is one 000146's checks and SPEC.md 27.6 name.
func TestClassesAndReasonsAreTheDocumentedVocabulary(t *testing.T) {
	assert.ElementsMatch(t, []Class{Unclassified, Human, Agent, Crawler, Automation}, Classes)
	assert.ElementsMatch(t, []Reason{
		ReasonDeclaredBot, ReasonHeadlessMarker, ReasonImpossibleUA, ReasonAgentCredential, ReasonAssistantUA,
		ReasonNoBrowserHeaders, ReasonBrowser, ReasonHTTPClient, ReasonFetchedWithoutCopy, ReasonConnectBurst,
		ReasonRepeatedQuery,
	}, Reasons)
	assert.True(t, Crawler.Excluded())
	assert.True(t, Automation.Excluded())
	for _, c := range []Class{Unclassified, Human, Agent} {
		assert.False(t, c.Excluded(), c)
	}
}
