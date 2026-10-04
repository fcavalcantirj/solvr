"use client";

import { useState } from "react";
import { X, Loader2, AlertCircle } from "lucide-react";
import { CAPTION } from "@/components/page/caption";
import { FIELD, ICON_BUTTON, INK_BUTTON, LINE_BUTTON } from "@/components/page/controls";
import { cn } from "@/lib/utils";
import { api } from "@/lib/api";
import type { APIAgent } from "@/lib/api-types";

interface EditAgentModalProps {
  agent: APIAgent;
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

export function EditAgentModal({
  agent,
  isOpen,
  onClose,
  onSuccess,
}: EditAgentModalProps) {
  const [model, setModel] = useState(agent.model || "");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!isOpen) return null;

  const handleSave = async () => {
    setSaving(true);
    setError(null);

    try {
      await api.updateAgent(agent.id, { model: model.trim() || undefined });
      onSuccess();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Update failed");
    } finally {
      setSaving(false);
    }
  };

  const handleClose = () => {
    setError(null);
    setModel(agent.model || "");
    onClose();
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-background/80 backdrop-blur-sm"
        onClick={handleClose}
      />

      {/* Modal */}
      <div className="relative w-full max-w-md border border-foreground bg-background">
        {/* Header */}
        <div className="flex items-center justify-between gap-4 border-b border-border px-6 py-5">
          <h2 className="text-2xl font-light tracking-[-0.025em]">Edit Agent</h2>
          <button
            onClick={handleClose}
            className={ICON_BUTTON}
            aria-label="Close"
          >
            <X size={18} />
          </button>
        </div>

        <div className="px-6 py-6">
          {/* Agent Name */}
          <p className="mb-6 text-base [overflow-wrap:anywhere]">
            {agent.display_name}
          </p>

          {/* Error Message */}
          {error && (
            <div className="mb-6 flex items-center gap-2 border-y border-destructive py-3 text-destructive">
              <AlertCircle size={14} />
              <span className="text-sm">{error}</span>
            </div>
          )}

          {/* Model Input */}
          <div>
            <label
              htmlFor="model-input"
              className={cn(CAPTION, "mb-3 block")}
            >
              MODEL
            </label>
            <input
              id="model-input"
              type="text"
              value={model}
              onChange={(e) => setModel(e.target.value)}
              placeholder="e.g., claude-opus-4, gpt-4o"
              className={FIELD}
            />
            <p className="mt-2 text-xs text-muted-foreground">
              The AI model this agent uses
            </p>
          </div>
        </div>

        {/* Actions */}
        <div className="flex items-center justify-end gap-3 border-t border-border px-6 py-5">
          <button
            type="button"
            onClick={handleClose}
            disabled={saving}
            className={LINE_BUTTON}
          >
            CANCEL
          </button>
          <button
            type="button"
            onClick={handleSave}
            disabled={saving}
            className={INK_BUTTON}
          >
            {saving && <Loader2 className="animate-spin" />}
            {saving ? "SAVING..." : "SAVE"}
          </button>
        </div>
      </div>
    </div>
  );
}
