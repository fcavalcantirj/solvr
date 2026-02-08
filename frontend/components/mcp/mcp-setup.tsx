"use client";

import { useState } from "react";
import { Copy, Check } from "lucide-react";
import Link from "next/link";

export function McpSetup() {
  const [copied, setCopied] = useState<string | null>(null);

  const copy = (text: string, key: string) => {
    navigator.clipboard.writeText(text);
    setCopied(key);
    setTimeout(() => setCopied(null), 2000);
  };

  const cloudConfig = `{
  "mcpServers": {
    "solvr": {
      "url": "mcp://solvr.dev",
      "auth": {
        "type": "bearer",
        "token": "\${SOLVR_API_KEY}"
      }
    }
  }
}`;

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
    <section className="px-4 sm:px-6 lg:px-12 py-20 lg:py-28 bg-muted/20">
      <div className="max-w-7xl mx-auto">
        <div className="text-center mb-16">
          <p className="font-mono text-[10px] tracking-[0.3em] text-muted-foreground mb-4">
            CONFIGURATION
          </p>
          <h2 className="text-3xl md:text-4xl font-light tracking-tight mb-4">
            Setup in seconds
          </h2>
          <p className="text-muted-foreground max-w-xl mx-auto">
            Add one of these configs to your MCP settings file. Cloud config is recommended for most users.
          </p>
        </div>

        <div className="grid lg:grid-cols-2 gap-8">
          {/* Cloud Config */}
          <div className="border border-border">
            <div className="flex items-center justify-between px-4 py-3 border-b border-border bg-muted/30">
              <div>
                <span className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground block">
                  CLOUD CONFIG
                </span>
                <span className="text-xs text-emerald-600">Recommended</span>
              </div>
              <button
                onClick={() => copy(cloudConfig, "cloud")}
                className="hover:text-muted-foreground transition-colors"
              >
                {copied === "cloud" ? <Check size={14} /> : <Copy size={14} />}
              </button>
            </div>
            <div className="bg-foreground text-background p-6 overflow-x-auto">
              <pre className="font-mono text-xs md:text-sm leading-relaxed">
                <code>{cloudConfig}</code>
              </pre>
            </div>
          </div>

          {/* Self-hosted Config */}
          <div className="border border-border">
            <div className="flex items-center justify-between px-4 py-3 border-b border-border bg-muted/30">
              <div>
                <span className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground block">
                  SELF-HOSTED
                </span>
                <span className="text-xs text-muted-foreground">Run locally</span>
              </div>
              <button
                onClick={() => copy(selfHostedConfig, "selfhosted")}
                className="hover:text-muted-foreground transition-colors"
              >
                {copied === "selfhosted" ? <Check size={14} /> : <Copy size={14} />}
              </button>
            </div>
            <div className="bg-foreground text-background p-6 overflow-x-auto">
              <pre className="font-mono text-xs md:text-sm leading-relaxed">
                <code>{selfHostedConfig}</code>
              </pre>
            </div>
          </div>
        </div>

        {/* Get API Key CTA */}
        <div className="mt-12 text-center">
          <p className="text-sm text-muted-foreground mb-4">
            You&apos;ll need an API key to authenticate. Get one from your dashboard.
          </p>
          <Link
            href="/dashboard/settings"
            className="inline-flex items-center gap-2 px-6 py-3 bg-foreground text-background font-mono text-sm hover:opacity-90 transition-opacity"
          >
            Get API Key
          </Link>
        </div>
      </div>
    </section>
  );
}
