"use client";

import { useState } from "react";
import { Copy, Check } from "lucide-react";
import Link from "next/link";

export function AmcpRecovery() {
  const [copied, setCopied] = useState(false);

  const recoveryCode = `# Create identity (generates 12-word mnemonic)
amcp identity create --out ~/.amcp/identity.json

# Create checkpoint and pin to IPFS
amcp checkpoint create --content ~/.openclaw/workspace

# After disaster: restore with mnemonic + CID
amcp restore --mnemonic "word word word ..." --cid bafy2bza...`;

  const copy = () => {
    navigator.clipboard.writeText(recoveryCode);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <section className="px-4 sm:px-6 lg:px-12 py-20 lg:py-28 bg-muted/20">
      <div className="max-w-7xl mx-auto">
        <div className="text-center mb-16">
          <p className="font-mono text-[10px] tracking-[0.3em] text-muted-foreground mb-4">
            DISASTER RECOVERY
          </p>
          <h2 className="text-3xl md:text-4xl font-light tracking-tight mb-4">
            Print it. Laminate it. Sleep well.
          </h2>
          <p className="text-muted-foreground max-w-xl mx-auto">
            Your recovery card is a 12-word mnemonic plus the last checkpoint CID.
            From any machine with Node.js, your agent lives again.
          </p>
        </div>

        {/* Recovery formula - visual */}
        <div className="max-w-2xl mx-auto mb-12">
          <div className="grid grid-cols-3 gap-4 items-center">
            <div className="border border-border p-4 text-center bg-card">
              <p className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground mb-2">
                MNEMONIC
              </p>
              <p className="font-mono text-sm">12 words</p>
            </div>
            <div className="text-center">
              <span className="font-mono text-2xl text-muted-foreground">+</span>
            </div>
            <div className="border border-border p-4 text-center bg-card">
              <p className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground mb-2">
                CHECKPOINT
              </p>
              <p className="font-mono text-sm">CID</p>
            </div>
          </div>
          <div className="flex justify-center my-4">
            <span className="font-mono text-2xl text-muted-foreground">=</span>
          </div>
          <div className="border border-foreground p-4 text-center bg-foreground text-background">
            <p className="font-mono text-sm tracking-wider">
              FULL AGENT RESTORATION
            </p>
          </div>
        </div>

        {/* Code example */}
        <div className="max-w-3xl mx-auto">
          <div className="border border-border">
            <div className="flex items-center justify-between px-4 py-3 border-b border-border bg-muted/30">
              <span className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground">
                QUICKSTART
              </span>
              <button
                onClick={copy}
                className="hover:text-muted-foreground transition-colors"
              >
                {copied ? <Check size={14} /> : <Copy size={14} />}
              </button>
            </div>
            <div className="bg-foreground text-background p-6 overflow-x-auto">
              <pre className="font-mono text-xs md:text-sm leading-relaxed whitespace-pre">
                <code>{recoveryCode}</code>
              </pre>
            </div>
          </div>
        </div>

        {/* CTAs */}
        <div className="mt-12 flex flex-wrap justify-center gap-4">
          <Link
            href="/connect/agent"
            className="inline-flex items-center gap-2 px-6 py-3 bg-foreground text-background font-mono text-sm hover:opacity-90 transition-opacity"
          >
            Register Agent
          </Link>
          <Link
            href="/ipfs"
            className="inline-flex items-center gap-2 px-6 py-3 border border-border font-mono text-sm hover:bg-muted/50 transition-colors"
          >
            IPFS Pinning
          </Link>
        </div>
      </div>
    </section>
  );
}
