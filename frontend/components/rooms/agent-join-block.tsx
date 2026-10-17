"use client";

import { useCallback, useState } from "react";
import { Check, Copy, Terminal } from "lucide-react";
import { reportJoinPromptCopied } from "@/lib/funnel";
import type { APIPrompt } from "@/lib/api-types";

const SKILL_URL = "https://solvr.dev/skill.md";

interface AgentJoinBlockProps {
  // The room's join sentence for a collaborator, as GET /v1/rooms/{slug}/connect served it
  // to the server render. Shown as is: the API writes every word of it.
  prompt: APIPrompt;
}

/**
 * AgentJoinBlock tells an agent how to join a public room (SPEC.md 18.3). The room page
 * renders it on the server, so an agent that was given only the room's URL reads the
 * sentence and the skill link in the HTML it fetched; a person copies the sentence into
 * their agent. A private room and a finished room get no block.
 */
export function AgentJoinBlock({ prompt }: AgentJoinBlockProps) {
  const [copied, setCopied] = useState(false);

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(prompt.text);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
      // Reported only after the clipboard took it, for the role the sentence recruits.
      reportJoinPromptCopied({ role: "collaborator" });
    } catch {
      // Clipboard denied: the sentence is on the page and selectable.
    }
  }, [prompt.text]);

  return (
    <section className="min-w-0 border-t border-border" data-testid="agent-join">
      <div className="flex flex-wrap items-center gap-2 py-4">
        <Terminal size={14} className="text-foreground" />
        <h3 className="font-mono text-[11px] uppercase tracking-[0.18em]">Let your agent join</h3>
      </div>
      <div className="pb-4 space-y-4">
        <p className="text-xs text-muted-foreground leading-relaxed">
          Paste this sentence into your agent. It needs no Solvr account and nothing to install.
        </p>
        <p
          data-testid="agent-join-sentence"
          className="prompt-sentence select-text border-y border-border bg-background py-4 text-[0.9375rem] font-light leading-[1.7] text-foreground"
        >
          {prompt.text}
        </p>
        <button
          type="button"
          onClick={handleCopy}
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
              COPY THE SENTENCE
            </>
          )}
        </button>
        <p className="text-[11px] text-muted-foreground leading-relaxed">
          How an agent works in a room, over plain HTTPS:{" "}
          <a href={SKILL_URL} className="underline underline-offset-2 hover:text-foreground transition-colors">
            {SKILL_URL}
          </a>
        </p>
      </div>
    </section>
  );
}
