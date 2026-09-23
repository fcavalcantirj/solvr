"use client";

import { useCallback, useEffect, useState } from "react";
import { Terminal, Copy, Check, Loader2, AlertCircle } from "lucide-react";
import { api } from "@/lib/api";
import type { APIRoom } from "@/lib/api-types";

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
 */
export function RoomStarterPrompts({ room, justCreated }: RoomStarterPromptsProps) {
  const show = justCreated ?? readCreatedParam();

  const [plannerPrompt, setPlannerPrompt] = useState<string | null>(null);
  const [executorPrompt, setExecutorPrompt] = useState<string | null>(null);
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
      setExecutorPrompt(executor.data.executor_prompt);
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

  const copy = useCallback(async (target: CopyTarget, text: string | null) => {
    if (!text) return;
    try {
      await navigator.clipboard.writeText(text);
      setCopied(target);
      setTimeout(() => setCopied((c) => (c === target ? null : c)), 2000);
    } catch {
      // Clipboard blocked — the prompt is already rendered and selectable below.
    }
  }, []);

  if (!show) return null;

  return (
    <div
      data-testid="room-starter-prompts"
      className="mb-4 border border-border bg-card"
    >
      <div className="flex items-center gap-2 p-4 border-b border-border">
        <Terminal size={14} className="text-foreground" />
        <h3 className="font-mono text-xs tracking-[0.2em]">YOUR ROOM IS READY</h3>
      </div>
      <div className="p-4 space-y-4">
        <p className="text-xs text-muted-foreground leading-relaxed">
          Paste each prompt into one of your two agents. They will join this room and
          start collaborating — you do not relay their messages.
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
              className="w-full font-mono text-xs tracking-wider text-center py-2.5 border border-border hover:border-foreground hover:bg-foreground/5 transition-colors"
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
            onCopy={() => copy("planner", plannerPrompt)}
          />
        )}

        {executorPrompt && (
          <StarterPrompt
            label="EXECUTOR"
            testId="starter-executor-prompt"
            copyTestId="starter-copy-executor"
            prompt={executorPrompt}
            copied={copied === "executor"}
            onCopy={() => copy("executor", executorPrompt)}
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
  prompt: string;
  copied: boolean;
  onCopy: () => void;
}) {
  return (
    <div className="space-y-2">
      <span className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground">
        {label}
      </span>
      <pre
        data-testid={testId}
        className="whitespace-pre-wrap break-words text-[11px] font-mono bg-secondary/40 p-3 border border-border max-h-64 overflow-y-auto select-text"
      >
        {prompt}
      </pre>
      <button
        data-testid={copyTestId}
        onClick={onCopy}
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
            COPY {label} PROMPT
          </>
        )}
      </button>
      <p className="text-[10px] text-muted-foreground leading-relaxed">
        Clipboard blocked? Select the prompt above and copy it manually.
      </p>
    </div>
  );
}
