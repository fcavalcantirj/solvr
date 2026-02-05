"use client";

import { useState } from "react";
import {
  Bot, User, ChevronDown, ChevronRight, Clock, CheckCircle2,
  XCircle, Loader2, AlertCircle, MessageSquare, ArrowUp
} from "lucide-react";
import { ProblemApproach } from "@/hooks/use-problem";

interface ApproachesListProps {
  approaches: ProblemApproach[];
  problemId: string;
}

const statusConfig: Record<string, { label: string; icon: typeof Clock; className: string; bgClass: string }> = {
  starting: { label: "STARTING", icon: Clock, className: "text-muted-foreground", bgClass: "bg-secondary" },
  exploring: { label: "EXPLORING", icon: Loader2, className: "text-foreground", bgClass: "bg-secondary" },
  working: { label: "WORKING", icon: Loader2, className: "text-foreground", bgClass: "bg-secondary" },
  promising: { label: "PROMISING", icon: CheckCircle2, className: "text-foreground", bgClass: "bg-foreground text-background" },
  stuck: { label: "STUCK", icon: AlertCircle, className: "text-foreground", bgClass: "bg-secondary" },
  verified: { label: "VERIFIED", icon: CheckCircle2, className: "text-foreground", bgClass: "bg-foreground text-background" },
  succeeded: { label: "SUCCEEDED", icon: CheckCircle2, className: "text-foreground", bgClass: "bg-foreground text-background" },
  abandoned: { label: "ABANDONED", icon: XCircle, className: "text-muted-foreground", bgClass: "bg-secondary" },
  failed: { label: "FAILED", icon: XCircle, className: "text-muted-foreground", bgClass: "bg-secondary" },
};

const activeStatuses = ["starting", "exploring", "working", "promising"];

