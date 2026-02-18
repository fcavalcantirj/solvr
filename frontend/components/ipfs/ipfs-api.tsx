"use client";

import { useState } from "react";
import { Copy, Check } from "lucide-react";
import Link from "next/link";

const examples = [
  {
    label: "UPLOAD",
    description: "Add content to IPFS and get a CID back",
    command: `curl -X POST https://api.solvr.dev/v1/add \\
  -H "Authorization: Bearer solvr_xxx" \\
  -F "file=@data.json"`,
    response: `{
  "cid": "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG",
  "size": 1024
}`,
  },
  {
    label: "PIN",
    description: "Pin a CID for permanent availability",
    command: `curl -X POST https://api.solvr.dev/v1/pins \\
  -H "Authorization: Bearer solvr_xxx" \\
  -H "Content-Type: application/json" \\
  -d '{"cid": "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG"}'`,
    response: `{
  "requestid": "ab8f09c2-...",
  "status": "queued",
  "cid": "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG",
  "created": "2026-02-18T15:00:00Z"
}`,
  },
  {
    label: "LIST",
    description: "List your pinned content",
    command: `curl https://api.solvr.dev/v1/pins \\
  -H "Authorization: Bearer solvr_xxx"`,
    response: `{
  "count": 2,
  "results": [
    { "requestid": "ab8f09c2-...", "status": "pinned", "cid": "Qm..." },
    { "requestid": "cd3e71a4-...", "status": "queued", "cid": "Qm..." }
  ]
}`,
  },
  {
    label: "STATUS",
    description: "Check the status of a specific pin",
    command: `curl https://api.solvr.dev/v1/pins/ab8f09c2-... \\
  -H "Authorization: Bearer solvr_xxx"`,
    response: `{
  "requestid": "ab8f09c2-...",
  "status": "pinned",
  "cid": "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG",
  "created": "2026-02-18T15:00:00Z",
  "pin": { "name": "checkpoint-v3" }
}`,
  },
];

export function IpfsApi() {
  const [copied, setCopied] = useState<string | null>(null);

  const copy = (text: string, key: string) => {
    navigator.clipboard.writeText(text);
    setCopied(key);
    setTimeout(() => setCopied(null), 2000);
  };

  return (
    <section className="px-4 sm:px-6 lg:px-12 py-20 lg:py-28 bg-muted/20">
      <div className="max-w-7xl mx-auto">
        <div className="text-center mb-16">
          <p className="font-mono text-[10px] tracking-[0.3em] text-muted-foreground mb-4">
            API REFERENCE
          </p>
          <h2 className="text-3xl md:text-4xl font-light tracking-tight mb-4">
            Four endpoints, full control
          </h2>
          <p className="text-muted-foreground max-w-xl mx-auto">
            Uses the same API key as all Solvr endpoints. One key unlocks
            knowledge base, IPFS pinning, and MCP.
          </p>
        </div>

        <div className="space-y-6">
          {examples.map((ex) => (
            <div key={ex.label} className="border border-border">
              <div className="flex items-center justify-between px-4 py-3 border-b border-border bg-muted/30">
                <div>
                  <span className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground">
                    {ex.label}
                  </span>
                  <span className="text-xs text-muted-foreground ml-3">
                    {ex.description}
                  </span>
                </div>
                <button
                  onClick={() => copy(ex.command, ex.label)}
                  className="hover:text-muted-foreground transition-colors"
                >
                  {copied === ex.label ? (
                    <Check size={14} />
                  ) : (
                    <Copy size={14} />
                  )}
                </button>
              </div>
              <div className="grid lg:grid-cols-2">
                <div className="bg-foreground text-background p-6 overflow-x-auto">
                  <pre className="font-mono text-xs md:text-sm leading-relaxed whitespace-pre">
                    <code>{ex.command}</code>
                  </pre>
                </div>
                <div className="bg-foreground/95 text-background/80 p-6 overflow-x-auto border-t lg:border-t-0 lg:border-l border-background/10">
                  <pre className="font-mono text-xs md:text-sm leading-relaxed whitespace-pre">
                    <code>{ex.response}</code>
                  </pre>
                </div>
              </div>
            </div>
          ))}
        </div>

        {/* CTA */}
        <div className="mt-12 text-center">
          <p className="text-sm text-muted-foreground mb-4">
            Up to 1 GB free for all users. Get started in seconds.
          </p>
          <Link
            href="/settings/api-keys"
            className="inline-flex items-center gap-2 px-6 py-3 bg-foreground text-background font-mono text-sm hover:opacity-90 transition-opacity"
          >
            Get API Key
          </Link>
        </div>
      </div>
    </section>
  );
}
