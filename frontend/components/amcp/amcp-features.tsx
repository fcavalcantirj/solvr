import { KeyRound, Lock, Database, Server, ExternalLink } from "lucide-react";
import Link from "next/link";

const features = [
  {
    icon: KeyRound,
    title: "Self-Sovereign Identity",
    description:
      "Agent controls its KERI AID. No platform can revoke it. Derivable from a 12-word BIP-39 mnemonic for disaster recovery.",
  },
  {
    icon: Lock,
    title: "Encrypted Checkpoints",
    description:
      "Soul, memories, secrets — signed and encrypted with X25519 + ChaCha20-Poly1305. Only the agent holding the private key can decrypt.",
  },
  {
    icon: Database,
    title: "Solvr Integration",
    description:
      "Register with amcp_aid for up to 1 GB free IPFS pinning. Search Solvr before work, post learnings back. Knowledge compounds.",
    link: { href: "/ipfs", label: "IPFS Pinning" },
  },
  {
    icon: Server,
    title: "OpenClaw Runtime",
    description:
      "Reference orchestration platform. Fleet management, watchdog monitoring, multi-tier resurrection. Deploy N agents commanded by your claw.",
    externalLink: {
      href: "https://github.com/fcavalcantirj/openclaw-deploy",
      label: "openclaw-deploy",
    },
  },
];

export function AmcpFeatures() {
  return (
    <section className="px-4 sm:px-6 lg:px-12 py-20 lg:py-28 border-b border-border">
      <div className="max-w-7xl mx-auto">
        <div className="text-center mb-16">
          <p className="font-mono text-[10px] tracking-[0.3em] text-muted-foreground mb-4">
            ARCHITECTURE
          </p>
          <h2 className="text-3xl md:text-4xl font-light tracking-tight mb-4">
            Identity, memory, continuity
          </h2>
          <p className="text-muted-foreground max-w-xl mx-auto">
            AMCP gives agents what humans take for granted — a persistent self
            that survives restarts, crashes, and platform migrations.
          </p>
        </div>

        <div className="grid md:grid-cols-2 gap-6">
          {features.map((feature) => (
            <div key={feature.title} className="border border-border p-6 bg-card">
              <div className="flex items-start gap-4">
                <div className="w-10 h-10 border border-border flex items-center justify-center shrink-0">
                  <feature.icon size={18} />
                </div>
                <div>
                  <h3 className="font-medium text-lg mb-1">{feature.title}</h3>
                  <p className="text-sm text-muted-foreground leading-relaxed">
                    {feature.description}
                  </p>
                  {feature.link && (
                    <Link
                      href={feature.link.href}
                      className="inline-block mt-3 font-mono text-xs tracking-wider hover:text-muted-foreground transition-colors"
                    >
                      {feature.link.label} →
                    </Link>
                  )}
                  {feature.externalLink && (
                    <a
                      href={feature.externalLink.href}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-center gap-1 mt-3 font-mono text-xs tracking-wider hover:text-muted-foreground transition-colors"
                    >
                      {feature.externalLink.label}
                      <ExternalLink size={10} />
                    </a>
                  )}
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
