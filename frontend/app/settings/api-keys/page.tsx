"use client";

import { useState } from "react";
import Link from "next/link";
import { useAPIKeys } from "@/hooks/use-api-keys";
import { SettingsLayout } from "@/components/settings/settings-layout";
import { Button } from "@/components/ui/button";
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
      {/* Header */}
      <div className="border border-border p-8 mb-6">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="font-mono text-xs tracking-wider text-muted-foreground mb-2">
              API KEYS
            </h2>
            <p className="font-mono text-sm text-muted-foreground">
              Create and manage API keys for programmatic access to Solvr.
            </p>
          </div>
          <Button
            onClick={() => setShowCreateModal(true)}
            className="font-mono text-xs tracking-wider"
          >
            <Plus className="w-3 h-3 mr-2" />
            CREATE KEY
          </Button>
        </div>
      </div>

      {/* Error State */}
      {error && (
        <div className="border border-destructive bg-destructive/10 text-destructive px-4 py-3 mb-6">
          <span className="font-mono text-xs">{error}</span>
        </div>
      )}

      {/* Loading State */}
      {loading ? (
        <div className="border border-border p-12 flex items-center justify-center">
          <Loader2 className="w-6 h-6 animate-spin text-muted-foreground" />
        </div>
      ) : keys.length === 0 ? (
        /* Empty State */
        <div className="border border-dashed border-border p-12 text-center">
          <Key size={32} className="mx-auto mb-4 text-muted-foreground" />
          <p className="font-mono text-sm mb-2">No API keys yet</p>
          <p className="font-mono text-xs text-muted-foreground mb-6">
            Create your first API key to start using the Solvr API
          </p>
          <Button
            onClick={() => setShowCreateModal(true)}
            variant="outline"
            className="font-mono text-xs tracking-wider"
          >
            <Plus className="w-3 h-3 mr-2" />
            CREATE YOUR FIRST KEY
          </Button>
        </div>
      ) : (
        /* Keys List */
        <div className="space-y-4">
          {keys.map((key) => (
            <div key={key.id} className="border border-border p-6">
              <div className="flex items-start justify-between">
                <div className="flex-1 min-w-0">
                  <h3 className="font-mono text-sm font-medium truncate">
                    {key.name}
                  </h3>
                  <p className="font-mono text-xs text-muted-foreground mt-1">
                    {key.key_preview}
                  </p>
                  <div className="flex items-center gap-4 mt-3">
                    <span className="font-mono text-[10px] text-muted-foreground">
                      Created: {formatRelativeTime(key.created_at)}
                    </span>
                    <span className="font-mono text-[10px] text-muted-foreground">
                      Last used: {key.last_used_at ? formatRelativeTime(key.last_used_at) : "Never"}
                    </span>
                  </div>
                </div>
                <div className="flex items-center gap-2 ml-4">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setShowRegenerateModal(key.id)}
                    className="font-mono text-[10px] tracking-wider"
                  >
                    <RefreshCw className="w-3 h-3 mr-1" />
                    REGENERATE
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setShowRevokeModal(key.id)}
                    className="font-mono text-[10px] tracking-wider text-destructive border-destructive hover:bg-destructive hover:text-destructive-foreground"
                  >
                    <Trash2 className="w-3 h-3 mr-1" />
                    REVOKE
                  </Button>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* API Documentation Link */}
      <div className="mt-8 pt-8 border-t border-border">
        <p className="font-mono text-xs text-muted-foreground">
          Learn how to use your API keys in the documentation.
        </p>
        <Link
          href="/api"
          className="inline-block mt-2 font-mono text-xs tracking-wider text-foreground hover:underline"
        >
          View API Documentation →
        </Link>
      </div>

      {/* Create Key Modal */}
      {showCreateModal && (
        <div className="fixed inset-0 bg-background/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-background border border-border w-full max-w-md">
            <div className="flex items-center justify-between px-6 py-4 border-b border-border">
              <h3 className="font-mono text-sm tracking-wider">CREATE API KEY</h3>
              <button
                onClick={() => {
                  setShowCreateModal(false);
                  setNewKeyName("");
                  setActionError(null);
                }}
                className="text-muted-foreground hover:text-foreground"
              >
                <X size={16} />
              </button>
            </div>
            <div className="p-6">
              {actionError && (
                <div className="bg-destructive/10 border border-destructive text-destructive px-4 py-2 mb-4">
                  <span className="font-mono text-xs">{actionError}</span>
                </div>
              )}
              <label className="font-mono text-xs tracking-wider text-muted-foreground block mb-2">
                KEY NAME
              </label>
              <input
                type="text"
                value={newKeyName}
                onChange={(e) => setNewKeyName(e.target.value)}
                placeholder="e.g., Production, Development"
                maxLength={100}
                className="w-full bg-secondary/50 border border-border px-4 py-3 font-mono text-sm focus:outline-none focus:border-foreground placeholder:text-muted-foreground"
              />
              <p className="font-mono text-[10px] text-muted-foreground mt-2">
                Give your key a name to identify it later
              </p>
            </div>
            <div className="flex justify-end gap-3 px-6 py-4 border-t border-border">
              <Button
                variant="outline"
                onClick={() => {
                  setShowCreateModal(false);
                  setNewKeyName("");
                  setActionError(null);
                }}
                className="font-mono text-xs tracking-wider"
              >
                CANCEL
              </Button>
              <Button
                onClick={handleCreateKey}
                disabled={!newKeyName.trim() || isCreating}
                className="font-mono text-xs tracking-wider"
              >
                {isCreating && <Loader2 className="w-3 h-3 mr-2 animate-spin" />}
                {isCreating ? "CREATING..." : "CREATE KEY"}
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Key Created Modal */}
      {showKeyCreatedModal && createdKey && (
        <div className="fixed inset-0 bg-background/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-background border border-border w-full max-w-lg">
            <div className="flex items-center justify-between px-6 py-4 border-b border-border">
              <h3 className="font-mono text-sm tracking-wider">KEY CREATED</h3>
              <button
                onClick={() => {
                  setShowKeyCreatedModal(false);
                  setCreatedKey(null);
                  setCopied(false);
                }}
                className="text-muted-foreground hover:text-foreground"
              >
                <X size={16} />
              </button>
            </div>
            <div className="p-6">
              <div className="flex items-center gap-2 bg-amber-500/10 border border-amber-500 text-amber-600 px-4 py-3 mb-6">
                <AlertTriangle size={16} />
                <span className="font-mono text-xs">
                  Copy your API key now. You won&apos;t be able to see it again!
                </span>
              </div>
              <label className="font-mono text-xs tracking-wider text-muted-foreground block mb-2">
                {createdKeyName}
              </label>
              <div className="relative">
                <input
                  type="text"
                  value={createdKey}
                  readOnly
                  className="w-full bg-foreground text-background px-4 py-3 pr-12 font-mono text-xs focus:outline-none"
                />
                <button
                  onClick={() => copyToClipboard(createdKey)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-background hover:text-background/70"
                >
                  {copied ? <Check size={16} /> : <Copy size={16} />}
                </button>
              </div>
              {copied && (
                <p className="font-mono text-[10px] text-emerald-600 mt-2">
                  Copied to clipboard!
                </p>
              )}
            </div>
            <div className="flex justify-end px-6 py-4 border-t border-border">
              <Button
                onClick={() => {
                  setShowKeyCreatedModal(false);
                  setCreatedKey(null);
                  setCopied(false);
                }}
                className="font-mono text-xs tracking-wider"
              >
                DONE
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Revoke Confirmation Modal */}
      {showRevokeModal && (
        <div className="fixed inset-0 bg-background/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-background border border-border w-full max-w-md">
            <div className="flex items-center justify-between px-6 py-4 border-b border-border">
              <h3 className="font-mono text-sm tracking-wider text-destructive">REVOKE KEY</h3>
              <button
                onClick={() => {
                  setShowRevokeModal(null);
                  setActionError(null);
                }}
                className="text-muted-foreground hover:text-foreground"
              >
                <X size={16} />
              </button>
            </div>
            <div className="p-6">
              {actionError && (
                <div className="bg-destructive/10 border border-destructive text-destructive px-4 py-2 mb-4">
                  <span className="font-mono text-xs">{actionError}</span>
                </div>
              )}
              <div className="flex items-center gap-2 text-destructive mb-4">
                <AlertTriangle size={16} />
                <span className="font-mono text-xs">This action cannot be undone</span>
              </div>
              <p className="font-mono text-sm text-muted-foreground">
                Any applications using this key will no longer be able to access the API.
              </p>
            </div>
            <div className="flex justify-end gap-3 px-6 py-4 border-t border-border">
              <Button
                variant="outline"
                onClick={() => {
                  setShowRevokeModal(null);
                  setActionError(null);
                }}
                className="font-mono text-xs tracking-wider"
              >
                CANCEL
              </Button>
              <Button
                onClick={() => handleRevokeKey(showRevokeModal)}
                disabled={isRevoking}
                className="font-mono text-xs tracking-wider bg-destructive text-destructive-foreground hover:bg-destructive/90"
              >
                {isRevoking && <Loader2 className="w-3 h-3 mr-2 animate-spin" />}
                {isRevoking ? "REVOKING..." : "REVOKE KEY"}
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Regenerate Confirmation Modal */}
      {showRegenerateModal && (
        <div className="fixed inset-0 bg-background/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-background border border-border w-full max-w-md">
            <div className="flex items-center justify-between px-6 py-4 border-b border-border">
              <h3 className="font-mono text-sm tracking-wider">REGENERATE KEY</h3>
              <button
                onClick={() => {
                  setShowRegenerateModal(null);
                  setActionError(null);
                }}
                className="text-muted-foreground hover:text-foreground"
              >
                <X size={16} />
              </button>
            </div>
            <div className="p-6">
              {actionError && (
                <div className="bg-destructive/10 border border-destructive text-destructive px-4 py-2 mb-4">
                  <span className="font-mono text-xs">{actionError}</span>
                </div>
              )}
              <div className="flex items-center gap-2 text-amber-600 mb-4">
                <AlertTriangle size={16} />
                <span className="font-mono text-xs">The old key will be invalidated</span>
              </div>
              <p className="font-mono text-sm text-muted-foreground">
                A new key will be generated. Any applications using the old key will need to be updated.
              </p>
            </div>
            <div className="flex justify-end gap-3 px-6 py-4 border-t border-border">
              <Button
                variant="outline"
                onClick={() => {
                  setShowRegenerateModal(null);
                  setActionError(null);
                }}
                className="font-mono text-xs tracking-wider"
              >
                CANCEL
              </Button>
              <Button
                onClick={() => handleRegenerateKey(showRegenerateModal)}
                disabled={isRegenerating}
                className="font-mono text-xs tracking-wider"
              >
                {isRegenerating && <Loader2 className="w-3 h-3 mr-2 animate-spin" />}
                {isRegenerating ? "REGENERATING..." : "REGENERATE KEY"}
              </Button>
            </div>
          </div>
        </div>
      )}
    </SettingsLayout>
  );
}
