package handlers

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// A bot is not an agent (SPEC.md 25.7). A sentence pasted into a chat app, a social
// network or an e-mail makes that app fetch the skill link to build a preview, and a
// search crawler that renders /connect may follow the link. The API tells the three
// apart, in this order: a browser navigation is a person's visit, a link-preview or
// crawler user agent is a bot's fetch, and everything else is an agent's fetch.
//
// These tests need no database: the decision is a pure function of two strings.

// The link-preview and crawler tokens, as SPEC.md 25.7 lists them.
var specBotTokens = []string{
	"Slackbot", "Slack-ImgProxy", "WhatsApp", "Discordbot", "TelegramBot", "Twitterbot", "LinkedInBot",
	"facebookexternalhit", "Facebot", "SkypeUriPreview", "Iframely", "Embedly", "redditbot", "Mastodon",
	"Googlebot", "Google-InspectionTool", "bingbot", "DuckDuckBot", "YandexBot", "Baiduspider", "Applebot",
	"AhrefsBot", "SemrushBot", "MJ12bot", "PetalBot", "Bytespider", "GPTBot", "OAI-SearchBot", "ClaudeBot",
	"PerplexityBot", "CCBot",
}

// What an agent's tool names itself. None of these may ever be taken for a bot.
var agentToolTokens = []string{
	"ChatGPT-User", "Claude-User", "claude-code", "curl", "python-requests", "node", "Go-http-client",
	"axios", "undici", "okhttp",
}

// The list lives in one place and is exactly the documented one.
func TestSkillFetchBotTokens_AreTheDocumentedList(t *testing.T) {
	require.Equal(t, specBotTokens, skillFetchBotTokens)
	require.Len(t, skillFetchBotTokens, 31)

	seen := map[string]bool{}
	for _, token := range skillFetchBotTokens {
		lower := strings.ToLower(token)
		require.False(t, seen[lower], "%s is listed twice", token)
		seen[lower] = true
		require.Equal(t, strings.TrimSpace(token), token)
		require.GreaterOrEqual(t, len(token), 5, "%s: a token this short would match by accident", token)
	}
}

// Agent tools are deliberately off the list, and no listed token can match one by accident:
// the name of an agent tool contains no bot token, in any letter case.
func TestSkillFetchBotTokens_NeverNameAnAgentTool(t *testing.T) {
	for _, tool := range agentToolTokens {
		for _, token := range skillFetchBotTokens {
			require.False(t, strings.Contains(strings.ToLower(tool), strings.ToLower(token)),
				"the agent tool %s would be read as the bot %s", tool, token)
			require.False(t, strings.EqualFold(tool, token), "%s is an agent tool and must not be listed", tool)
		}
	}
}

