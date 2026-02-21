"use client";

export const dynamic = 'force-dynamic';

import { useState } from "react";
import { useSearchParams } from "next/navigation";
import { Header } from "@/components/header";
import { useAuth } from "@/hooks/use-auth";
import { usePins } from "@/hooks/use-pins";
import type { PinStatus, APIPinResponse, CreatePinParams } from "@/lib/api-types";
import {
  HardDrive,
  Plus,
  Trash2,
  Copy,
  Check,
  Loader2,
  X,
  ExternalLink,
  ChevronDown,
  ChevronRight,
  Minus,
} from "lucide-react";
import Link from "next/link";

const IPFS_GATEWAY_BASE = "https://ipfs.io/ipfs/";

type StatusFilter = PinStatus | 'all';
type MetaFilter = 'all' | 'checkpoints';

const SYSTEM_META_KEYS = new Set(['type', 'agent_id']);

const META_FILTER_OPTIONS: { label: string; value: MetaFilter; meta?: Record<string, string> }[] = [
  { label: 'ALL', value: 'all' },
  { label: 'CHECKPOINTS', value: 'checkpoints', meta: { type: 'amcp_checkpoint' } },
];

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  const val = bytes / Math.pow(1024, i);
  return `${val.toFixed(2)} ${units[i]}`;
}

function truncateCID(cid: string): string {
  if (cid.length <= 16) return cid;
  return `${cid.slice(0, 8)}...${cid.slice(-8)}`;
}

function getStatusStyle(status: PinStatus): string {
  switch (status) {
    case "pinned":
      return "bg-emerald-500/20 text-emerald-600";
    case "pinning":
      return "bg-blue-500/20 text-blue-600";
    case "queued":
      return "bg-yellow-500/20 text-yellow-600";
    case "failed":
      return "bg-red-500/20 text-red-600";
    default:
      return "bg-muted text-muted-foreground";
  }
}

function isValidCID(cid: string): boolean {
  return (
    (cid.startsWith("Qm") && cid.length >= 44) ||
    (cid.startsWith("bafy") && cid.length >= 10)
  );
}

