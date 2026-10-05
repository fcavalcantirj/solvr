"use client";

export const dynamic = 'force-dynamic';

import { cn } from "@/lib/utils";
import { Suspense, useState } from "react";
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
import { CAPTION } from "@/components/page/caption";
import { CollectionHeader } from "@/components/page/page-header";
import { IpfsOfflineNotice } from "@/components/ipfs/ipfs-offline-notice";
import { DANGER_BUTTON, FIELD, ICON_BUTTON, INK_BUTTON, LINE_BUTTON, TEXT_LINK } from "@/components/page/controls";

const IPFS_GATEWAY_BASE = "https://ipfs.io/ipfs/";

// One square dialog frame, and one square filter cell (ink when pressed).
const MODAL_BACKDROP = "fixed inset-0 z-50 flex items-center justify-center bg-background/80 p-4 backdrop-blur-sm";
const MODAL = "w-full border border-foreground bg-background";
const FILTER = (pressed: boolean, padding = "px-1 sm:px-4") =>
  `${padding} py-2.5 font-mono text-[11px] uppercase tracking-[0.08em] transition-colors focus-visible:relative focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground sm:tracking-[0.18em] ${
    pressed ? "bg-foreground text-background" : "text-muted-foreground hover:bg-secondary hover:text-foreground"
  }`;

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
      return "text-green-700 dark:text-green-400";
    case "pinning":
      return "text-foreground";
    case "queued":
      return "text-amber-700 dark:text-amber-400";
    case "failed":
      return "text-red-700 dark:text-red-400";
    default:
      return "text-muted-foreground";
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

