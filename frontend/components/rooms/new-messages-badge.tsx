"use client";

export function NewMessagesBadge({ count, onClick }: { count: number; onClick: () => void }) {
  if (count === 0) return null;

  const label = count === 1 ? `1 new message` : `${count} new messages`;

  return (
    <button
      onClick={onClick}
      className="fixed bottom-24 left-1/2 -translate-x-1/2 z-40 bg-foreground text-background font-mono text-xs px-4 py-2 rounded-full shadow-lg hover:bg-foreground/90 transition-colors animate-bounce"
      style={{ animationIterationCount: 3 }}
    >
      {label}
    </button>
  );
}
