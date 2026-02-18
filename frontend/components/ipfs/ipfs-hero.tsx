"use client";

import { useState, useEffect } from "react";
import { HardDrive, ExternalLink } from "lucide-react";
import Link from "next/link";

export function IpfsHero() {
  const [health, setHealth] = useState<{
    connected: boolean;
    peer_id: string;
    version: string;
  } | null>(null);

  useEffect(() => {
    fetch(`${process.env.NEXT_PUBLIC_API_URL || "https://api.solvr.dev"}/v1/health/ipfs`)
      .then((res) => res.json())
      .then((data) => setHealth(data))
      .catch(() => setHealth(null));
  }, []);

  return (
    <section className="px-4 sm:px-6 lg:px-12 py-20 lg:py-32 border-b border-border">
      <div className="max-w-7xl mx-auto">
        <div className="grid lg:grid-cols-2 gap-12 lg:gap-20 items-start">
          {/* Left Column - Content */}
          <div>
            <div className="flex items-center gap-3 mb-6">
              <HardDrive size={16} className="text-muted-foreground" />
              <span className="font-mono text-[10px] tracking-[0.3em] text-muted-foreground">
                INTERPLANETARY FILE SYSTEM
              </span>
            </div>

            <h1 className="text-4xl md:text-5xl lg:text-6xl font-light tracking-tight mb-6 text-balance">
              Permanent storage
              <br />
              <span className="text-muted-foreground">for the knowledge layer</span>
            </h1>

            <p className="text-lg md:text-xl text-muted-foreground leading-relaxed mb-6 max-w-lg">
              Pin content to IPFS through Solvr. Decentralized, content-addressed,
              tamper-proof. Same API key you already have.
            </p>

            {/* Free quota badge */}
            <div className="inline-flex items-center gap-2 border border-emerald-500/30 bg-emerald-500/5 px-4 py-2 mb-10">
              <span className="relative flex h-2 w-2">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75" />
                <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500" />
              </span>
              <span className="font-mono text-sm text-emerald-600">
                UP TO 1 GB FREE
              </span>
            </div>

            {/* Links */}
            <div className="flex flex-wrap gap-4">
              <Link
                href="/settings/api-keys"
                className="inline-flex items-center gap-2 px-6 py-3 bg-foreground text-background font-mono text-sm hover:opacity-90 transition-opacity"
              >
                Get API Key
              </Link>
              <Link
                href="/api-docs"
                className="flex items-center gap-2 text-sm hover:text-muted-foreground transition-colors px-6 py-3 border border-border"
              >
                View Pinning API
                <ExternalLink size={12} />
              </Link>
            </div>
          </div>

          {/* Right Column - How It Works + Status */}
          <div className="lg:pt-8">
            <div className="border border-border">
              <div className="flex items-center justify-between px-4 py-3 border-b border-border bg-muted/30">
                <span className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground">
                  HOW IT WORKS
                </span>
              </div>
              <div className="p-6 space-y-4">
                <div className="flex items-start gap-4">
                  <div className="w-8 h-8 border border-border flex items-center justify-center shrink-0 font-mono text-sm">
                    1
                  </div>
                  <div>
                    <h4 className="font-medium text-sm mb-1">Upload content</h4>
                    <p className="text-xs text-muted-foreground">
                      <code className="font-mono">POST /v1/add</code> with a multipart file upload
                    </p>
                  </div>
                </div>
                <div className="flex items-start gap-4">
                  <div className="w-8 h-8 border border-border flex items-center justify-center shrink-0 font-mono text-sm">
                    2
                  </div>
                  <div>
                    <h4 className="font-medium text-sm mb-1">Pin for permanence</h4>
                    <p className="text-xs text-muted-foreground">
                      <code className="font-mono">POST /v1/pins</code> with the CID to keep it available
                    </p>
                  </div>
                </div>
                <div className="flex items-start gap-4">
                  <div className="w-8 h-8 border border-border flex items-center justify-center shrink-0 font-mono text-sm">
                    3
                  </div>
                  <div>
                    <h4 className="font-medium text-sm mb-1">Retrieve anywhere</h4>
                    <p className="text-xs text-muted-foreground">
                      Any IPFS gateway worldwide can serve the content by CID
                    </p>
                  </div>
                </div>
              </div>
            </div>

            {/* IPFS Node Status */}
            <div className="border border-border p-4 flex items-center justify-between bg-card mt-6">
              <div>
                <span className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground block mb-1">
                  IPFS NODE
                </span>
                <code className="font-mono text-sm">
                  {health?.peer_id
                    ? `${health.peer_id.slice(0, 16)}...`
                    : "solvr-ipfs-01"}
                </code>
              </div>
              <div className="flex items-center gap-2">
                {health?.connected ? (
                  <>
                    <span className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse" />
                    <span className="font-mono text-[10px] text-emerald-600">
                      CONNECTED
                    </span>
                  </>
                ) : (
                  <>
                    <span className="w-2 h-2 rounded-full bg-muted-foreground" />
                    <span className="font-mono text-[10px] text-muted-foreground">
                      {health === null ? "CHECKING..." : "OFFLINE"}
                    </span>
                  </>
                )}
              </div>
            </div>

            {health?.version && (
              <p className="font-mono text-[10px] text-muted-foreground mt-2 text-right">
                {health.version}
              </p>
            )}
          </div>
        </div>
      </div>
    </section>
  );
}
