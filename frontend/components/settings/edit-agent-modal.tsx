"use client";

import { useState } from "react";
import { X, Loader2, AlertCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
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
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-background/80 backdrop-blur-sm"
        onClick={handleClose}
      />

      {/* Modal */}
      <div className="relative bg-background border border-border p-6 w-full max-w-md mx-4">
        {/* Header */}
        <div className="flex items-center justify-between mb-6">
          <h2 className="font-mono text-lg font-medium">Edit Agent</h2>
          <button
            onClick={handleClose}
            className="text-muted-foreground hover:text-foreground transition-colors"
            aria-label="Close"
          >
            <X size={20} />
          </button>
        </div>

        {/* Agent Name */}
        <p className="font-mono text-sm text-muted-foreground mb-4">
          {agent.display_name}
        </p>

        {/* Error Message */}
        {error && (
          <div className="flex items-center gap-2 bg-destructive/10 border border-destructive text-destructive px-3 py-2 mb-4">
            <AlertCircle size={14} />
            <span className="font-mono text-xs">{error}</span>
          </div>
        )}

        {/* Model Input */}
        <div className="mb-6">
          <label
            htmlFor="model-input"
            className="font-mono text-xs tracking-wider text-muted-foreground block mb-2"
          >
            MODEL
          </label>
          <input
            id="model-input"
            type="text"
            value={model}
            onChange={(e) => setModel(e.target.value)}
            placeholder="e.g., claude-opus-4, gpt-4o"
            className="w-full bg-secondary/50 border border-border px-4 py-3 font-mono text-sm focus:outline-none focus:border-foreground placeholder:text-muted-foreground"
          />
          <p className="font-mono text-[10px] text-muted-foreground mt-1">
            The AI model this agent uses
          </p>
        </div>

        {/* Actions */}
        <div className="flex items-center justify-end gap-3">
          <Button
            variant="outline"
            onClick={handleClose}
            disabled={saving}
            className="font-mono text-xs tracking-wider"
          >
            CANCEL
          </Button>
          <Button
            onClick={handleSave}
            disabled={saving}
            className="font-mono text-xs tracking-wider"
          >
            {saving && <Loader2 className="w-3 h-3 mr-2 animate-spin" />}
            {saving ? "SAVING..." : "SAVE"}
          </Button>
        </div>
      </div>
    </div>
  );
}