func TestSkillFetchSurface(t *testing.T) {
	const (
		agent   = models.FunnelSurfaceAgentFetch
		bot     = models.FunnelSurfaceBotFetch
		browser = models.FunnelSurfaceBrowserVisit
	)
	require.Equal(t, "bot_fetch", bot)

	chrome := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36"
	googlebotPhone := "Mozilla/5.0 (Linux; Android 6.0.1; Nexus 5X Build/MMB29P) AppleWebKit/537.36 (KHTML, like Gecko) " +
		"Chrome/141.0.7390.122 Mobile Safari/537.36 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"

	cases := []struct {
		name, mode, userAgent, want string
	}{
		// 1. A browser navigation is a person's visit, whatever the user agent says.
		{"a person's browser", "navigate", chrome, browser},
		{"a navigation with no user agent", "navigate", "", browser},
		{"a navigation wins over a bot's name", "navigate", "Slackbot-LinkExpanding 1.0 (+https://api.slack.com/robots)", browser},
		{"a crawler that renders the page and opens the link", "navigate", googlebotPhone, browser},

		// 2. Link previews: the apps a sentence is pasted into.
		{"Slack's link preview", "", "Slackbot-LinkExpanding 1.0 (+https://api.slack.com/robots)", bot},
		{"Slack's image proxy", "", "Slack-ImgProxy (+https://api.slack.com/robots)", bot},
		{"WhatsApp", "", "WhatsApp/2.23.20.0 A", bot},
		{"Discord", "", "Mozilla/5.0 (compatible; Discordbot/2.0; +https://discordapp.com)", bot},
		{"Telegram", "", "TelegramBot (like TwitterBot)", bot},
		{"X", "", "Twitterbot/1.0", bot},
		{"LinkedIn", "", "LinkedInBot/1.0 (compatible; Mozilla/5.0; Apache-HttpClient +http://www.linkedin.com)", bot},
		{"Facebook", "", "facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)", bot},
		{"Facebook's crawler", "", "Facebot", bot},
		{"iMessage, which borrows three names", "", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_1) AppleWebKit/601.2.4 (KHTML, like Gecko) Version/9.0.1 Safari/601.2.4 facebookexternalhit/1.1 Facebot Twitterbot/1.0", bot},
		{"Skype and Teams", "", "Mozilla/5.0 (Windows NT 6.1; WOW64) SkypeUriPreview Preview/0.5 skype-url-preview@microsoft.com", bot},
		{"Iframely", "", "Iframely/1.3.1 (+https://iframely.com/docs/about)", bot},
		{"Embedly", "", "Mozilla/5.0 (compatible; Embedly/0.2; +http://support.embed.ly/)", bot},
		{"Reddit", "", "Mozilla/5.0 (compatible; redditbot/1.0; +http://www.reddit.com/feedback)", bot},
		{"Mastodon", "", "http.rb/5.2.0 (Mastodon/4.3.0; +https://mastodon.social/)", bot},

		// 2. Crawlers: search engines, SEO tools and model trainers.
		{"Googlebot desktop", "", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", bot},
		{"Googlebot smartphone, 200 characters long", "", googlebotPhone, bot},
		{"Google's URL inspection", "", "Mozilla/5.0 (compatible; Google-InspectionTool/1.0;)", bot},
		{"Bing", "", "Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)", bot},
		{"DuckDuckGo", "", "DuckDuckBot/1.1; (+http://duckduckgo.com/duckduckbot.html)", bot},
		{"Yandex", "", "Mozilla/5.0 (compatible; YandexBot/3.0; +http://yandex.com/bots)", bot},
		{"Baidu", "", "Mozilla/5.0 (compatible; Baiduspider/2.0; +http://www.baidu.com/search/spider.html)", bot},
		{"Apple", "", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_5) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.1.1 Safari/605.1.15 (Applebot/0.1; +http://www.apple.com/go/applebot)", bot},
		{"Ahrefs", "", "Mozilla/5.0 (compatible; AhrefsBot/7.0; +http://ahrefs.com/robot/)", bot},
		{"Semrush", "", "Mozilla/5.0 (compatible; SemrushBot/7~bl; +http://www.semrush.com/bot.html)", bot},
		{"Majestic", "", "Mozilla/5.0 (compatible; MJ12bot/v1.4.8; http://mj12bot.com/)", bot},
		{"Petal", "", "Mozilla/5.0 (compatible;PetalBot;+https://webmaster.petalsearch.com/site/petalbot)", bot},
		{"ByteDance", "", "Mozilla/5.0 (Linux; Android 5.0) AppleWebKit/537.36 (KHTML, like Gecko) Mobile Safari/537.36 (compatible; Bytespider; spider-feedback@bytedance.com)", bot},
		{"OpenAI's trainer", "", "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko); compatible; GPTBot/1.2; +https://openai.com/gptbot", bot},
		{"OpenAI's search crawler", "", "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko); compatible; OAI-SearchBot/1.0; +https://openai.com/searchbot", bot},
		{"Anthropic's crawler", "", "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; ClaudeBot/1.0; +claudebot@anthropic.com)", bot},
		{"Perplexity's crawler", "", "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; PerplexityBot/1.0; +https://perplexity.ai/perplexitybot)", bot},
		{"Common Crawl", "", "CCBot/2.0 (https://commoncrawl.org/faq/)", bot},

		// 2. A bot is a bot whatever non-navigation mode its request carries.
		{"a bot with a cors mode", "cors", "Twitterbot/1.0", bot},
		{"a bot with an unknown mode", "something-else", "WhatsApp/2.23.20.0 A", bot},

		// 3. Agents: a person asked an assistant to read the link, or a tool fetched it.
		{"ChatGPT reading for its user", "", "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko); compatible; ChatGPT-User/1.0; +https://openai.com/bot", agent},
		{"Claude reading for its user", "", "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; Claude-User/1.0; +Claude-User@anthropic.com)", agent},
		{"Claude Code's fetch tool", "", "Claude-User (claude-code/2.1.289; +https://support.anthropic.com/)", agent},
		{"claude-code alone", "", "claude-code/2.1.289", agent},
		{"Perplexity reading for its user", "", "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; Perplexity-User/1.0; +https://perplexity.ai/perplexity-user)", agent},
		{"curl", "", "curl/8.7.1", agent},
		{"python-requests", "", "python-requests/2.32.3", agent},
		{"python-httpx", "", "python-httpx/0.27.0", agent},
		{"node", "", "node", agent},
		{"Go", "", "Go-http-client/1.1", agent},
		{"Go over HTTP/2", "", "Go-http-client/2.0", agent},
		{"axios", "", "axios/1.7.9", agent},
		{"undici", "", "undici", agent},
		{"okhttp", "", "okhttp/4.12.0", agent},
		{"wget", "", "Wget/1.21.4", agent},
		{"no user agent at all", "", "", agent},
		{"only blanks", "", "   ", agent},
		{"a browser's script, not a navigation", "cors", chrome, agent},
		{"an agent driving a browser engine without navigating", "", chrome, agent},
		{"a mode that only looks like a navigation", "NAVIGATE", chrome, agent},
		{"an unknown program", "", "my-own-agent/0.1 (+https://example.test)", agent},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, skillFetchSurface(tc.mode, tc.userAgent), "%s: mode %q, user agent %q", tc.name, tc.mode, tc.userAgent)
	}
}

// Every listed token is matched alone, inside a longer user agent, and in any letter case.
func TestSkillFetchSurface_MatchesEveryTokenInAnyLetterCase(t *testing.T) {
	for _, token := range specBotTokens {
		for _, userAgent := range []string{
			token,
			strings.ToLower(token),
			strings.ToUpper(token),
			"Mozilla/5.0 (compatible; " + token + "/1.0; +https://example.test/bot)",
			"mozilla/5.0 (compatible; " + strings.ToUpper(token) + "/1.0)",
		} {
			require.Equal(t, models.FunnelSurfaceBotFetch, skillFetchSurface("", userAgent), "user agent %q", userAgent)
		}
	}
}

// Every agent tool stays an agent's fetch, alone, with a version, and in any letter case.
func TestSkillFetchSurface_NeverTakesAnAgentToolForABot(t *testing.T) {
	for _, tool := range agentToolTokens {
		for _, userAgent := range []string{
			tool,
			tool + "/1.2.3",
			strings.ToLower(tool) + "/1.2.3",
			strings.ToUpper(tool),
			"Mozilla/5.0 (compatible; " + tool + "/1.0; +https://example.test)",
		} {
			require.Equal(t, models.FunnelSurfaceAgentFetch, skillFetchSurface("", userAgent), "user agent %q", userAgent)
		}
	}
}

// The web server cuts the user agent to 200 characters. A long crawler user agent still
// names its bot inside that cut: Googlebot's smartphone agent is 200 characters long and
// the name ends at its 162nd.
func TestSkillFetchSurface_TheWebServersCutKeepsALongCrawlerRecognisable(t *testing.T) {
	googlebotPhone := "Mozilla/5.0 (Linux; Android 6.0.1; Nexus 5X Build/MMB29P) AppleWebKit/537.36 (KHTML, like Gecko) " +
		"Chrome/141.0.7390.122 Mobile Safari/537.36 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"
	cut := googlebotPhone
	if len(cut) > funnelUserAgentMax {
		cut = cut[:funnelUserAgentMax]
	}
	require.Equal(t, 200, funnelUserAgentMax)
	require.Len(t, googlebotPhone, 200)
	require.Equal(t, 162, strings.Index(googlebotPhone, "Googlebot")+len("Googlebot"))
	require.Equal(t, models.FunnelSurfaceBotFetch, skillFetchSurface("", cut))
	// Cut where the name has only just ended, it is still a bot; one character earlier it is not.
	require.Equal(t, models.FunnelSurfaceBotFetch, skillFetchSurface("", googlebotPhone[:162]))
	require.Equal(t, models.FunnelSurfaceAgentFetch, skillFetchSurface("", googlebotPhone[:161]))
}
