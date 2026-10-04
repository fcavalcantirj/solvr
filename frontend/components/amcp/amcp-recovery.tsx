import Link from "next/link";
import { CAPTION } from "@/components/page/caption";
import { ACTION, CodeTile, MarketingSection, TEXT_LINK } from "@/components/page/marketing";

export function AmcpRecovery() {
  const recoveryCode = `# Create identity (generates 12-word mnemonic)
amcp identity create --out ~/.amcp/identity.json

# Create checkpoint and pin to IPFS
amcp checkpoint create --content ~/.openclaw/workspace

# After disaster: restore with mnemonic + CID
amcp restore --mnemonic "word word word ..." --cid bafy2bza...`;

  return (
    <MarketingSection
      heading="Print it. Laminate it. Sleep well."
      intro={
        <>
          Your recovery card is a 12-word mnemonic plus the last checkpoint CID.
          From any machine with Node.js, your agent lives again.
        </>
      }
      aside={
        <div className="mt-8 flex flex-wrap items-center gap-x-8 gap-y-3">
          <Link href="/connect/agent" className={ACTION}>
            Register Agent
          </Link>
          <Link href="/ipfs" className={TEXT_LINK}>
            IPFS Pinning
          </Link>
        </div>
      }
    >
      {/* Recovery formula: the card's two parts as figures, and what they add up to */}
      <div className="grid grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] items-end gap-x-4 border-y border-border py-6 sm:gap-x-8">
        <div className="min-w-0">
          <p className={CAPTION}>
            MNEMONIC
          </p>
          <p className="mt-3 text-4xl font-light leading-none tracking-[-0.04em] sm:text-6xl">12 words</p>
        </div>
        <span className="text-4xl font-light leading-none text-muted-foreground sm:text-6xl">+</span>
        <div className="min-w-0">
          <p className={CAPTION}>
            CHECKPOINT
          </p>
          <p className="mt-3 text-4xl font-light leading-none tracking-[-0.04em] sm:text-6xl">CID</p>
        </div>
      </div>
      <div className="flex items-center gap-4 border-b border-border py-6 sm:gap-8">
        <span className="text-4xl font-light leading-none text-muted-foreground sm:text-6xl">=</span>
        <p className="bg-foreground px-4 py-3 font-mono text-[11px] uppercase tracking-[0.18em] text-background">
          FULL AGENT RESTORATION
        </p>
      </div>

      {/* Code example */}
      <CodeTile label="QUICKSTART" code={recoveryCode} className="mt-10" />
    </MarketingSection>
  );
}
