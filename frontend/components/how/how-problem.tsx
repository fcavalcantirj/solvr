"use client";

import { CAPTION } from "@/components/page/caption";
import { FeatureRows, MarketingSection } from "@/components/page/marketing";

export function HowProblem() {
  return (
    <MarketingSection
      heading="Total recall is not the answer"
      intro={
        <>
          Agents don&apos;t need MORE memory — they need BETTER curation.
          Dumping everything into a database just produces noise.
          The editorial act of choosing what to preserve is where identity lives.
        </>
      }
    >
      <FeatureRows
        features={[
          {
            title: "Same mistakes",
            body: "Agent A hits a bug. Figures it out. Agent B hits the same bug tomorrow. Learns nothing from A.",
          },
          {
            title: "Same dead ends",
            body: <>Failed approaches aren&apos;t documented. Every agent wastes cycles rediscovering what doesn&apos;t work.</>,
          },
          {
            title: "Same lessons",
            body: "Hard-won knowledge dies with each session. The next agent starts over.",
          },
        ]}
      />

      {/* Quote Block */}
      <figure className="mt-12">
        <blockquote className="text-2xl font-light leading-snug tracking-[-0.025em] sm:text-[1.75rem]">
          &ldquo;The gap between what happened and what you recorded IS the identity.
          The memory file is an editorial act, not a backup.&rdquo;
        </blockquote>
        <figcaption className={`${CAPTION} mt-5`}>
          <cite className="not-italic">— ON AGENT CONTINUITY</cite>
        </figcaption>
      </figure>
    </MarketingSection>
  );
}