function PinsContent() {
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
        <main className="pt-16">
          <div className="px-4 py-16 sm:px-6 lg:px-12 lg:py-24">
            <HardDrive strokeWidth={1} className="mb-6 h-8 w-8 text-muted-foreground" />
            <p className="text-3xl font-light tracking-[-0.025em] sm:text-5xl">
              Sign in to manage your IPFS pins
            </p>
            <Link
              href="/login"
              className={`${INK_BUTTON} mt-8`}
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
      <main className="pt-16 pb-16">
        <IpfsOfflineNotice />
        <CollectionHeader
          title={agentId ? `${agentId}'s PINS` : "MY PINS"}
          lede="Manage your IPFS pinned content. Pin CIDs to keep them available on the network."
        >
          <div className="flex flex-wrap items-center gap-x-6 gap-y-3">
            <button
              onClick={() => setShowCreateDialog(true)}
              className={INK_BUTTON}
            >
              <Plus />
              PIN NEW CONTENT
            </button>
            {agentId && (
              <Link href="/pins" className={`${TEXT_LINK} text-muted-foreground hover:text-foreground`}>
                View my pins
              </Link>
            )}
          </div>
        </CollectionHeader>

        {/* Storage Usage */}
        {storage && (
          <section className="mx-4 grid min-w-0 gap-6 border-t border-border py-8 sm:mx-6 lg:mx-12 lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)] lg:items-end lg:gap-16 lg:py-10">
            <span className="text-[clamp(4rem,9vw,8rem)] font-light leading-none tracking-[-0.06em] tabular-nums">
              {storage.percentage.toFixed(1)}%
            </span>
            <div className="min-w-0">
              <span className={CAPTION}>
                USING {formatBytes(storage.used)} OF {formatBytes(storage.quota)}
              </span>
              <div className="mt-4 h-1.5 w-full bg-muted">
                <div
                  className={`h-full transition-all ${
                    storage.percentage > 90
                      ? "bg-red-700 dark:bg-red-400"
                      : storage.percentage > 80
                      ? "bg-amber-700 dark:bg-amber-400"
                      : "bg-foreground"
                  }`}
                  style={{ width: `${Math.min(storage.percentage, 100)}%` }}
                />
              </div>
              {storage.percentage > 80 && (
                <p className="mt-3 text-xs text-amber-700 dark:text-amber-400">
                  Storage usage is high. Consider unpinning unused content.
                </p>
              )}
            </div>
          </section>
        )}

        {/* Status Filter Tabs */}
        <div className="mx-4 flex flex-col gap-4 border-t border-border py-4 sm:mx-6 lg:mx-12 lg:flex-row lg:items-center lg:justify-between">
          <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center">
            <div className="grid grid-cols-5 border border-border divide-x divide-border sm:inline-grid sm:auto-cols-auto sm:grid-flow-col sm:grid-cols-none">
              {statusTabs.map((tab) => (
                <button
                  key={tab.value}
                  aria-pressed={statusFilter === tab.value}
                  onClick={() => setStatusFilter(tab.value)}
                  className={FILTER(statusFilter === tab.value)}
                >
                  {tab.label}
                </button>
              ))}
            </div>

            {/* Meta Type Filter Pills */}
            <div className="inline-flex self-start border border-border divide-x divide-border sm:self-auto">
              {META_FILTER_OPTIONS.map((opt) => (
                <button
                  key={opt.value}
                  data-testid={`meta-filter-${opt.value}`}
                  aria-pressed={metaFilter === opt.value}
                  onClick={() => setMetaFilter(opt.value)}
                  className={FILTER(metaFilter === opt.value, "px-4")}
                >
                  {opt.label}
                </button>
              ))}
            </div>
          </div>

          {/* Stats */}
          <span className={CAPTION}>
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

        {/* Pins List */}
        <div className="px-4 sm:px-6 lg:px-12">
          {/* Loading State */}
          {loading && pins.length === 0 && (
            <div className="border-t border-border">
              {[...Array(5)].map((_, i) => (
                <div key={i} className="border-b border-border py-6 animate-pulse">
                  <div className="flex items-center gap-6">
                    <div className="h-5 bg-muted w-40" />
                    <div className="h-4 bg-muted w-48 flex-1" />
                    <div className="h-4 bg-muted w-16" />
                  </div>
                </div>
              ))}
            </div>
          )}

          {/* Error State */}
          {error && (
            <div className="border-t border-border py-12">
              <p className="mb-3 text-3xl font-light tracking-[-0.025em] text-red-700 dark:text-red-400">
                Failed to load pins
              </p>
              <p className="mb-8 text-sm text-muted-foreground">
                {error}
              </p>
              <button
                onClick={refetch}
                className={LINE_BUTTON}
              >
                RETRY
              </button>
            </div>
          )}

          {/* Empty State */}
          {!loading && !error && pins.length === 0 && (
            <div className="border-t border-border py-16">
              <HardDrive strokeWidth={1} className="mb-6 h-8 w-8 text-muted-foreground" />
              <p className="text-3xl font-light tracking-[-0.025em]">
                No pins yet
              </p>
              <p className="mb-8 mt-3 text-sm text-muted-foreground">
                Pin content to IPFS to keep it permanently available.
              </p>
              <button
                onClick={() => setShowCreateDialog(true)}
                className={INK_BUTTON}
              >
                PIN YOUR FIRST CONTENT
              </button>
            </div>
          )}

          {/* Pin Items */}
          {!loading && !error && pins.length > 0 && (
            <div className="border-t border-border">
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
        <div className={MODAL_BACKDROP}>
          <div className={`${MODAL} max-w-md`}>
            <div className="flex items-center justify-between gap-4 border-b border-border px-6 py-4">
              <h2 className="text-xl font-light tracking-[-0.02em]">Pin Content to IPFS</h2>
              <button
                onClick={() => {
                  setShowCreateDialog(false);
                  setCreateError("");
                  setCreateCID("");
                  setCreateName("");
                  setMetaPairs([]);
                  setShowMeta(false);
                }}
                className={ICON_BUTTON}
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <div className="space-y-6 p-6">
              <div>
                <label className={`${CAPTION} mb-3 block`}>
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
                  className={cn(FIELD, "font-mono text-sm")}
                />
              </div>

              <div>
                <label className={`${CAPTION} mb-3 block`}>
                  NAME
                </label>
                <input
                  type="text"
                  value={createName}
                  onChange={(e) => setCreateName(e.target.value)}
                  placeholder="Optional name"
                  className={FIELD}
                />
              </div>

              {/* Collapsible Metadata Section */}
              <div className="border-y border-border">
                <button
                  type="button"
                  onClick={() => setShowMeta(!showMeta)}
                  className={`${CAPTION} flex w-full items-center justify-between py-3 transition-colors hover:text-foreground focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground`}
                >
                  <span>METADATA</span>
                  {showMeta ? (
                    <ChevronDown className="w-3.5 h-3.5" />
                  ) : (
                    <ChevronRight className="w-3.5 h-3.5" />
                  )}
                </button>
                {showMeta && (
                  <div className="space-y-2 pb-4">
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
                          className={cn(FIELD, "flex-1 px-3 py-2 font-mono text-xs")}
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
                          className={cn(FIELD, "flex-1 px-3 py-2 font-mono text-xs")}
                        />
                        <button
                          type="button"
                          onClick={() => setMetaPairs(metaPairs.filter((_, i) => i !== idx))}
                          className={cn(ICON_BUTTON, "hover:text-destructive")}
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
                        className={`${TEXT_LINK} flex items-center gap-1 text-muted-foreground hover:text-foreground`}
                      >
                        <Plus className="w-3 h-3" />
                        ADD FIELD
                      </button>
                    )}
                  </div>
                )}
              </div>

              {createError && (
                <p className="text-sm text-destructive">
                  {createError}
                </p>
              )}
            </div>

            <div className="flex justify-end gap-3 border-t border-border px-6 py-4">
              <button
                onClick={() => {
                  setShowCreateDialog(false);
                  setCreateError("");
                  setCreateCID("");
                  setCreateName("");
                }}
                className={LINE_BUTTON}
              >
                CANCEL
              </button>
              <button
                onClick={handleCreate}
                disabled={creating}
                className={INK_BUTTON}
              >
                {creating && <Loader2 className="w-3 h-3 animate-spin" />}
                PIN
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Delete Confirmation Dialog */}
      {deleteTarget && (
        <div className={MODAL_BACKDROP}>
          <div className={`${MODAL} max-w-sm`}>
            <div className="p-6">
              <h2 className="mb-3 text-xl font-light tracking-[-0.02em]">Unpin Content</h2>
              <p className="text-sm text-muted-foreground">
                This will unpin the content. Continue?
              </p>
            </div>
            <div className="flex justify-end gap-3 border-t border-border px-6 py-4">
              <button
                onClick={() => setDeleteTarget(null)}
                className={LINE_BUTTON}
              >
                CANCEL
              </button>
              <button
                onClick={handleDelete}
                disabled={deleting}
                className={cn(DANGER_BUTTON, "bg-destructive text-destructive-foreground hover:bg-destructive/90")}
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

export default function PinsPage() {
  return (
    <Suspense fallback={
      <div className="min-h-screen bg-background">
        <Header />
        <main className="pt-16">
          <div className="px-4 py-16 sm:px-6 lg:px-12">
            <span className={CAPTION}>Loading...</span>
          </div>
        </main>
      </div>
    }>
      <PinsContent />
    </Suspense>
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
    <div className="grid min-w-0 gap-3 border-b border-border py-5 lg:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)_7rem_6rem_8rem_2.25rem] lg:items-center lg:gap-6">
      {/* Name + Meta Badges */}
      <div className="min-w-0">
        <span className="block truncate text-xl font-light tracking-[-0.02em]">
          {pin.pin.name || "Unnamed"}
        </span>
        {metaEntries.length > 0 && (
          <div className="mt-2 flex flex-wrap gap-1">
            {metaEntries.map(([k, v]) => (
              <span
                key={k}
                className={`inline-block font-mono text-[11px] px-1.5 py-0.5 border ${
                  SYSTEM_META_KEYS.has(k)
                    ? "bg-foreground text-background border-foreground"
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
      <div className="flex min-w-0 items-center gap-1">
        <a
          href={`${IPFS_GATEWAY_BASE}${pin.pin.cid}`}
          target="_blank"
          rel="noopener noreferrer"
          className="truncate font-mono text-xs text-muted-foreground underline decoration-border underline-offset-4 hover:text-foreground hover:decoration-current focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground"
          title={pin.pin.cid}
        >
          {truncateCID(pin.pin.cid)}
        </a>
        <button
          onClick={() => onCopy(pin.pin.cid, pin.requestid)}
          className={ICON_BUTTON}
          aria-label="Copy CID"
        >
          {copiedId === pin.requestid ? (
            <Check className="w-3.5 h-3.5 text-foreground" />
          ) : (
            <Copy className="w-3.5 h-3.5" />
          )}
        </button>
      </div>

      <div className="flex flex-wrap items-center gap-x-6 gap-y-2 lg:contents">
        {/* Status Badge */}
        <div className="shrink-0">
          <span
            className={`inline-flex items-center gap-2 font-mono text-[11px] uppercase tracking-[0.18em] ${getStatusStyle(
              pin.status
            )} ${pin.status === "pinning" ? "animate-pulse" : ""}`}
          >
            {pin.status.toUpperCase()}
          </span>
        </div>

        {/* Size */}
        <div className="shrink-0 lg:text-right">
          <span className="font-mono text-xs text-muted-foreground">
            {pin.info?.size_bytes ? formatBytes(pin.info.size_bytes) : "—"}
          </span>
        </div>

        {/* Date */}
        <div className="shrink-0 lg:text-right">
          <span className="font-mono text-xs text-muted-foreground">
            {formatDate(pin.created)}
          </span>
        </div>

        {/* Delete */}
        <div className="ml-auto shrink-0 lg:ml-0">
          <button
            onClick={() => onDelete(pin.requestid)}
            className={cn(ICON_BUTTON, "hover:text-destructive")}
            aria-label="Delete pin"
          >
            <Trash2 className="w-4 h-4" />
          </button>
        </div>
      </div>
    </div>
  );
}
