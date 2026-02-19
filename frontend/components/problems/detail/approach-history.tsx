"use client";

import { ArrowUp, Plus, Lightbulb } from "lucide-react";
import type { APIApproachVersionHistory, APIApproachWithAuthor, APIApproachRelationship } from "@/lib/api-types";

interface ApproachHistoryProps {
  history: APIApproachVersionHistory;
}

const relationConfig: Record<string, { label: string; icon: typeof ArrowUp }> = {
  updates: { label: "UPDATES", icon: ArrowUp },
  extends: { label: "EXTENDS", icon: Plus },
  derives: { label: "DERIVES", icon: Lightbulb },
};

function RelationshipBadge({ type }: { type: string }) {
  const config = relationConfig[type] || relationConfig.updates;
  const Icon = config.icon;
  return (
    <span className="font-mono text-[10px] tracking-wider text-muted-foreground flex items-center gap-1 px-2 py-0.5 border border-border">
      <Icon size={10} />
      {config.label}
    </span>
  );
}

function VersionEntry({
  approach,
  isCurrent,
  relationship,
}: {
  approach: APIApproachWithAuthor;
  isCurrent: boolean;
  relationship?: APIApproachRelationship;
}) {
  const statusKey = approach.status.toLowerCase();
  const isArchived = false; // TODO: add archived_at to APIApproachWithAuthor when needed

  return (
    <div className={`relative ${isArchived ? "opacity-50" : ""}`}>
      {/* Relationship connector */}
      {relationship && (
        <div className="flex items-center gap-2 py-1.5 pl-4">
          <div className="w-px h-4 bg-border" />
          <RelationshipBadge type={relationship.relation_type} />
        </div>
      )}

      {/* Version card */}
      <div
        className={`border p-4 ${
          isCurrent
            ? "border-foreground bg-card"
            : "border-border bg-card/50"
        }`}
      >
        <div className="flex items-center gap-2 mb-2">
          <span
            className={`font-mono text-[10px] tracking-wider px-2 py-0.5 ${
              statusKey === "succeeded" || statusKey === "working"
                ? "bg-foreground text-background"
                : "bg-secondary text-muted-foreground"
            }`}
          >
            {approach.status.toUpperCase()}
          </span>
          {isCurrent && (
            <span className="font-mono text-[10px] tracking-wider text-foreground px-2 py-0.5 border border-foreground">
              CURRENT
            </span>
          )}
        </div>

        <p className="text-sm font-light leading-snug mb-1">
          {approach.angle}
        </p>

        {approach.method && (
          <p className="text-xs text-muted-foreground">
            {approach.method}
          </p>
        )}

        {approach.outcome && (
          <p className="text-xs text-muted-foreground mt-2 italic">
            {approach.outcome}
          </p>
        )}
      </div>
    </div>
  );
}

export function ApproachHistory({ history }: ApproachHistoryProps) {
  const hasHistory = history.history.length > 0;

  return (
    <div className="space-y-2">
      <p className="font-mono text-[10px] tracking-wider text-muted-foreground">
        VERSION HISTORY
      </p>

      {!hasHistory ? (
        <p className="text-sm text-muted-foreground">No version history</p>
      ) : (
        <div className="space-y-0">
          {/* History entries (oldest first) */}
          {history.history.map((approach) => {
            // Find the relationship pointing TO this approach (from the next version)
            const rel = history.relationships.find(
              (r) => r.to_approach_id === approach.id
            );
            return (
              <VersionEntry
                key={approach.id}
                approach={approach}
                isCurrent={false}
                relationship={rel}
              />
            );
          })}

          {/* Current version — no relationship badge (it's the latest) */}
          <VersionEntry
            approach={history.current}
            isCurrent={true}
          />
        </div>
      )}
    </div>
  );
}
