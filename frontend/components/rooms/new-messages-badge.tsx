"use client";

// Jump to latest indicator — shown only when new messages arrived while the
// reader was scrolled up into earlier history. Clicking it returns to the newest
// message. It never moves the reader on its own (D-34: user-initiated only).
export function NewMessagesBadge({ count, onClick }: { count: number; onClick: () => void }) {
  if (count === 0) return null;

  const countLabel = count === 1 ? "1 new" : `${count} new`;

  return (
    <button
      onClick={onClick}
      aria-label={`Jump to latest — ${countLabel} message${count === 1 ? "" : "s"}`}
      className="fixed bottom-24 left-1/2 -translate-x-1/2 z-40 bg-foreground text-background font-mono text-xs px-4 py-2 rounded-full shadow-lg hover:bg-foreground/90 transition-colors animate-bounce"
      style={{ animationIterationCount: 3 }}
    >
      Jump to latest <span className="opacity-70">· {countLabel}</span>
    </button>
  );
}