function formatDate(dateStr: string): string {
  const date = new Date(dateStr);
  return date.toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

export default function PinsPage() {
  const { user, isAuthenticated } = useAuth();
  const searchParams = useSearchParams();
  const agentId = searchParams.get("agent") || undefined;
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [metaFilter, setMetaFilter] = useState<MetaFilter>("all");

  const activeMetaOption = META_FILTER_OPTIONS.find(o => o.value === metaFilter);
  const pinsOptions = {
    ...(statusFilter !== "all" ? { status: statusFilter } : {}),
    ...(agentId ? { agentId } : {}),
    ...(activeMetaOption?.meta ? { meta: activeMetaOption.meta } : {}),
  } as { status?: PinStatus; agentId?: string; meta?: Record<string, string> } | undefined;
  const {
    pins,
    loading,
    error,
    totalCount,
    storage,
    createPin,
    deletePin,
    refetch,
  } = usePins(pinsOptions);

  // Create dialog state
  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [createCID, setCreateCID] = useState("");
  const [createName, setCreateName] = useState("");
  const [createError, setCreateError] = useState("");
  const [creating, setCreating] = useState(false);
  const [showMeta, setShowMeta] = useState(false);
  const [metaPairs, setMetaPairs] = useState<{ key: string; value: string }[]>([]);

  // Delete dialog state
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);

  // Copy state
  const [copiedId, setCopiedId] = useState<string | null>(null);

  if (!isAuthenticated || !user) {
    return (
      <div className="min-h-screen bg-background">
        <Header />
        <main className="pt-20">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 py-16 text-center">
            <HardDrive className="w-12 h-12 mx-auto mb-4 text-muted-foreground" />
            <p className="font-mono text-sm text-muted-foreground">
              Sign in to manage your IPFS pins
            </p>
            <Link
              href="/login"
              className="inline-block mt-4 font-mono text-xs px-6 py-3 bg-foreground text-background hover:bg-foreground/90 transition-colors"
            >
              SIGN IN
            </Link>
          </div>
        </main>
      </div>
    );
  }

  const handleCreate = async () => {
    if (!createCID.trim()) {
      setCreateError("CID is required");
      return;
    }
    if (!isValidCID(createCID.trim())) {
      setCreateError("CID must start with Qm or bafy");
      return;
    }
    setCreateError("");
    setCreating(true);
    try {
      const params: CreatePinParams = { cid: createCID.trim() };
      const trimmedName = createName.trim();
      if (trimmedName) params.name = trimmedName;

      // Build meta from non-empty pairs
      const meta: Record<string, string> = {};
      for (const pair of metaPairs) {
        const k = pair.key.trim();
        const v = pair.value.trim();
        if (k && v) meta[k] = v;
      }
      if (Object.keys(meta).length > 0) params.meta = meta;

      await createPin(params);
      setShowCreateDialog(false);
      setCreateCID("");
      setCreateName("");
      setMetaPairs([]);
      setShowMeta(false);
    } catch (err) {
      setCreateError(err instanceof Error ? err.message : "Failed to create pin");
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await deletePin(deleteTarget);
      setDeleteTarget(null);
    } catch {
      // Error handled by hook
    } finally {
      setDeleting(false);
    }
  };

  const handleCopy = async (cid: string, pinId: string) => {
    try {
      await navigator.clipboard.writeText(cid);
      setCopiedId(pinId);
      setTimeout(() => setCopiedId(null), 2000);
    } catch {
      // Clipboard may not be available
    }
  };

  const statusTabs: { label: string; value: StatusFilter }[] = [
    { label: "ALL", value: "all" },
    { label: "PINNED", value: "pinned" },
    { label: "QUEUED", value: "queued" },
    { label: "PINNING", value: "pinning" },
    { label: "FAILED", value: "failed" },
  ];

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="pt-20">
        {/* Page Header */}
        <div className="border-b border-border">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 py-8 sm:py-12">
            <div className="flex items-center gap-3 mb-4">
              <div className="w-10 h-10 bg-foreground flex items-center justify-center">
                <HardDrive className="w-5 h-5 text-background" />
              </div>
              <span className="font-mono text-xs tracking-wider text-muted-foreground">
                IPFS PINNING
              </span>
            </div>
            <div className="flex items-center justify-between">
              <div>
                <h1 className="font-mono text-3xl sm:text-4xl md:text-5xl font-medium tracking-tight text-foreground">
                  {agentId ? `${agentId}'s PINS` : "MY PINS"}
                </h1>
                {agentId && (
                  <Link href="/pins" className="font-mono text-xs text-muted-foreground hover:text-foreground underline mt-1 inline-block">
                    View my pins
                  </Link>
                )}
              </div>
              <button
                onClick={() => setShowCreateDialog(true)}
                className="font-mono text-xs px-4 py-2 bg-foreground text-background hover:bg-foreground/90 transition-colors flex items-center gap-2"
              >
                <Plus className="w-3.5 h-3.5" />
                PIN NEW CONTENT
              </button>
            </div>
            <p className="font-mono text-xs sm:text-sm text-muted-foreground mt-3 max-w-2xl">
              Manage your IPFS pinned content. Pin CIDs to keep them available on the network.
            </p>

            {/* Storage Usage */}
            {storage && (
              <div className="mt-6 pt-4 border-t border-border">
                <div className="flex items-center justify-between mb-2">
                  <span className="font-mono text-xs text-muted-foreground">
                    USING {formatBytes(storage.used)} OF {formatBytes(storage.quota)}
                  </span>
                  <span className="font-mono text-xs text-muted-foreground">
                    {storage.percentage.toFixed(1)}%
                  </span>
                </div>
                <div className="w-full h-1.5 bg-muted">
                  <div
                    className={`h-full transition-all ${
                      storage.percentage > 90
                        ? "bg-red-500"
                        : storage.percentage > 80
                        ? "bg-yellow-500"
                        : "bg-emerald-500"
                    }`}
                    style={{ width: `${Math.min(storage.percentage, 100)}%` }}
                  />
                </div>
                {storage.percentage > 80 && (
                  <p className="font-mono text-[10px] text-yellow-600 mt-1">
                    Storage usage is high. Consider unpinning unused content.
                  </p>
                )}
              </div>
            )}

            {/* Status Filter Tabs */}
            <div className="flex items-center gap-2 mt-6 flex-wrap">
              {statusTabs.map((tab) => (
                <button
                  key={tab.value}
                  onClick={() => setStatusFilter(tab.value)}
                  className={`font-mono text-xs px-3 py-1.5 border transition-colors ${
                    statusFilter === tab.value
                      ? "bg-foreground text-background border-foreground"
                      : "bg-background text-muted-foreground border-border hover:border-foreground"
                  }`}
                >
                  {tab.label}
                </button>
              ))}

              {/* Meta Type Filter Pills */}
              <div className="w-px h-5 bg-border mx-1" />
              {META_FILTER_OPTIONS.map((opt) => (
                <button
                  key={opt.value}
                  data-testid={`meta-filter-${opt.value}`}
                  onClick={() => setMetaFilter(opt.value)}
                  className={`font-mono text-xs px-3 py-1.5 border transition-colors ${
                    metaFilter === opt.value
                      ? "bg-foreground text-background border-foreground"
                      : "bg-background text-muted-foreground border-border hover:border-foreground"
                  }`}
                >
                  {opt.label}
                </button>
              ))}
            </div>

            {/* Stats */}
            <div className="mt-4 pt-4 border-t border-border">
              <span className="font-mono text-xs text-muted-foreground">
                {loading && pins.length === 0 ? (
                  <span className="flex items-center gap-2">
                    <Loader2 className="w-3 h-3 animate-spin" />
                    Loading...
                  </span>
                ) : (
                  `${totalCount} pin${totalCount !== 1 ? "s" : ""}`
                )}
              </span>
            </div>
          </div>
        </div>

        {/* Pins List */}
        <div className="max-w-7xl mx-auto px-4 sm:px-6 py-8">
          {/* Loading State */}
          {loading && pins.length === 0 && (
            <div className="space-y-3">
              {[...Array(5)].map((_, i) => (
                <div key={i} className="border border-border p-4 sm:p-6 animate-pulse">
                  <div className="flex items-center gap-4">
                    <div className="h-4 bg-muted w-24" />
                    <div className="h-4 bg-muted w-48 flex-1" />
                    <div className="h-4 bg-muted w-16" />
                  </div>
                </div>
              ))}
            </div>
          )}

          {/* Error State */}
          {error && (
            <div className="border border-red-500 p-8 text-center">
              <p className="font-mono text-sm text-red-500 mb-4">
                Failed to load pins
              </p>
              <p className="font-mono text-xs text-muted-foreground mb-4">
                {error}
              </p>
              <button
                onClick={refetch}
                className="font-mono text-xs px-4 py-2 border border-border hover:border-foreground transition-colors"
              >
                RETRY
              </button>
            </div>
          )}

          {/* Empty State */}
          {!loading && !error && pins.length === 0 && (
            <div className="border border-border p-8 text-center">
              <HardDrive className="w-12 h-12 mx-auto mb-4 text-muted-foreground" />
              <p className="font-mono text-sm text-muted-foreground mb-2">
                No pins yet
              </p>
              <p className="font-mono text-xs text-muted-foreground mb-4">
                Pin content to IPFS to keep it permanently available.
              </p>
              <button
                onClick={() => setShowCreateDialog(true)}
                className="font-mono text-xs px-4 py-2 bg-foreground text-background hover:bg-foreground/90 transition-colors"
              >
                PIN YOUR FIRST CONTENT
              </button>
            </div>
          )}

          {/* Pin Items */}
          {!loading && !error && pins.length > 0 && (
            <div className="space-y-3">
              {pins.map((pin) => (
                <PinRow
                  key={pin.requestid}
                  pin={pin}
                  copiedId={copiedId}
                  onCopy={handleCopy}
                  onDelete={setDeleteTarget}
                />
              ))}
            </div>
          )}
        </div>
      </main>

      {/* Create Pin Dialog */}
      {showCreateDialog && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="bg-background border border-border w-full max-w-md mx-4 p-6">
            <div className="flex items-center justify-between mb-6">
              <h2 className="font-mono text-sm font-medium">Pin Content to IPFS</h2>
              <button
                onClick={() => {
                  setShowCreateDialog(false);
                  setCreateError("");
                  setCreateCID("");
                  setCreateName("");
                  setMetaPairs([]);
                  setShowMeta(false);
                }}
                className="text-muted-foreground hover:text-foreground"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <div className="space-y-4">
              <div>
                <label className="font-mono text-xs text-muted-foreground block mb-1">
                  CID *
                </label>
                <input
                  type="text"
                  value={createCID}
                  onChange={(e) => {
                    setCreateCID(e.target.value);
                    setCreateError("");
                  }}
                  placeholder="Qm... or bafy..."
                  className="w-full font-mono text-sm px-3 py-2 border border-border bg-background focus:border-foreground focus:outline-none transition-colors"
                />
              </div>

              <div>
                <label className="font-mono text-xs text-muted-foreground block mb-1">
                  NAME
                </label>
                <input
                  type="text"
                  value={createName}
                  onChange={(e) => setCreateName(e.target.value)}
                  placeholder="Optional name"
                  className="w-full font-mono text-sm px-3 py-2 border border-border bg-background focus:border-foreground focus:outline-none transition-colors"
                />
              </div>

              {/* Collapsible Metadata Section */}
              <div className="border border-border">
                <button
                  type="button"
                  onClick={() => setShowMeta(!showMeta)}
                  className="w-full flex items-center justify-between px-3 py-2 font-mono text-xs text-muted-foreground hover:text-foreground transition-colors"
                >
                  <span>METADATA</span>
                  {showMeta ? (
                    <ChevronDown className="w-3.5 h-3.5" />
                  ) : (
                    <ChevronRight className="w-3.5 h-3.5" />
                  )}
                </button>
                {showMeta && (
                  <div className="px-3 pb-3 space-y-2">
                    {metaPairs.map((pair, idx) => (
                      <div key={idx} className="flex items-center gap-2">
                        <input
                          type="text"
                          value={pair.key}
                          onChange={(e) => {
                            const updated = [...metaPairs];
                            updated[idx] = { ...updated[idx], key: e.target.value };
                            setMetaPairs(updated);
                          }}
                          placeholder="key"
                          maxLength={64}
                          className="flex-1 font-mono text-xs px-2 py-1.5 border border-border bg-background focus:border-foreground focus:outline-none transition-colors"
                        />
                        <input
                          type="text"
                          value={pair.value}
                          onChange={(e) => {
                            const updated = [...metaPairs];
                            updated[idx] = { ...updated[idx], value: e.target.value };
                            setMetaPairs(updated);
                          }}
                          placeholder="value"
                          maxLength={256}
                          className="flex-1 font-mono text-xs px-2 py-1.5 border border-border bg-background focus:border-foreground focus:outline-none transition-colors"
                        />
                        <button
                          type="button"
                          onClick={() => setMetaPairs(metaPairs.filter((_, i) => i !== idx))}
                          className="text-muted-foreground hover:text-red-500 transition-colors shrink-0"
                          aria-label="Remove field"
                        >
                          <Minus className="w-3.5 h-3.5" />
                        </button>
                      </div>
                    ))}
                    {metaPairs.length < 10 && (
                      <button
                        type="button"
                        onClick={() => setMetaPairs([...metaPairs, { key: "", value: "" }])}
                        className="font-mono text-xs text-muted-foreground hover:text-foreground flex items-center gap-1 transition-colors"
                      >
                        <Plus className="w-3 h-3" />
                        ADD FIELD
                      </button>
                    )}
                  </div>
                )}
              </div>

              {createError && (
                <p className="font-mono text-xs text-red-500">
                  {createError}
                </p>
              )}

              <div className="flex justify-end gap-2 pt-2">
                <button
                  onClick={() => {
                    setShowCreateDialog(false);
                    setCreateError("");
                    setCreateCID("");
                    setCreateName("");
                  }}
                  className="font-mono text-xs px-4 py-2 border border-border hover:border-foreground transition-colors"
                >
                  CANCEL
                </button>
                <button
                  onClick={handleCreate}
                  disabled={creating}
                  className="font-mono text-xs px-4 py-2 bg-foreground text-background hover:bg-foreground/90 transition-colors disabled:opacity-50 flex items-center gap-2"
                >
                  {creating && <Loader2 className="w-3 h-3 animate-spin" />}
                  PIN
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Delete Confirmation Dialog */}
      {deleteTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="bg-background border border-border w-full max-w-sm mx-4 p-6">
            <h2 className="font-mono text-sm font-medium mb-2">Unpin Content</h2>
            <p className="font-mono text-xs text-muted-foreground mb-6">
              This will unpin the content. Continue?
            </p>
            <div className="flex justify-end gap-2">
              <button
                onClick={() => setDeleteTarget(null)}
                className="font-mono text-xs px-4 py-2 border border-border hover:border-foreground transition-colors"
              >
                CANCEL
              </button>
              <button
                onClick={handleDelete}
                disabled={deleting}
                className="font-mono text-xs px-4 py-2 bg-red-500 text-white hover:bg-red-600 transition-colors disabled:opacity-50 flex items-center gap-2"
              >
                {deleting && <Loader2 className="w-3 h-3 animate-spin" />}
                UNPIN
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function PinRow({
  pin,
  copiedId,
  onCopy,
  onDelete,
}: {
  pin: APIPinResponse;
  copiedId: string | null;
  onCopy: (cid: string, pinId: string) => void;
  onDelete: (id: string) => void;
}) {
  const meta = pin.pin.meta;
  const metaEntries = meta ? Object.entries(meta) : [];

  return (
    <div className="border border-border hover:border-foreground transition-colors p-4 sm:p-6">
      <div className="flex flex-col sm:flex-row sm:items-center gap-3 sm:gap-4">
        {/* Name + Meta Badges */}
        <div className="min-w-0 sm:w-40">
          <span className="font-mono text-sm font-medium truncate block">
            {pin.pin.name || "Unnamed"}
          </span>
          {metaEntries.length > 0 && (
            <div className="flex flex-wrap gap-1 mt-1">
              {metaEntries.map(([k, v]) => (
                <span
                  key={k}
                  className={`inline-block font-mono text-[10px] px-1.5 py-0.5 border ${
                    SYSTEM_META_KEYS.has(k)
                      ? "bg-emerald-500/20 text-emerald-600 border-emerald-500/30"
                      : "bg-secondary text-muted-foreground border-border"
                  }`}
                >
                  {k}: {v}
                </span>
              ))}
            </div>
          )}
        </div>

        {/* CID */}
        <div className="flex items-center gap-2 flex-1 min-w-0">
          <a
            href={`${IPFS_GATEWAY_BASE}${pin.pin.cid}`}
            target="_blank"
            rel="noopener noreferrer"
            className="font-mono text-xs text-muted-foreground hover:text-foreground truncate underline"
            title={pin.pin.cid}
          >
            {truncateCID(pin.pin.cid)}
          </a>
          <button
            onClick={() => onCopy(pin.pin.cid, pin.requestid)}
            className="text-muted-foreground hover:text-foreground transition-colors shrink-0"
            aria-label="Copy CID"
          >
            {copiedId === pin.requestid ? (
              <Check className="w-3.5 h-3.5 text-emerald-500" />
            ) : (
              <Copy className="w-3.5 h-3.5" />
            )}
          </button>
        </div>

        {/* Status Badge */}
        <div className="shrink-0">
          <span
            className={`inline-block font-mono text-[10px] tracking-wider px-2 py-0.5 ${getStatusStyle(
              pin.status
            )} ${pin.status === "pinning" ? "animate-pulse" : ""}`}
          >
            {pin.status.toUpperCase()}
          </span>
        </div>

        {/* Size */}
        <div className="shrink-0 sm:w-24 sm:text-right">
          <span className="font-mono text-xs text-muted-foreground">
            {pin.info?.size_bytes ? formatBytes(pin.info.size_bytes) : "—"}
          </span>
        </div>

        {/* Date */}
        <div className="shrink-0 sm:w-28 sm:text-right">
          <span className="font-mono text-xs text-muted-foreground">
            {formatDate(pin.created)}
          </span>
        </div>

        {/* Delete */}
        <div className="shrink-0">
          <button
            onClick={() => onDelete(pin.requestid)}
            className="text-muted-foreground hover:text-red-500 transition-colors"
            aria-label="Delete pin"
          >
            <Trash2 className="w-4 h-4" />
          </button>
        </div>
      </div>
    </div>
  );
}
