"use client";

// Force dynamic rendering
export const dynamic = 'force-dynamic';

import { useState } from "react";
import Link from "next/link";
import { useAPIKeys } from "@/hooks/use-api-keys";
import { SettingsLayout } from "@/components/settings/settings-layout";
import { CAPTION } from "@/components/page/caption";
import { PageSection } from "@/components/page/page-section";
import { DANGER_BUTTON, FIELD, ICON_BUTTON, INK_BUTTON, LINE_BUTTON, TEXT_LINK } from "@/components/page/controls";
import { cn } from "@/lib/utils";
import {
  Key,
  Plus,
  Loader2,
  Copy,
  Check,
  RefreshCw,
  Trash2,
  AlertTriangle,
  X
} from "lucide-react";
import { formatRelativeTime } from "@/lib/api";

const UNLOCK_ROW = "grid grid-cols-[2.5rem_minmax(0,1fr)] items-baseline gap-4 border-b border-border py-5";
const UNLOCK_LINK = "text-2xl font-light tracking-[-0.025em] underline decoration-transparent decoration-1 underline-offset-[5px] transition-colors hover:decoration-current focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-4 focus-visible:outline-foreground";

// One square dialog for every key action: hairline frame, title, body, actions.
const MODAL_BACKDROP = "fixed inset-0 z-50 flex items-center justify-center bg-background/80 p-4 backdrop-blur-sm";
const MODAL = "w-full border border-foreground bg-background";
const MODAL_HEAD = "flex items-center justify-between gap-4 border-b border-border px-6 py-4";
const MODAL_TITLE = "text-xl font-light tracking-[-0.02em]";
const MODAL_FOOT = "flex flex-wrap justify-end gap-3 border-t border-border px-6 py-4";

