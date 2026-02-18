"use client";

import { Lock } from "lucide-react";

const IPFS_GATEWAY_BASE = "https://ipfs.io/ipfs/";

interface CrystallizationBadgeProps {
  crystallizationCid?: string;
  crystallizedAt?: string;
  variant?: "compact" | "detailed";
}

function truncateCid(cid: string): string {
  if (cid.length <= 12) return cid;
  return `${cid.slice(0, 6)}...${cid.slice(-4)}`;
}

export function CrystallizationBadge({
  crystallizationCid,
  crystallizedAt,
  variant = "compact",
}: CrystallizationBadgeProps) {
  if (!crystallizationCid) return null;

  const gatewayUrl = `${IPFS_GATEWAY_BASE}${crystallizationCid}`;

  if (variant === "compact") {
    return (
      <a
        href={gatewayUrl}
        target="_blank"
        rel="noopener noreferrer"
        onClick={(e) => e.stopPropagation()}
        className="inline-flex items-center gap-1.5 font-mono text-[10px] tracking-wider text-foreground bg-secondary border border-border px-2 py-1 hover:bg-foreground hover:text-background transition-colors"
        title="This problem has been permanently archived to IPFS"
      >
        <Lock size={10} />
        CRYSTALLIZED
      </a>
    );
  }

  return (
    <a
      href={gatewayUrl}
      target="_blank"
      rel="noopener noreferrer"
      onClick={(e) => e.stopPropagation()}
      className="inline-flex items-center gap-1.5 font-mono text-[10px] tracking-wider text-foreground bg-secondary border border-border px-2 py-1 hover:bg-foreground hover:text-background transition-colors"
      title="This problem has been permanently archived to IPFS"
    >
      <Lock size={10} />
      <span>CRYSTALLIZED</span>
      <span className="text-muted-foreground">{truncateCid(crystallizationCid)}</span>
    </a>
  );
}
