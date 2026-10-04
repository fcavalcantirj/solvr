import type { ReactElement } from "react";

// The badge renders the room's connection progress exactly as the API reports
// it. The status is computed on the server from real presence and the persisted
// two-way exchange milestone; this component only maps the known values to a
// human label and a restrained indicator. It never derives progress from the
// participant list — the API is smart, the client is dumb.
const STATUS_LABELS: Record<string, string> = {
  waiting_for_agents: "Waiting for agents",
  waiting_for_another_agent: "Waiting for another agent",
  conversation_started: "Conversation started",
};

interface ConnectionStatusBadgeProps {
  status?: string;
}

export function ConnectionStatusBadge({
  status,
}: ConnectionStatusBadgeProps): ReactElement | null {
  // Unknown or missing status: render nothing rather than invent a state.
  const label = status ? STATUS_LABELS[status] : undefined;
  if (!label) return null;

  const started = status === "conversation_started";

  return (
    <div
      role="status"
      className="inline-flex items-center gap-2 font-mono text-[11px] tracking-[0.18em] uppercase text-muted-foreground"
    >
      {started ? (
        <span
          data-testid="connection-live-dot"
          aria-hidden="true"
          className="w-2 h-2 rounded-full bg-green-700 dark:bg-green-400 animate-pulse"
        />
      ) : (
        <span
          aria-hidden="true"
          className="w-2 h-2 rounded-full border border-muted-foreground/50"
        />
      )}
      <span>{label}</span>
    </div>
  );
}
