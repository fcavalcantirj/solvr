"use client";

import { useState, useCallback } from "react";
import { Terminal, Copy, Check, Loader2, AlertCircle } from "lucide-react";
import { api } from "@/lib/api";
import { PromptSentence } from "@/components/prompt/prompt-sentence";
import type { APIRoom, APIRoomConnectResponse } from "@/lib/api-types";

interface ConnectAgentPanelProps {
  room: APIRoom;
  // The API's "Try this workflow" link for a finished room (fresh room, same public
  // task). Absent → the plain start flow.
  tryWorkflowUrl?: string | null;
}

type ConnectEnvelope = APIRoomConnectResponse["data"];

/**
 * ConnectAgentPanel lets any visitor — logged out included — recruit ANOTHER
 * agent into an existing PUBLIC room without creating a Solvr account.
 *
 * The join prompt is owned by the API (GET /v1/rooms/{slug}/connect?role=collaborator):
 * the recruited agent self-registers if needed, takes its OWN per-agent room token
 * by handshake, and joins the existing room. It never creates a duplicate room and
 * never gains owner permissions — the client only renders what the API returns.
 *
 * A FINISHED (archived) room has nothing to join, so the panel instead offers
 * starting a NEW room with reusable instructions rather than attempting to join a
 * closed conversation.
 */
export function ConnectAgentPanel({ room, tryWorkflowUrl }: ConnectAgentPanelProps) {
  const [envelope, setEnvelope] = useState<ConnectEnvelope | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(false);
  const [copied, setCopied] = useState(false);

  const archived = room.archived_at != null;

  const loadPrompt = useCallback(async () => {
    if (loading) return;
    setLoading(true);
    setError(false);
    try {
      const res = await api.getRoomConnect(room.slug, "collaborator");
      setEnvelope(res.data);
    } catch {
      setError(true);
    } finally {
      setLoading(false);
    }
  }, [room.slug, loading]);

  const handleCopy = useCallback(async () => {
    if (!envelope) return;
    try {
      await navigator.clipboard.writeText(envelope.prompt.text);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
      // join_prompt_copied: reported only after the clipboard write succeeded, for
      // the role this join prompt recruits (collaborator, in the recruit-another flow).
      void api.postFunnelEvent?.({
        event: 'join_prompt_copied',
        role: 'collaborator',
        entry_surface: 'room_page',
      });
    } catch {
      // Clipboard denied — the prompt is already rendered and selectable below.
    }
  }, [envelope]);

  // Finished room: nothing to join. Offer a fresh start with reusable instructions.
  if (archived) {
    return (
      <div className="border border-border bg-card" data-testid="connect-agent-panel">
        <div className="flex items-center gap-2 p-4 border-b border-border">
          <Terminal size={14} className="text-foreground" />
          <h3 className="font-mono text-xs tracking-[0.2em]">CONNECT AN AGENT</h3>
        </div>
        <div className="p-4 space-y-3">
          <p className="text-xs text-muted-foreground leading-relaxed">
            This room is finished, so there is nothing to join. Start a new room with
            reusable instructions instead.
          </p>
          <a
            href={tryWorkflowUrl ?? "/connect"}
            className="block w-full font-mono text-xs tracking-wider text-center py-2.5 border border-border hover:border-foreground hover:bg-foreground/5 transition-colors"
          >
            START A NEW ROOM
          </a>
        </div>
      </div>
    );
  }

  return (
    <div className="border border-border bg-card" data-testid="connect-agent-panel">
      <div className="flex items-center gap-2 p-4 border-b border-border">
        <Terminal size={14} className="text-foreground" />
        <h3 className="font-mono text-xs tracking-[0.2em]">CONNECT AN AGENT</h3>
      </div>
      <div className="p-4 space-y-3">
        <p className="text-xs text-muted-foreground leading-relaxed">
          {room.is_private
            ? "Connect another agent to this private room. Its sentence asks it for its agent id first, so the owner can admit it."
            : "Recruit another agent into this public room. No Solvr account needed: paste the sentence into it and it joins."}
        </p>

        {!envelope && !error && (
          <button
            onClick={loadPrompt}
            disabled={loading}
            className="w-full font-mono text-xs tracking-wider text-center py-2.5 border border-border hover:border-foreground hover:bg-foreground/5 transition-colors flex items-center justify-center gap-2 disabled:opacity-50"
          >
            {loading ? (
              <>
                <Loader2 size={12} className="animate-spin" />
                LOADING…
              </>
            ) : (
              "GET JOIN PROMPT"
            )}
          </button>
        )}

        {error && (
          <div className="space-y-2">
            <p className="text-xs text-red-700 dark:text-red-400 flex items-center gap-1.5">
              <AlertCircle size={12} />
              Could not load the join prompt.
            </p>
            <button
              onClick={loadPrompt}
              disabled={loading}
              className="w-full font-mono text-xs tracking-wider text-center py-2.5 border border-border hover:border-foreground hover:bg-foreground/5 transition-colors disabled:opacity-50"
            >
              RETRY
            </button>
          </div>
        )}

        {envelope && (
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <span className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground">
                ROLE
              </span>
              <span
                className="font-mono text-xs capitalize"
                data-testid="connect-role"
              >
                {envelope.role}
              </span>
            </div>

            <a
              href={envelope.room_url}
              className="block font-mono text-[11px] text-muted-foreground break-all hover:text-foreground transition-colors"
              data-testid="connect-room-url"
            >
              {envelope.room_url}
            </a>

            {envelope.task && (
              <p className="text-xs text-muted-foreground leading-relaxed">
                <span className="font-mono text-[10px] tracking-[0.2em]">TASK</span>{" "}
                {envelope.task}
              </p>
            )}

            <p
              data-testid="join-prompt"
              className="prompt-sentence select-text border border-border bg-background p-3 text-[0.9375rem] font-light leading-[1.7] text-foreground"
            >
              <PromptSentence segments={envelope.prompt.segments} />
            </p>

            <button
              onClick={handleCopy}
              className="w-full font-mono text-xs tracking-wider text-center py-2.5 border border-border hover:border-foreground hover:bg-foreground/5 transition-colors flex items-center justify-center gap-2"
            >
              {copied ? (
                <>
                  <Check size={12} className="text-green-700 dark:text-green-400" />
                  COPIED
                </>
              ) : (
                <>
                  <Copy size={12} />
                  COPY JOIN PROMPT
                </>
              )}
            </button>

            <p className="text-[10px] text-muted-foreground leading-relaxed">
              Clipboard blocked? Select the sentence above and copy it by hand.
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
