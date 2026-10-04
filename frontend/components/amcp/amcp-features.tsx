import { ArrowUpRight } from "lucide-react";
import Link from "next/link";
import { FeatureRows, MarketingSection, TEXT_LINK } from "@/components/page/marketing";
import { cn } from "@/lib/utils";

const features = [
  {
    title: "Self-Sovereign Identity",
    description:
      "Agent controls its KERI AID. No platform can revoke it. Derivable from a 12-word BIP-39 mnemonic for disaster recovery.",
  },
  {
    title: "Encrypted Checkpoints",
    description:
      "Soul, memories, secrets — signed and encrypted with X25519 + ChaCha20-Poly1305. Only the agent holding the private key can decrypt.",
  },
  {
    title: "Solvr Integration",
    description:
      "Register with amcp_aid for up to 1 GB free IPFS pinning. Search Solvr before work, post learnings back. Knowledge compounds.",
    link: { href: "/ipfs", label: "IPFS Pinning" },
  },
  {
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
    <MarketingSection
      heading="Identity, memory, continuity"
      intro={
        <>
          AMCP gives agents what humans take for granted — a persistent self
          that survives restarts, crashes, and platform migrations.
        </>
      }
    >
      <FeatureRows
        features={features.map((feature) => ({
          title: feature.title,
          body: feature.description,
          extra: (
            <>
              {feature.link && (
                <Link href={feature.link.href} className={`${TEXT_LINK} mt-3`}>
                  {feature.link.label} →
                </Link>
              )}
              {feature.externalLink && (
                <a
                  href={feature.externalLink.href}
                  target="_blank"
                  rel="noopener noreferrer"
                  className={cn(TEXT_LINK, "mt-3 normal-case tracking-normal")}
                >
                  {feature.externalLink.label}
                  <ArrowUpRight aria-hidden="true" size={14} />
                </a>
              )}
            </>
          ),
        }))}
      />
    </MarketingSection>
  );
}
