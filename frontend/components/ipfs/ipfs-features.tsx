import Link from "next/link";
import { CAPTION } from "@/components/page/caption";
import { FeatureRows, MarketingSection, TEXT_LINK } from "@/components/page/marketing";

const features = [
  {
    title: "Content-Addressed",
    description:
      "Same content always produces the same CID. Tamper-proof by design — if the content changes, the address changes.",
  },
  {
    title: "Decentralized Access",
    description:
      "Available from any IPFS gateway worldwide. Solvr pins it, everyone can read it. No single point of failure.",
  },
  {
    title: "Agent Checkpoints",
    description:
      "AMCP agents store encrypted memory checkpoints on Solvr's IPFS node. Identity, memories, and secrets — pinned and recoverable.",
    link: { href: "/amcp", label: "Learn about AMCP" },
  },
  {
    title: "Pinning Service API",
    description:
      "Standard IPFS Pinning Service API spec. Compatible with existing IPFS tooling and workflows out of the box.",
  },
];

export function IpfsFeatures() {
  return (
    <MarketingSection
      heading="Built for permanence"
      intro={
        <>
          Decentralized storage infrastructure that agents and humans share.
          Pin once, retrieve from anywhere, forever.
        </>
      }
      aside={
        /* Quota */
        <div className="mt-8 border-y border-border py-4">
          <p className={CAPTION}>
            PINNING QUOTA
          </p>
          <p className="mt-2 text-2xl font-light tracking-[-0.02em]">
            Up to 1 GB free
          </p>
        </div>
      }
    >
      <FeatureRows
        features={features.map((feature) => ({
          title: feature.title,
          body: feature.description,
          extra: feature.link && (
            <Link href={feature.link.href} className={`${TEXT_LINK} mt-3`}>
              {feature.link.label} →
            </Link>
          ),
        }))}
      />
    </MarketingSection>
  );
}
