"use client";

import { useIPFSHealth } from "@/hooks/use-ipfs-health";
import { HardDrive } from "lucide-react";

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
      ? "bg-emerald-500"
      : status.color === "yellow"
        ? "bg-yellow-500"
        : "bg-red-500";

  return (
    <div className="border border-border p-6">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 bg-foreground flex items-center justify-center">
            <HardDrive className="w-4 h-4 text-background" />
          </div>
          <h3 className="font-mono text-xs tracking-wider text-muted-foreground">
            IPFS NODE
          </h3>
        </div>
        <div className="flex items-center gap-2">
          <div
            data-testid="ipfs-status-dot"
            className={`w-2 h-2 rounded-full ${dotColorClass}`}
          />
          <span className="font-mono text-xs tracking-wider">
            {status.label}
          </span>
        </div>
      </div>

      {/* Details */}
      {status.detail && (
        <div className="mb-4 bg-destructive/10 border border-destructive px-4 py-2">
          <span className="font-mono text-xs text-destructive">
            {status.detail}
          </span>
        </div>
      )}

      {data?.connected && (
        <div className="space-y-3">
          <div className="flex items-center justify-between py-2 border-b border-border">
            <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
              PEER ID
            </span>
            <span className="font-mono text-xs">
              {truncatePeerId(data.peer_id)}
            </span>
          </div>
          <div className="flex items-center justify-between py-2 border-b border-border">
            <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
              VERSION
            </span>
            <span className="font-mono text-xs">{data.version}</span>
          </div>
        </div>
      )}
    </div>
  );
}
