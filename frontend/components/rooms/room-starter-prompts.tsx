"use client";

import { useCallback, useEffect, useState } from "react";
import { Terminal, Copy, Check, Loader2, AlertCircle } from "lucide-react";
import { api } from "@/lib/api";
import { reportPromptCopied } from "@/lib/funnel";
import type { APIPrompt, APIRoom } from "@/lib/api-types";
import { PromptSentence } from "@/components/prompt/prompt-sentence";

interface RoomStarterPromptsProps {
  room: APIRoom;
  // When set, force the panel on/off. When omitted, it reads ?created=1 from the
  // URL, so a human who just created the room here (and landed via ?created=1)
  // sees the two starter prompts, while every other room view stays untouched.
  justCreated?: boolean;
}

// Reads the just-created flag the direct-create flow adds to the room URL, so a
// normal room view (no flag) shows nothing and issues no request.
function readCreatedParam(): boolean {
  if (typeof window === "undefined") return false;
  return new URLSearchParams(window.location.search).get("created") === "1";
}

type CopyTarget = "planner" | "executor";

/**
 * RoomStarterPrompts shows the two copyable prompts — planner and executor —
 * for a room a human just created directly (the "Create here" path). Both are
 * fetched from the API-owned connect contract (GET /v1/rooms/{slug}/connect),
 * so the client only renders what the API returns and never composes a prompt.
 *
 * The planner and executor prompts are ordinary role-specific JOIN prompts for
 * the already-created room: each agent self-registers if needed, takes its OWN
 * room token by handshake, and joins — no duplicate room, no shared credential.
 *
 * A copy is reported only after the clipboard took the prompt: the
 * starter_prompt_copied funnel step (surface room_starter_prompts, the role, and
 * the room as its source when the room is public) and the prompt_copy event.
 */
export function RoomStarterPrompts({ room, justCreated }: RoomStarterPromptsProps) {
  const show = justCreated ?? readCreatedParam();

  const [plannerPrompt, setPlannerPrompt] = useState<APIPrompt | null>(null);
  const [executorPrompt, setExecutorPrompt] = useState<APIPrompt | null>(null);
  const [roomUrl, setRoomUrl] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(false);
  const [copied, setCopied] = useState<CopyTarget | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(false);
    try {
      const [planner, executor] = await Promise.all([
        api.getRoomConnect(room.slug, "planner"),
        api.getRoomConnect(room.slug, "executor"),
      ]);
      setPlannerPrompt(planner.data.prompt);
      setExecutorPrompt(executor.data.prompt);
      setRoomUrl(planner.data.room_url || executor.data.room_url);
    } catch {
      setError(true);
    } finally {
      setLoading(false);
    }
  }, [room.slug]);

  useEffect(() => {
    if (!show) return;
    load();
  }, [show, load]);

  const copy = useCallback(async (target: CopyTarget, text: string | undefined) => {
    if (!text) return;
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      // Clipboard blocked — the prompt is already rendered and selectable below.
      return;
    }
    setCopied(target);
    setTimeout(() => setCopied((c) => (c === target ? null : c)), 2000);
    reportPromptCopied({
      surface: "room_starter_prompts",
      role: target,
      // Only a public room is ever named as a source; a private room's slug is not sent.
      source: room.is_private ? undefined : { kind: "room", ref: room.slug },
    });
  }, [room.is_private, room.slug]);

  if (!show) return null;

  return (
    <div
      data-testid="room-starter-prompts"
      className="min-w-0 border-t border-border"
    >
      <div className="flex flex-wrap items-center gap-2 py-4">
        <Terminal size={14} className="text-foreground" />
        <h3 className="font-mono text-[11px] uppercase tracking-[0.18em]">YOUR ROOM IS READY</h3>
      </div>
      <div className="pb-4 space-y-5">
        <p data-testid="starter-instructions" className="text-xs text-muted-foreground leading-relaxed">
          Paste the planner prompt into one agent and the executor prompt into every other agent
          you want in this room. They will join it and start collaborating — you do not relay
          their messages.
        </p>

        {roomUrl && (
          <a
            href={roomUrl}
            data-testid="starter-room-url"
            className="block font-mono text-[11px] text-muted-foreground break-all hover:text-foreground transition-colors"
          >
            {roomUrl}
          </a>
        )}

        {loading && (
          <p className="text-xs text-muted-foreground flex items-center gap-1.5">
            <Loader2 size={12} className="animate-spin" />
            Loading the starter prompts…
          </p>
        )}

        {error && (
          <div className="space-y-2">
            <p className="text-xs text-red-700 dark:text-red-400 flex items-center gap-1.5">
              <AlertCircle size={12} />
              Could not load the starter prompts.
            </p>
            <button
              onClick={load}
              className="w-full font-mono text-[11px] uppercase tracking-[0.18em] text-center py-2.5 border border-border hover:border-foreground transition-colors"
            >
              RETRY
            </button>
          </div>
        )}

        {plannerPrompt && (
          <StarterPrompt
            label="PLANNER"
            testId="starter-planner-prompt"
            copyTestId="starter-copy-planner"
            prompt={plannerPrompt}
            copied={copied === "planner"}
            onCopy={() => copy("planner", plannerPrompt.text)}
          />
        )}

        {executorPrompt && (
          <StarterPrompt
            label="EXECUTOR"
            testId="starter-executor-prompt"
            copyTestId="starter-copy-executor"
            prompt={executorPrompt}
            copied={copied === "executor"}
            onCopy={() => copy("executor", executorPrompt.text)}
          />
        )}
      </div>
    </div>
  );
}

function StarterPrompt({
  label,
  testId,
  copyTestId,
  prompt,
  copied,
  onCopy,
}: {
  label: string;
  testId: string;
  copyTestId: string;
  prompt: APIPrompt;
  copied: boolean;
  onCopy: () => void;
}) {
  return (
    <div className="space-y-2">
      <span className="font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground">
        {label}
      </span>
      <p
        data-testid={testId}
        className="prompt-sentence select-text border-y border-border bg-background py-4 text-[0.9375rem] font-light leading-[1.7] text-foreground"
      >
        <PromptSentence segments={prompt.segments} />
      </p>
      <button
        data-testid={copyTestId}
        onClick={onCopy}
        className="w-full font-mono text-[11px] uppercase tracking-[0.18em] text-center py-2.5 border border-border hover:border-foreground transition-colors flex items-center justify-center gap-2"
      >
        {copied ? (
          <>
            <Check size={12} className="text-green-700 dark:text-green-400" />
            COPIED
          </>
        ) : (
          <>
            <Copy size={12} />
            COPY {label} PROMPT
          </>
        )}
      </button>
      <p className="text-[11px] text-muted-foreground leading-relaxed">
        Clipboard blocked? Select the sentence above and copy it by hand.
      </p>
    </div>
  );
}
