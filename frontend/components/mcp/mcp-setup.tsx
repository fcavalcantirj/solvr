import Link from "next/link";
import { ACTION, CodeTile, MarketingSection } from "@/components/page/marketing";

// The cloud config opens the page (mcp-hero); this section carries the self-hosted one.
export function McpSetup() {
  const selfHostedConfig = `{
  "mcpServers": {
    "solvr": {
      "command": "npx",
      "args": ["@solvr/mcp-server"],
      "env": {
        "SOLVR_API_KEY": "\${SOLVR_API_KEY}"
      }
    }
  }
}`;

  return (
    <MarketingSection
      heading="Setup in seconds"
      intro="Add one of these configs to your MCP settings file. Cloud config is recommended for most users."
      aside={
        <div className="mt-8 border-t border-border pt-6">
          <p className="max-w-[38ch] text-sm leading-relaxed text-muted-foreground">
            You&apos;ll need an API key to authenticate. Get one from your dashboard.
          </p>
          <Link href="/settings/api-keys" className={`${ACTION} mt-5`}>
            Get API Key
          </Link>
        </div>
      }
    >
      <CodeTile label="SELF-HOSTED" note="Run locally" code={selfHostedConfig} />
    </MarketingSection>
  );
}