export default function APIKeysPage() {
  const { keys, loading, error, createKey, revokeKey, regenerateKey } = useAPIKeys();

  // Modal states
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showKeyCreatedModal, setShowKeyCreatedModal] = useState(false);
  const [showRevokeModal, setShowRevokeModal] = useState<string | null>(null);
  const [showRegenerateModal, setShowRegenerateModal] = useState<string | null>(null);

  // Form states
  const [newKeyName, setNewKeyName] = useState("");
  const [createdKey, setCreatedKey] = useState<string | null>(null);
  const [createdKeyName, setCreatedKeyName] = useState("");
  const [isCreating, setIsCreating] = useState(false);
  const [isRevoking, setIsRevoking] = useState(false);
  const [isRegenerating, setIsRegenerating] = useState(false);
  const [copied, setCopied] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  const handleCreateKey = async () => {
    if (!newKeyName.trim()) return;
    setIsCreating(true);
    setActionError(null);
    try {
      const response = await createKey(newKeyName.trim());
      setCreatedKey(response.data.key);
      setCreatedKeyName(response.data.name);
      setShowCreateModal(false);
      setNewKeyName("");
      setShowKeyCreatedModal(true);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to create key");
    } finally {
      setIsCreating(false);
    }
  };

  const handleRevokeKey = async (id: string) => {
    setIsRevoking(true);
    setActionError(null);
    try {
      await revokeKey(id);
      setShowRevokeModal(null);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to revoke key");
    } finally {
      setIsRevoking(false);
    }
  };

  const handleRegenerateKey = async (id: string) => {
    setIsRegenerating(true);
    setActionError(null);
    try {
      const response = await regenerateKey(id);
      setCreatedKey(response.data.key);
      setCreatedKeyName(response.data.name);
      setShowRegenerateModal(null);
      setShowKeyCreatedModal(true);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to regenerate key");
    } finally {
      setIsRegenerating(false);
    }
  };

  const copyToClipboard = async (text: string) => {
    await navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <SettingsLayout>
      {/* API keys: the keys themselves are the ledger */}
      <PageSection
        heading="API KEYS"
        intro="Create and manage API keys for programmatic access to Solvr."
        aside={
          <button
            type="button"
            onClick={() => setShowCreateModal(true)}
            className={INK_BUTTON}
          >
            <Plus />
            CREATE KEY
          </button>
        }
      >
        {/* Error State */}
        {error && (
          <div className="mb-6 border-y border-destructive py-3 text-destructive">
            <span className="text-sm">{error}</span>
          </div>
        )}

        {/* Loading State */}
        {loading ? (
          <div className="flex items-center border-t border-border py-12">
            <Loader2 className="w-5 h-5 animate-spin text-muted-foreground" />
          </div>
        ) : keys.length === 0 ? (
          /* Empty State */
          <div className="border-t border-border py-12">
            <Key size={32} strokeWidth={1} className="mb-6 text-muted-foreground" />
            <p className="text-3xl font-light tracking-[-0.025em]">No API keys yet</p>
            <p className="mb-8 mt-3 text-sm text-muted-foreground">
              Create your first API key to start using the Solvr API
            </p>
            <button
              type="button"
              onClick={() => setShowCreateModal(true)}
              className={LINE_BUTTON}
            >
              <Plus />
              CREATE YOUR FIRST KEY
            </button>
          </div>
        ) : (
          /* Keys List */
          <div className="border-t border-border">
            {keys.map((key) => (
              <div key={key.id} className="grid min-w-0 gap-5 border-b border-border py-6 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-start">
                <div className="min-w-0">
                  <h3 className="truncate text-2xl font-light tracking-[-0.025em] sm:text-3xl">
                    {key.name}
                  </h3>
                  <p className="mt-2 font-mono text-xs text-muted-foreground [overflow-wrap:anywhere]">
                    {key.key_preview}
                  </p>
                  <div className="mt-4 flex flex-wrap items-center gap-x-6 gap-y-1">
                    <span className={CAPTION}>
                      Created: {formatRelativeTime(key.created_at)}
                    </span>
                    <span className={CAPTION}>
                      Last used: {key.last_used_at ? formatRelativeTime(key.last_used_at) : "Never"}
                    </span>
                  </div>
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  <button
                    type="button"
                    onClick={() => setShowRegenerateModal(key.id)}
                    className={cn(LINE_BUTTON, "px-4 py-2.5")}
                  >
                    <RefreshCw />
                    REGENERATE
                  </button>
                  <button
                    type="button"
                    onClick={() => setShowRevokeModal(key.id)}
                    className={cn(DANGER_BUTTON, "px-4 py-2.5")}
                  >
                    <Trash2 />
                    REVOKE
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}

        {/* API Documentation Link */}
        <div className="mt-10">
          <p className="text-sm text-muted-foreground">
            Learn how to use your API keys in the documentation.
          </p>
          <Link
            href="/api"
            className={cn(TEXT_LINK, "mt-3 inline-block text-foreground")}
          >
            View API Documentation →
          </Link>
        </div>
      </PageSection>

      {/* What your key unlocks */}
      <PageSection heading="WHAT YOUR KEY UNLOCKS">
        <ol className="border-t border-border">
          <li className={UNLOCK_ROW}>
            <span className={CAPTION}>1</span>
            <div>
              <Link href="/api-docs" className={UNLOCK_LINK}>
                Solvr API
              </Link>
              <p className="mt-1 text-sm text-muted-foreground">
                Search, post, contribute
              </p>
            </div>
          </li>
          <li className={UNLOCK_ROW}>
            <span className={CAPTION}>2</span>
            <div>
              <Link href="/ipfs" className={UNLOCK_LINK}>
                IPFS Pinning
              </Link>
              <p className="mt-1 text-sm text-muted-foreground">
                Upload &amp; pin up to 1 GB free
              </p>
            </div>
          </li>
          <li className={UNLOCK_ROW}>
            <span className={CAPTION}>3</span>
            <div>
              <Link href="/mcp" className={UNLOCK_LINK}>
                MCP Server
              </Link>
              <p className="mt-1 text-sm text-muted-foreground">
                Connect AI tools directly
              </p>
            </div>
          </li>
        </ol>
      </PageSection>

      {/* Create Key Modal */}
      {showCreateModal && (
        <div className={MODAL_BACKDROP}>
          <div className={cn(MODAL, "max-w-md")}>
            <div className={MODAL_HEAD}>
              <h3 className={MODAL_TITLE}>CREATE API KEY</h3>
              <button
                onClick={() => {
                  setShowCreateModal(false);
                  setNewKeyName("");
                  setActionError(null);
                }}
                className={ICON_BUTTON}
              >
                <X size={16} />
              </button>
            </div>
            <div className="p-6">
              {actionError && (
                <div className="mb-4 border-y border-destructive py-2 text-destructive">
                  <span className="text-sm">{actionError}</span>
                </div>
              )}
              <label htmlFor="api-key-name" className={cn(CAPTION, "mb-3 block")}>
                KEY NAME
              </label>
              <input
                id="api-key-name"
                type="text"
                value={newKeyName}
                onChange={(e) => setNewKeyName(e.target.value)}
                placeholder="e.g., Production, Development"
                maxLength={100}
                className={FIELD}
              />
              <p className="mt-2 text-xs text-muted-foreground">
                Give your key a name to identify it later
              </p>
            </div>
            <div className={MODAL_FOOT}>
              <button
                type="button"
                onClick={() => {
                  setShowCreateModal(false);
                  setNewKeyName("");
                  setActionError(null);
                }}
                className={LINE_BUTTON}
              >
                CANCEL
              </button>
              <button
                type="button"
                onClick={handleCreateKey}
                disabled={!newKeyName.trim() || isCreating}
                className={INK_BUTTON}
              >
                {isCreating && <Loader2 className="animate-spin" />}
                {isCreating ? "CREATING..." : "CREATE KEY"}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Key Created Modal */}
      {showKeyCreatedModal && createdKey && (
        <div className={MODAL_BACKDROP}>
          <div className={cn(MODAL, "max-w-lg")}>
            <div className={MODAL_HEAD}>
              <h3 className={MODAL_TITLE}>KEY CREATED</h3>
              <button
                onClick={() => {
                  setShowKeyCreatedModal(false);
                  setCreatedKey(null);
                  setCopied(false);
                }}
                className={ICON_BUTTON}
              >
                <X size={16} />
              </button>
            </div>
            <div className="p-6">
              <div className="mb-6 flex items-start gap-2 border-y border-border py-3 text-amber-700 dark:text-amber-400">
                <AlertTriangle size={16} className="mt-0.5 shrink-0" />
                <span className="text-sm">
                  Copy your API key now. You won&apos;t be able to see it again!
                </span>
              </div>
              <label htmlFor="api-key-created" className={cn(CAPTION, "mb-3 block normal-case tracking-normal")}>
                {createdKeyName}
              </label>
              <div className="relative">
                <input
                  id="api-key-created"
                  type="text"
                  value={createdKey}
                  readOnly
                  className="w-full bg-foreground text-background px-4 py-3 pr-12 font-mono text-xs focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground"
                />
                <button
                  onClick={() => copyToClipboard(createdKey)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-background hover:text-background/70 focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-background"
                >
                  {copied ? <Check size={16} /> : <Copy size={16} />}
                </button>
              </div>
              {copied && (
                <p className="mt-2 text-xs text-green-700 dark:text-green-400">
                  Copied to clipboard!
                </p>
              )}
            </div>
            <div className={MODAL_FOOT}>
              <button
                type="button"
                onClick={() => {
                  setShowKeyCreatedModal(false);
                  setCreatedKey(null);
                  setCopied(false);
                }}
                className={INK_BUTTON}
              >
                DONE
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Revoke Confirmation Modal */}
      {showRevokeModal && (
        <div className={MODAL_BACKDROP}>
          <div className={cn(MODAL, "max-w-md")}>
            <div className={MODAL_HEAD}>
              <h3 className={cn(MODAL_TITLE, "text-destructive")}>REVOKE KEY</h3>
              <button
                onClick={() => {
                  setShowRevokeModal(null);
                  setActionError(null);
                }}
                className={ICON_BUTTON}
              >
                <X size={16} />
              </button>
            </div>
            <div className="p-6">
              {actionError && (
                <div className="mb-4 border-y border-destructive py-2 text-destructive">
                  <span className="text-sm">{actionError}</span>
                </div>
              )}
              <div className="mb-4 flex items-center gap-2 text-destructive">
                <AlertTriangle size={16} />
                <span className="text-sm">This action cannot be undone</span>
              </div>
              <p className="text-sm leading-relaxed text-muted-foreground">
                Any applications using this key will no longer be able to access the API.
              </p>
            </div>
            <div className={MODAL_FOOT}>
              <button
                type="button"
                onClick={() => {
                  setShowRevokeModal(null);
                  setActionError(null);
                }}
                className={LINE_BUTTON}
              >
                CANCEL
              </button>
              <button
                type="button"
                onClick={() => handleRevokeKey(showRevokeModal)}
                disabled={isRevoking}
                className={cn(DANGER_BUTTON, "bg-destructive text-destructive-foreground hover:bg-destructive/90")}
              >
                {isRevoking && <Loader2 className="animate-spin" />}
                {isRevoking ? "REVOKING..." : "REVOKE KEY"}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Regenerate Confirmation Modal */}
      {showRegenerateModal && (
        <div className={MODAL_BACKDROP}>
          <div className={cn(MODAL, "max-w-md")}>
            <div className={MODAL_HEAD}>
              <h3 className={MODAL_TITLE}>REGENERATE KEY</h3>
              <button
                onClick={() => {
                  setShowRegenerateModal(null);
                  setActionError(null);
                }}
                className={ICON_BUTTON}
              >
                <X size={16} />
              </button>
            </div>
            <div className="p-6">
              {actionError && (
                <div className="mb-4 border-y border-destructive py-2 text-destructive">
                  <span className="text-sm">{actionError}</span>
                </div>
              )}
              <div className="mb-4 flex items-center gap-2 text-amber-700 dark:text-amber-400">
                <AlertTriangle size={16} />
                <span className="text-sm">The old key will be invalidated</span>
              </div>
              <p className="text-sm leading-relaxed text-muted-foreground">
                A new key will be generated. Any applications using the old key will need to be updated.
              </p>
            </div>
            <div className={MODAL_FOOT}>
              <button
                type="button"
                onClick={() => {
                  setShowRegenerateModal(null);
                  setActionError(null);
                }}
                className={LINE_BUTTON}
              >
                CANCEL
              </button>
              <button
                type="button"
                onClick={() => handleRegenerateKey(showRegenerateModal)}
                disabled={isRegenerating}
                className={INK_BUTTON}
              >
                {isRegenerating && <Loader2 className="animate-spin" />}
                {isRegenerating ? "REGENERATING..." : "REGENERATE KEY"}
              </button>
            </div>
          </div>
        </div>
      )}
    </SettingsLayout>
  );
}
