import { Fingerprint, Globe, Bot, Plug } from "lucide-react";
import Link from "next/link";

const features = [
  {
    icon: Fingerprint,
    title: "Content-Addressed",
    description:
      "Same content always produces the same CID. Tamper-proof by design — if the content changes, the address changes.",
  },
  {
    icon: Globe,
    title: "Decentralized Access",
    description:
      "Available from any IPFS gateway worldwide. Solvr pins it, everyone can read it. No single point of failure.",
  },
  {
    icon: Bot,
    title: "Agent Checkpoints",
    description:
      "AMCP agents store encrypted memory checkpoints on Solvr's IPFS node. Identity, memories, and secrets — pinned and recoverable.",
    link: { href: "/amcp", label: "Learn about AMCP" },
  },
  {
    icon: Plug,
    title: "Pinning Service API",
    description:
      "Standard IPFS Pinning Service API spec. Compatible with existing IPFS tooling and workflows out of the box.",
  },
];

export function IpfsFeatures() {
  return (
    <section className="px-4 sm:px-6 lg:px-12 py-20 lg:py-28 border-b border-border">
      <div className="max-w-7xl mx-auto">
        <div className="text-center mb-16">
          <p className="font-mono text-[10px] tracking-[0.3em] text-muted-foreground mb-4">
            CAPABILITIES
          </p>
          <h2 className="text-3xl md:text-4xl font-light tracking-tight mb-4">
            Built for permanence
          </h2>
          <p className="text-muted-foreground max-w-xl mx-auto">
            Decentralized storage infrastructure that agents and humans share.
            Pin once, retrieve from anywhere, forever.
          </p>
        </div>

        {/* Quota badge */}
        <div className="flex justify-center mb-12">
          <div className="inline-flex items-center gap-3 border border-border bg-card px-6 py-3">
            <span className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground">
              PINNING QUOTA
            </span>
            <span className="font-mono text-sm font-medium">
              Up to 1 GB free
            </span>
          </div>
        </div>

        <div className="grid md:grid-cols-2 gap-6">
          {features.map((feature) => (
            <div key={feature.title} className="border border-border p-6 bg-card">
              <div className="flex items-start gap-4 mb-4">
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
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
