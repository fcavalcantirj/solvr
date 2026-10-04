"use client";

import { useIPFSHealth } from "@/hooks/use-ipfs-health";
import { HardDrive } from "lucide-react";
import { CAPTION } from "@/components/page/caption";

function truncatePeerId(peerId: string): string {
  if (peerId.length <= 12) return peerId;
  return `${peerId.slice(0, 6)}...${peerId.slice(-4)}`;
}

type StatusColor = "emerald" | "red" | "yellow";

function getStatusInfo(
  loading: boolean,
  error: string | null,
  connected: boolean | undefined,
  nodeError: string | undefined
): { label: string; color: StatusColor; detail?: string } {
  if (loading) {
    return { label: "CHECKING...", color: "yellow" };
  }
  if (error) {
    return { label: "ERROR", color: "red", detail: error };
  }
  if (connected) {
    return { label: "CONNECTED", color: "emerald" };
  }
  return { label: "DISCONNECTED", color: "red", detail: nodeError };
}

interface IPFSStatusIndicatorProps {
  pollIntervalMs?: number;
}

export function IPFSStatusIndicator({
  pollIntervalMs = 30000,
}: IPFSStatusIndicatorProps) {
  const { data, loading, error } = useIPFSHealth({ pollIntervalMs });

  const status = getStatusInfo(loading, error, data?.connected, data?.error);

  const dotColorClass =
    status.color === "emerald"
      ? "bg-green-700 dark:bg-green-400"
      : status.color === "yellow"
        ? "bg-amber-700 dark:bg-amber-400"
        : "bg-red-700 dark:bg-red-400";

  return (
    <div className="min-w-0">
      {/* Header */}
      <div className="flex items-center gap-3">
        <HardDrive aria-hidden="true" className="w-4 h-4 text-muted-foreground" />
        <h3 className={CAPTION}>
          IPFS NODE
        </h3>
      </div>

      {/* The state itself, set big */}
      <div className="mt-6 flex min-w-0 items-center gap-4">
        <div
          data-testid="ipfs-status-dot"
          className={`size-3 shrink-0 rounded-full ${dotColorClass}`}
        />
        <span className="text-[clamp(2.75rem,6vw,5.5rem)] font-light leading-none tracking-[-0.05em] [overflow-wrap:anywhere]">
          {status.label}
        </span>
      </div>

      {/* Details */}
      {status.detail && (
        <div className="mt-6 border-y border-destructive py-3">
          <span className="font-mono text-xs text-destructive [overflow-wrap:anywhere]">
            {status.detail}
          </span>
        </div>
      )}

      {data?.connected && (
        <dl className="mt-10 border-t border-border">
          <div className="flex items-center justify-between gap-6 border-b border-border py-5">
            <dt className={CAPTION}>
              PEER ID
            </dt>
            <dd className="font-mono text-sm">
              {truncatePeerId(data.peer_id)}
            </dd>
          </div>
          <div className="flex items-center justify-between gap-6 border-b border-border py-5">
            <dt className={CAPTION}>
              VERSION
            </dt>
            <dd className="font-mono text-sm [overflow-wrap:anywhere]">{data.version}</dd>
          </div>
        </dl>
      )}
    </div>
  );
}
