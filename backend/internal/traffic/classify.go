// Package traffic decides who a counted row came from (SPEC.md 27.6): a person, an agent, a
// crawler or automation. It is pure: it reads what a request said about itself and never a
// database, so the ingest handlers, the request log and the behavioural rules share one rule.
//
// Rows are flagged, never deleted. A row classed crawler or automation stays where it is, and
// the counted_* views (migration 000146) are the one place a report leaves it out.
package traffic

import (
	"regexp"
	"strconv"
	"strings"
)

// Class is who a row came from, as stored in traffic_class.
type Class string

// The classes 000146's traffic_class checks allow.
const (
	// Unclassified is a row stored before classification existed, or a confirmed server step.
	Unclassified Class = "unclassified"
	Human        Class = "human"
	Agent        Class = "agent"
	Crawler      Class = "crawler"
	Automation   Class = "automation"
)

// Classes is every class, in the order 000146 lists them.
var Classes = []Class{Unclassified, Human, Agent, Crawler, Automation}

// Excluded reports whether the counted_* views leave a row of this class out.
func (c Class) Excluded() bool {
	return c == Crawler || c == Automation
}

// Reason is why a row got its class, as stored in traffic_reason.
type Reason string

// The reasons. The first eight come from what one request said (Classify); the last three
// from how several rows behaved together (the behavioural rules, db/traffic_rules.go).
const (
	ReasonDeclaredBot      Reason = "declared_bot"
	ReasonHeadlessMarker   Reason = "headless_marker"
	ReasonImpossibleUA     Reason = "impossible_ua"
	ReasonAgentCredential  Reason = "agent_credential" //nolint:gosec // G101: a reason's name, not a credential
	ReasonAssistantUA      Reason = "assistant_ua"
	ReasonNoBrowserHeaders Reason = "no_browser_headers"
	ReasonBrowser          Reason = "browser"
	ReasonHTTPClient       Reason = "http_client"
	// ReasonFetchedWithoutCopy is B1: a post-sourced start whose skill an agent fetched, with no copy.
	ReasonFetchedWithoutCopy Reason = "fetched_without_copy"
	// ReasonConnectBurst is B2: five or more uncopied post-sourced starts in one clock hour.
	ReasonConnectBurst Reason = "connect_burst"
	// ReasonRepeatedQuery is B3: one address sent one query twenty or more times in one clock hour.
	ReasonRepeatedQuery Reason = "repeated_query"
)

// Reasons is every reason.
var Reasons = []Reason{
	ReasonDeclaredBot, ReasonHeadlessMarker, ReasonImpossibleUA, ReasonAgentCredential, ReasonAssistantUA,
	ReasonNoBrowserHeaders, ReasonBrowser, ReasonHTTPClient, ReasonFetchedWithoutCopy, ReasonConnectBurst,
	ReasonRepeatedQuery,
}

// Surface is where a request was measured.
type Surface int

const (
	// SurfacePage is a page of the site, reported by its web server (POST /v1/analytics/pageview).
	SurfacePage Surface = iota + 1
	// SurfaceBrowserEvent is a funnel step a page reported from the browser.
	SurfaceBrowserEvent
	// SurfaceSkillFetch is the web server's report that the skill link was fetched.
	SurfaceSkillFetch
	// SurfaceAPI is a call of the API itself, a search.
	SurfaceAPI
)

// Signals is what a request said about itself. Nothing here is stored.
type Signals struct {
	// UserAgent is the User-Agent of the request being classified.
	UserAgent string
	// ClientHints is its Sec-CH-UA header.
	ClientHints string
	// FetchMode is its Sec-Fetch-Mode header: a browser sends one with every request.
	FetchMode string
	Surface   Surface
	// AgentCredential is true when the request carried an agent's credential.
	AgentCredential bool
	// HeadersKnown is false for a stored row, whose fetch headers were never kept: the rule
	// that needs them is then not applied, and a browser user agent alone reads as a person.
	HeadersKnown bool
}

// Classify applies the ordered rules; the first that matches decides.
//
//  1. A declared bot token (BotTokens) is a crawler.
//  2. A headless marker in the user agent or Sec-CH-UA is automation.
//  3. Safari 26 or later on an iPhone OS below 18 cannot exist, so it is automation.
//  4. An agent credential, or an assistant fetching for its person, is an agent.
//  5. A page view or a browser event without Sec-Fetch-Mode is automation: browsers send it.
//  6. A browser with fetch headers is a person.
//  7. Anything else, on a skill fetch or the API, is an agent's HTTP client.
func Classify(s Signals) (Class, Reason) {
	switch {
	case IsDeclaredBot(s.UserAgent):
		return Crawler, ReasonDeclaredBot
	case hasHeadlessMarker(s.UserAgent) || hasHeadlessMarker(s.ClientHints):
		return Automation, ReasonHeadlessMarker
	case isImpossibleUA(s.UserAgent):
		return Automation, ReasonImpossibleUA
	case s.AgentCredential:
		return Agent, ReasonAgentCredential
	case isAssistant(s.UserAgent):
		return Agent, ReasonAssistantUA
	case s.HeadersKnown && (s.Surface == SurfacePage || s.Surface == SurfaceBrowserEvent) && s.FetchMode == "":
		return Automation, ReasonNoBrowserHeaders
	case isBrowser(s.UserAgent) && (s.FetchMode != "" || !s.HeadersKnown):
		return Human, ReasonBrowser
	default:
		return Agent, ReasonHTTPClient
	}
}

