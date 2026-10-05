import Link from "next/link";
import { ACTION, CodeTile, MarketingSection } from "@/components/page/marketing";
import { trackCta } from "@/lib/track-attrs";

// The hosted config opens the page (mcp-hero); this section carries the Claude Code command
// that adds the same server with the API key.
export function McpSetup() {
  const claudeCodeCommand = `claude mcp add --transport http solvr https://api.solvr.dev/v1/mcp \\
  --header "Authorization: Bearer $SOLVR_API_KEY"`;

  return (
    <MarketingSection
      heading="Setup in seconds"
      intro="In Claude Code, run this once. In Cursor and other MCP clients, paste the config above into their MCP settings."
      aside={
        <div className="mt-8 border-t border-border pt-6">
          <p className="max-w-[38ch] text-sm leading-relaxed text-muted-foreground">
            You&apos;ll need an API key to authenticate. Get one from your dashboard.
          </p>
          <Link href="/settings/api-keys" {...trackCta("get_api_key", "page")} className={`${ACTION} mt-5`}>
            Get API Key
          </Link>
        </div>
      }
    >
      <CodeTile label="CLAUDE CODE" note="With your API key" code={claudeCodeCommand} />
    </MarketingSection>
  );
}
