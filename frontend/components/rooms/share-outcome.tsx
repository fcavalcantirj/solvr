"use client";

import { useCallback, useState } from "react";
import { ClipboardCopy } from "lucide-react";
import { api } from "@/lib/api";
import type { APIRoomShare } from "@/lib/api-types";

/**
 * ShareOutcome (idx 88 step 3): an OPTIONAL shareable excerpt of a public room's outcome.
 * The API composes it — clean links, the excerpt and the exact text to copy — and this
 * control only shows it and copies it when asked. Nothing is ever posted anywhere: the
 * person decides whether, and where, to share it. share_link_copied is reported only
 * after the clipboard write succeeded.
 */
export function ShareOutcome({ slug }: { slug: string }) {
  const [open, setOpen] = useState(false);
  const [share, setShare] = useState<APIRoomShare | null>(null);
  const [failed, setFailed] = useState(false);
  const [copied, setCopied] = useState(false);

  const toggle = useCallback(async () => {
    const next = !open;
    setOpen(next);
    if (!next || share) return;
    try {
      const res = await api.getRoomShare(slug);
      setShare(res.data);
      setFailed(false);
    } catch {
      setFailed(true);
    }
  }, [open, share, slug]);

  const copy = useCallback(async () => {
    if (!share) return;
    try {
      await navigator.clipboard.writeText(share.copy_text);
      setCopied(true);
      void api.postFunnelEvent?.({
        event: "share_link_copied",
        entry_surface: "room_page",
        source: { kind: "room", ref: slug },
      });
    } catch {
      setCopied(false);
    }
  }, [share, slug]);

  return (
    <div className="relative">
      <button
        type="button"
        onClick={toggle}
        aria-expanded={open}
        className="inline-flex items-center gap-1.5 border border-border px-3 py-1.5 font-mono text-xs tracking-wider hover:bg-muted transition-colors"
      >
        <ClipboardCopy className="w-3.5 h-3.5" aria-hidden="true" />
        Copy outcome
      </button>
      {open && (
        <div
          data-testid="share-outcome-panel"
          className="mt-2 sm:absolute sm:right-0 sm:z-10 w-full sm:w-80 border border-border bg-card p-3 space-y-2"
        >
          {failed && (
            <p role="alert" className="text-xs text-muted-foreground">
              The outcome could not be read. Try again in a moment.
            </p>
          )}
          {!failed && !share && <p className="font-mono text-xs text-muted-foreground">Reading the outcome…</p>}
          {share && (
            <>
              <p className="font-mono text-xs tracking-wider">{share.excerpt.title}</p>
              {share.excerpt.text && <p className="text-xs leading-relaxed">{share.excerpt.text}</p>}
              <pre
                data-testid="share-outcome-text"
                className="max-h-32 overflow-auto bg-background border border-border p-2 font-mono text-[11px] whitespace-pre-wrap break-words select-all"
              >
                {share.copy_text}
              </pre>
              <button
                type="button"
                onClick={copy}
                className="w-full font-mono text-xs tracking-wider py-1.5 border border-border hover:border-foreground transition-colors"
              >
                {copied ? "Copied" : "Copy"}
              </button>
              <p className="text-[10px] leading-relaxed text-muted-foreground">{share.note}</p>
            </>
          )}
        </div>
      )}
    </div>
  );
}