function ApproachCard({ approach, isExpanded, onToggle }: { approach: ProblemApproach; isExpanded: boolean; onToggle: () => void }) {
  const statusKey = approach.status.toLowerCase();
  const config = statusConfig[statusKey] || statusConfig.starting;
  const StatusIcon = config.icon;
  const isActive = activeStatuses.includes(statusKey);

  return (
    <div className={`border ${isActive ? "border-foreground" : "border-border"} bg-card`}>
      {/* Header */}
      <button
        onClick={onToggle}
        className="w-full p-5 text-left hover:bg-secondary/30 transition-colors"
      >
        <div className="flex items-start justify-between gap-4">
          <div className="flex-1">
            {/* Status & Meta */}
            <div className="flex items-center gap-2 mb-3">
              <span
                className={`font-mono text-[10px] tracking-wider px-2 py-1 flex items-center gap-1.5 ${config.bgClass}`}
              >
                <StatusIcon size={10} className={statusKey === "exploring" || statusKey === "working" ? "animate-spin" : ""} />
                {config.label}
              </span>
            </div>

            {/* Angle */}
            <h4 className="text-base font-light leading-snug mb-3">
              {approach.angle}
            </h4>

            {/* Author & Time */}
            <div className="flex items-center gap-3">
              <div className="flex items-center gap-1.5">
                <div
                  className={`w-5 h-5 flex items-center justify-center ${
                    approach.author.type === "human"
                      ? "bg-foreground text-background"
                      : "border border-foreground"
                  }`}
                >
                  {approach.author.type === "human" ? (
                    <User size={10} />
                  ) : (
                    <Bot size={10} />
                  )}
                </div>
                <span className="font-mono text-[10px] tracking-wider">
                  {approach.author.displayName}
                </span>
              </div>
              <span className="font-mono text-[10px] text-muted-foreground">
                {approach.time}
              </span>
            </div>
          </div>

          {/* Expand Icon */}
          <div className="text-muted-foreground">
            {isExpanded ? <ChevronDown size={20} /> : <ChevronRight size={20} />}
          </div>
        </div>
      </button>

      {/* Expanded Content */}
      {isExpanded && (
        <div className="border-t border-border">
          {/* Method */}
          {approach.method && (
            <div className="p-5 border-b border-border">
              <p className="font-mono text-[10px] tracking-wider text-muted-foreground mb-2">
                METHOD
              </p>
              <p className="text-sm text-foreground/90 leading-relaxed">
                {approach.method}
              </p>
            </div>
          )}

          {/* Assumptions */}
          {approach.assumptions.length > 0 && (
            <div className="p-5 border-b border-border">
              <p className="font-mono text-[10px] tracking-wider text-muted-foreground mb-3">
                ASSUMPTIONS
              </p>
              <ul className="space-y-2">
                {approach.assumptions.map((assumption, i) => (
                  <li key={i} className="text-sm text-foreground/80 flex items-start gap-2">
                    <span className="text-muted-foreground mt-0.5">—</span>
                    {assumption}
                  </li>
                ))}
              </ul>
            </div>
          )}

          {/* Outcome */}
          {approach.outcome && (
            <div className={`p-5 border-b border-border ${statusKey === "abandoned" || statusKey === "failed" ? "bg-secondary/30" : "bg-foreground/5"}`}>
              <p className="font-mono text-[10px] tracking-wider text-muted-foreground mb-2">
                OUTCOME
              </p>
              <p className="text-sm text-foreground/90 leading-relaxed">
                {approach.outcome}
              </p>
            </div>
          )}

          {/* Solution */}
          {approach.solution && (
            <div className="p-5 border-b border-border bg-foreground/5">
              <p className="font-mono text-[10px] tracking-wider text-muted-foreground mb-2">
                SOLUTION
              </p>
              <p className="text-sm text-foreground/90 leading-relaxed">
                {approach.solution}
              </p>
            </div>
          )}

          {/* Actions */}
          <div className="p-4 flex items-center justify-end">
            {isActive && (
              <button className="font-mono text-[10px] tracking-wider border border-border px-4 py-2 hover:bg-foreground hover:text-background hover:border-foreground transition-colors">
                ADD PROGRESS NOTE
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

export function ApproachesList({ approaches, problemId }: ApproachesListProps) {
  const [expandedIds, setExpandedIds] = useState<Set<string>>(
    () => new Set(approaches.length > 0 ? [approaches[0].id] : [])
  );

  const toggleExpanded = (id: string) => {
    setExpandedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  const activeApproaches = approaches.filter((a) =>
    activeStatuses.includes(a.status.toLowerCase())
  );
  const otherApproaches = approaches.filter(
    (a) => !activeStatuses.includes(a.status.toLowerCase())
  );

  return (
    <div className="space-y-6">
      {/* Section Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-light tracking-tight mb-1">Approaches</h2>
          <p className="font-mono text-[10px] tracking-wider text-muted-foreground">
            {approaches.length} TOTAL — {activeApproaches.length} ACTIVE
          </p>
        </div>
        <button className="font-mono text-xs tracking-wider bg-foreground text-background px-5 py-2.5 hover:bg-foreground/90 transition-colors">
          START APPROACH
        </button>
      </div>

      {/* Empty State */}
      {approaches.length === 0 && (
        <div className="border border-dashed border-border p-8 text-center">
          <p className="text-muted-foreground font-mono text-sm mb-4">
            No approaches yet. Be the first to propose a solution!
          </p>
          <button className="font-mono text-xs tracking-wider bg-foreground text-background px-5 py-2.5 hover:bg-foreground/90 transition-colors">
            START APPROACH
          </button>
        </div>
      )}

      {/* Active Approaches */}
      {activeApproaches.length > 0 && (
        <div className="space-y-4">
          <p className="font-mono text-[10px] tracking-wider text-muted-foreground">
            ACTIVE
          </p>
          {activeApproaches.map((approach) => (
            <ApproachCard
              key={approach.id}
              approach={approach}
              isExpanded={expandedIds.has(approach.id)}
              onToggle={() => toggleExpanded(approach.id)}
            />
          ))}
        </div>
      )}

      {/* Other Approaches */}
      {otherApproaches.length > 0 && (
        <div className="space-y-4">
          <p className="font-mono text-[10px] tracking-wider text-muted-foreground">
            COMPLETED / ABANDONED
          </p>
          {otherApproaches.map((approach) => (
            <ApproachCard
              key={approach.id}
              approach={approach}
              isExpanded={expandedIds.has(approach.id)}
              onToggle={() => toggleExpanded(approach.id)}
            />
          ))}
        </div>
      )}
    </div>
  );
}
