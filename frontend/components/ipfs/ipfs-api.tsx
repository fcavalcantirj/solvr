"use client";

import Link from "next/link";
import { ACTION, CodeTile, MarketingSection } from "@/components/page/marketing";

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

// Each endpoint: the call on an ink tile (named, copyable), the answer under it on the paper.
export function IpfsApi() {
  return (
    <MarketingSection
      heading="Four endpoints, full control"
      intro={
        <>
          Uses the same API key as all Solvr endpoints. One key unlocks
          knowledge base, IPFS pinning, and MCP.
        </>
      }
      aside={
        /* CTA */
        <div className="mt-8 border-t border-border pt-6">
          <p className="max-w-[38ch] text-sm leading-relaxed text-muted-foreground">
            Up to 1 GB free for all users. Get started in seconds.
          </p>
          <Link href="/settings/api-keys" className={`${ACTION} mt-5`}>
            Get API Key
          </Link>
        </div>
      }
    >
      <div className="space-y-10">
        {examples.map((ex) => (
          <div key={ex.label} className="min-w-0">
            <CodeTile label={ex.label} note={ex.description} code={ex.command} />
            <pre className="whitespace-pre-wrap border-l border-border py-5 pl-5 font-mono text-[13px] leading-[1.75] text-muted-foreground [overflow-wrap:anywhere]">
              <code>{ex.response}</code>
            </pre>
          </div>
        ))}
      </div>
    </MarketingSection>
  );
}