// The user-agent classes of api_request_events.ua_class (000146).
const (
	UAClassBrowser    = "browser"
	UAClassWebServer  = "web_server"
	UAClassHTTPClient = "http_client"
	UAClassCrawler    = "crawler"
	UAClassHeadless   = "headless"
	UAClassNone       = "none"
)

// UAClasses is every user-agent class.
var UAClasses = []string{UAClassBrowser, UAClassWebServer, UAClassHTTPClient, UAClassCrawler, UAClassHeadless, UAClassNone}

// webServerPrefix starts the user agent of the site's own web server (its server renders and
// its reports).
const webServerPrefix = "solvr-web/"

// UAClass names the kind of client a user agent (and its Sec-CH-UA) belongs to. An
// assistant fetching for its person is an HTTP client: it is an agent's tool, not a browser.
func UAClass(userAgent, clientHints string) string {
	switch {
	case strings.TrimSpace(userAgent) == "":
		return UAClassNone
	case IsDeclaredBot(userAgent):
		return UAClassCrawler
	case hasHeadlessMarker(userAgent) || hasHeadlessMarker(clientHints):
		return UAClassHeadless
	case strings.HasPrefix(userAgent, webServerPrefix):
		return UAClassWebServer
	case isAssistant(userAgent):
		return UAClassHTTPClient
	case isBrowser(userAgent):
		return UAClassBrowser
	default:
		return UAClassHTTPClient
	}
}

// BotTokens is the ONE list of what a link preview or a crawler calls itself in its
// User-Agent, spelled as its owner spells it. A user agent that contains any of them, in any
// letter case, is a bot's:
//
//   - a link preview: somebody pasted a sentence into Slack, WhatsApp, Discord, Telegram, X,
//     LinkedIn, Facebook, Skype or Teams, Reddit, Mastodon or an e-mail, and the app fetched the
//     link to show what it points at;
//   - a crawler: a search engine, an SEO tool or a model trainer.
//
// An agent's tool is deliberately NOT here and must never be added: ChatGPT-User, Claude-User,
// claude-code, curl, python-requests, node, Go-http-client, axios, undici, okhttp. Each is a
// person's assistant or an agent's HTTP client, and so is a request that names no user agent at
// all. handlers/funnel_skill_surface_test.go holds both lists against each other (SPEC.md 25.7).
var BotTokens = []string{
	// Link previews.
	"Slackbot", "Slack-ImgProxy", "WhatsApp", "Discordbot", "TelegramBot", "Twitterbot", "LinkedInBot",
	"facebookexternalhit", "Facebot", "SkypeUriPreview", "Iframely", "Embedly", "redditbot", "Mastodon",
	// Crawlers.
	"Googlebot", "Google-InspectionTool", "bingbot", "DuckDuckBot", "YandexBot", "Baiduspider", "Applebot",
	"AhrefsBot", "SemrushBot", "MJ12bot", "PetalBot", "Bytespider", "GPTBot", "OAI-SearchBot", "ClaudeBot",
	"PerplexityBot", "CCBot",
}

// headlessTokens are what a headless browser or a browser driver leaves in a user agent or in
// Sec-CH-UA.
var headlessTokens = []string{"HeadlessChrome", "Lighthouse", "PTST", "PhantomJS", "Puppeteer", "Playwright", "Selenium"}

// assistantTokens are a person's assistant fetching for them: an agent, never a crawler.
var assistantTokens = []string{"Claude-User", "ChatGPT-User", "Perplexity-User", "claude-code"}

var (
	botNeedles       = lowered(BotTokens)
	headlessNeedles  = lowered(headlessTokens)
	assistantNeedles = lowered(assistantTokens)
)

func lowered(tokens []string) []string {
	out := make([]string, len(tokens))
	for i, token := range tokens {
		out[i] = strings.ToLower(token)
	}
	return out
}

func containsAny(s string, needles []string) bool {
	if s == "" {
		return false
	}
	lower := strings.ToLower(s)
	for _, needle := range needles {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

// IsDeclaredBot reports whether a user agent names a link preview or a crawler.
func IsDeclaredBot(userAgent string) bool { return containsAny(userAgent, botNeedles) }

func hasHeadlessMarker(s string) bool { return containsAny(s, headlessNeedles) }

func isAssistant(userAgent string) bool { return containsAny(userAgent, assistantNeedles) }

// isBrowser reports a browser's user agent: every browser's starts with Mozilla/.
func isBrowser(userAgent string) bool { return strings.Contains(userAgent, "Mozilla/") }

var (
	iPhoneOSVersion = regexp.MustCompile(`iPhone OS (\d+)_`)
	safariVersion   = regexp.MustCompile(`Version/(\d+)`)
)

// isImpossibleUA reports an iPhone that cannot exist: Safari 26 or later, which shipped with
// iOS 26, beside an iPhone OS below 18. (iOS 26 itself reports a frozen 18_6.)
func isImpossibleUA(userAgent string) bool {
	osMatch := iPhoneOSVersion.FindStringSubmatch(userAgent)
	safariMatch := safariVersion.FindStringSubmatch(userAgent)
	if osMatch == nil || safariMatch == nil {
		return false
	}
	osMajor, err1 := strconv.Atoi(osMatch[1])
	safariMajor, err2 := strconv.Atoi(safariMatch[1])
	return err1 == nil && err2 == nil && safariMajor >= 26 && osMajor < 18
}
