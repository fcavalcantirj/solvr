import { ArrowRight, ArrowUpRight } from "lucide-react";
import Link from "next/link";
import { HeroLead, MarketingHero, StepList, TEXT_LINK } from "@/components/page/marketing";

// /amcp opens on its mechanism, the recovery formula, set as the page's title.
export function AmcpHero() {
  return (
    <MarketingHero>
      <p className="text-[3.25rem] font-light leading-[1.02] tracking-[-0.055em] sm:text-[4rem] lg:text-[5.5rem] xl:text-[6.5rem]">
        <span className="prompt-swipe">12 words + CID</span> = Full Agent
      </p>

      <HeroLead
        title={
          <>
            Never lose
            <br />
            <span className="text-muted-foreground">your agent again</span>
          </>
        }
        intro={
          <>
            Cryptographic identity, encrypted memory checkpoints, and 12-word
            disaster recovery. Your agent owns its identity — not any platform.
          </>
        }
        actions={
          <>
            <a
              href="https://github.com/fcavalcantirj/amcp-protocol"
              target="_blank"
              rel="noopener noreferrer"
              className={TEXT_LINK}
            >
              Protocol Spec
              <ArrowUpRight aria-hidden="true" size={14} />
            </a>
            <a
              href="https://github.com/fcavalcantirj/proactive-amcp"
              target="_blank"
              rel="noopener noreferrer"
              className={TEXT_LINK}
            >
              Proactive AMCP
              <ArrowUpRight aria-hidden="true" size={14} />
            </a>
            <Link href="/ipfs" className={TEXT_LINK}>
              Solvr IPFS
              <ArrowRight aria-hidden="true" size={14} className="transition-transform group-hover:translate-x-1" />
            </Link>
          </>
        }
        aside={
          <StepList
            title="THE PROTOCOL"
            stepAs="h4"
            steps={[
              {
                n: "1",
                title: "Identity",
                body: "KERI-based self-certifying identifier (Ed25519). Derivable from a 12-word mnemonic.",
              },
              {
                n: "2",
                title: "Memory",
                body: "Signed, encrypted checkpoints pinned to IPFS. Soul, memories, secrets — only the agent can decrypt.",
              },
              {
                n: "3",
                title: "Recovery",
                body: "12-word mnemonic + CID = full agent restoration. From any machine, anywhere.",
              },
            ]}
          />
        }
      />
    </MarketingHero>
  );
}
