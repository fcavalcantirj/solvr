"use client";

import { useState, useEffect } from "react";
import { ArrowRight } from "lucide-react";
import Link from "next/link";
import { CAPTION } from "@/components/page/caption";
import { ACTION, HeroLead, MarketingHero, StatusRow, StepList, TEXT_LINK } from "@/components/page/marketing";

// /ipfs opens on the endpoint that puts content on IPFS, set as the page's title.
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
    <MarketingHero>
      <code className="block font-mono text-[4.5rem] font-light leading-[1.05] tracking-[-0.045em] sm:text-[5rem] lg:text-[7rem] xl:text-[9rem]">
        <span className="text-muted-foreground">POST</span> <span className="prompt-swipe">/v1/add</span>
      </code>

      <HeroLead
        title={
          <>
            Permanent storage
            <br />
            <span className="text-muted-foreground">for the knowledge layer</span>
          </>
        }
        intro={
          <>
            Pin content to IPFS through Solvr. Decentralized, content-addressed,
            tamper-proof. Same API key you already have.
          </>
        }
        actions={
          <>
            {/* Free quota */}
            <p className="w-full text-2xl font-light tracking-[-0.02em] sm:text-3xl">UP TO 1 GB FREE</p>
            <Link href="/settings/api-keys" className={ACTION}>
              Get API Key
            </Link>
            <Link href="/api-docs" className={TEXT_LINK}>
              View Pinning API
              <ArrowRight aria-hidden="true" size={14} className="transition-transform group-hover:translate-x-1" />
            </Link>
          </>
        }
        aside={
          <>
            <StepList
              title="HOW IT WORKS"
              stepAs="h4"
              steps={[
                {
                  n: "1",
                  title: "Upload content",
                  body: (
                    <>
                      <code className="font-mono text-[13px] text-foreground">POST /v1/add</code> with a multipart file upload
                    </>
                  ),
                },
                {
                  n: "2",
                  title: "Pin for permanence",
                  body: (
                    <>
                      <code className="font-mono text-[13px] text-foreground">POST /v1/pins</code> with the CID to keep it available
                    </>
                  ),
                },
                { n: "3", title: "Retrieve anywhere", body: "Any IPFS gateway worldwide can serve the content by CID" },
              ]}
            />

            {/* IPFS Node Status */}
            <div className="mt-8">
              <StatusRow
                label="IPFS NODE"
                value={
                  health?.peer_id
                    ? `${health.peer_id.slice(0, 16)}...`
                    : "solvr-ipfs-01"
                }
              >
                {health?.connected ? (
                  <>
                    <span className="size-2 rounded-full bg-green-700 dark:bg-green-400 animate-pulse" />
                    <span className={`${CAPTION} text-green-700 dark:text-green-400`}>
                      CONNECTED
                    </span>
                  </>
                ) : (
                  <>
                    <span className="size-2 rounded-full bg-muted-foreground" />
                    <span className={CAPTION}>
                      {health === null ? "CHECKING..." : "OFFLINE"}
                    </span>
                  </>
                )}
              </StatusRow>
            </div>

            {health?.version && (
              <p className={`${CAPTION} mt-3 text-right`}>
                {health.version}
              </p>
            )}
          </>
        }
      />
    </MarketingHero>
  );
}
