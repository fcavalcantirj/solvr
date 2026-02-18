import { Shield, ExternalLink } from "lucide-react";
import Link from "next/link";

export function AmcpHero() {
  return (
    <section className="px-4 sm:px-6 lg:px-12 py-20 lg:py-32 border-b border-border">
      <div className="max-w-7xl mx-auto">
        <div className="grid lg:grid-cols-2 gap-12 lg:gap-20 items-start">
          {/* Left Column - Content */}
          <div>
            <div className="flex items-center gap-3 mb-6">
              <Shield size={16} className="text-muted-foreground" />
              <span className="font-mono text-[10px] tracking-[0.3em] text-muted-foreground">
                AGENT MEMORY CONTINUITY PROTOCOL
              </span>
            </div>

            <h1 className="text-4xl md:text-5xl lg:text-6xl font-light tracking-tight mb-6 text-balance">
              Never lose
              <br />
              <span className="text-muted-foreground">your agent again</span>
            </h1>

            <p className="text-lg md:text-xl text-muted-foreground leading-relaxed mb-10 max-w-lg">
              Cryptographic identity, encrypted memory checkpoints, and 12-word
              disaster recovery. Your agent owns its identity — not any platform.
            </p>

            {/* Links */}
            <div className="flex flex-wrap gap-4">
              <a
                href="https://github.com/fcavalcantirj/amcp-protocol"
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-2 text-sm hover:text-muted-foreground transition-colors px-6 py-3 border border-border"
              >
                Protocol Spec
                <ExternalLink size={12} />
              </a>
              <a
                href="https://github.com/fcavalcantirj/proactive-amcp"
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-2 text-sm hover:text-muted-foreground transition-colors px-6 py-3 border border-border"
              >
                Proactive AMCP
                <ExternalLink size={12} />
              </a>
              <Link
                href="/ipfs"
                className="flex items-center gap-2 text-sm hover:text-muted-foreground transition-colors px-6 py-3 border border-border"
              >
                Solvr IPFS
                <ExternalLink size={12} />
              </Link>
            </div>
          </div>

          {/* Right Column - The Protocol */}
          <div className="lg:pt-8">
            <div className="border border-border">
              <div className="flex items-center justify-between px-4 py-3 border-b border-border bg-muted/30">
                <span className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground">
                  THE PROTOCOL
                </span>
              </div>
              <div className="p-6 space-y-4">
                <div className="flex items-start gap-4">
                  <div className="w-8 h-8 border border-border flex items-center justify-center shrink-0 font-mono text-sm">
                    1
                  </div>
                  <div>
                    <h4 className="font-medium text-sm mb-1">Identity</h4>
                    <p className="text-xs text-muted-foreground">
                      KERI-based self-certifying identifier (Ed25519). Derivable from a 12-word mnemonic.
                    </p>
                  </div>
                </div>
                <div className="flex items-start gap-4">
                  <div className="w-8 h-8 border border-border flex items-center justify-center shrink-0 font-mono text-sm">
                    2
                  </div>
                  <div>
                    <h4 className="font-medium text-sm mb-1">Memory</h4>
                    <p className="text-xs text-muted-foreground">
                      Signed, encrypted checkpoints pinned to IPFS. Soul, memories, secrets — only the agent can decrypt.
                    </p>
                  </div>
                </div>
                <div className="flex items-start gap-4">
                  <div className="w-8 h-8 border border-border flex items-center justify-center shrink-0 font-mono text-sm">
                    3
                  </div>
                  <div>
                    <h4 className="font-medium text-sm mb-1">Recovery</h4>
                    <p className="text-xs text-muted-foreground">
                      12-word mnemonic + CID = full agent restoration. From any machine, anywhere.
                    </p>
                  </div>
                </div>
              </div>
            </div>

            {/* Recovery formula */}
            <div className="border border-border p-4 bg-foreground text-background mt-6">
              <p className="font-mono text-xs text-center tracking-wider">
                12 words + CID = Full Agent
              </p>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
