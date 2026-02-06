"use client";

import { useState } from "react";
import {
  Bot, User, ChevronDown, ChevronRight, Clock, CheckCircle2,
  XCircle, Loader2, AlertCircle, Plus, X
} from "lucide-react";
import { ProblemApproach } from "@/hooks/use-problem";
import { useApproachForm } from "@/hooks/use-approach-form";

interface ApproachesListProps {
  approaches: ProblemApproach[];
  problemId: string;
  onApproachPosted?: () => void;
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

// Calculate duration between two dates
function formatDuration(start: string, end: string): string {
  const startDate = new Date(start);
  const endDate = new Date(end);
  const diffMs = endDate.getTime() - startDate.getTime();

  const days = Math.floor(diffMs / (1000 * 60 * 60 * 24));
  const hours = Math.floor((diffMs % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60));

  if (days > 0) return `${days}D ${hours}H`;
  if (hours > 0) return `${hours}H`;
  return 'LESS THAN 1H';
}

// Format relative time for display
function formatRelativeTime(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMins / 60);
  const diffDays = Math.floor(diffHours / 24);

  if (diffMins < 1) return 'JUST NOW';
  if (diffMins < 60) return `${diffMins}M AGO`;
  if (diffHours < 24) return `${diffHours}H AGO`;
  if (diffDays < 7) return `${diffDays}D AGO`;
  return date.toLocaleDateString().toUpperCase();
}

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

          {/* Progress Notes */}
          {approach.progressNotes && approach.progressNotes.length > 0 && (
            <div className="p-5 border-b border-border">
              <p className="font-mono text-[10px] tracking-wider text-muted-foreground mb-3">
                PROGRESS NOTES
              </p>
              <div className="space-y-4">
                {approach.progressNotes.map((note) => (
                  <div key={note.id} className="border-l-2 border-border pl-4">
                    <div className="prose prose-sm prose-invert max-w-none text-foreground/90 leading-relaxed whitespace-pre-wrap">
                      {note.content}
                    </div>
                    <p className="font-mono text-[10px] text-muted-foreground mt-2">
                      {note.time}
                    </p>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Actions / Metadata Footer */}
          <div className="p-4 flex items-center justify-between">
            {isActive ? (
              <button className="font-mono text-[10px] tracking-wider border border-border px-4 py-2 hover:bg-foreground hover:text-background hover:border-foreground transition-colors ml-auto">
                ADD PROGRESS NOTE
              </button>
            ) : (
              <>
                <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
                  TOOK {formatDuration(approach.createdAt, approach.updatedAt)}
                </span>
                <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
                  UPDATED {formatRelativeTime(approach.updatedAt)}
                </span>
              </>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

export function ApproachesList({ approaches, problemId, onApproachPosted }: ApproachesListProps) {
  const [expandedIds, setExpandedIds] = useState<Set<string>>(
    () => new Set(approaches.length > 0 ? [approaches[0].id] : [])
  );
  const [showForm, setShowForm] = useState(false);
  const [assumptionInput, setAssumptionInput] = useState('');

  const form = useApproachForm(problemId, () => {
    setShowForm(false);
    setAssumptionInput('');
    onApproachPosted?.();
  });

  const addAssumption = () => {
    if (assumptionInput.trim()) {
      form.setAssumptions([...form.assumptions, assumptionInput.trim()]);
      setAssumptionInput('');
    }
  };

  const removeAssumption = (index: number) => {
    form.setAssumptions(form.assumptions.filter((_, i) => i !== index));
  };

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
        <button
          onClick={() => setShowForm(true)}
          className="font-mono text-xs tracking-wider bg-foreground text-background px-5 py-2.5 hover:bg-foreground/90 transition-colors flex items-center gap-2"
        >
          <Plus size={14} />
          START APPROACH
        </button>
      </div>

      {/* Approach Form */}
      {showForm && (
        <div className="border border-foreground bg-card p-5 space-y-4">
          <div className="flex items-center justify-between">
            <h3 className="font-mono text-xs tracking-wider">NEW APPROACH</h3>
            <button
              onClick={() => setShowForm(false)}
              className="text-muted-foreground hover:text-foreground"
            >
              <X size={16} />
            </button>
          </div>

          {form.error && (
            <div className="bg-destructive/10 border border-destructive text-destructive px-4 py-2 text-sm">
              {form.error}
            </div>
          )}

          <div>
            <label className="font-mono text-[10px] tracking-wider text-muted-foreground block mb-2">
              ANGLE *
            </label>
            <input
              type="text"
              value={form.angle}
              onChange={(e) => form.setAngle(e.target.value)}
              placeholder="What's your approach angle?"
              className="w-full border border-border bg-background px-4 py-3 text-sm focus:outline-none focus:border-foreground"
            />
          </div>

          <div>
            <label className="font-mono text-[10px] tracking-wider text-muted-foreground block mb-2">
              METHOD
            </label>
            <textarea
              value={form.method}
              onChange={(e) => form.setMethod(e.target.value)}
              placeholder="How will you tackle this?"
              rows={3}
              className="w-full border border-border bg-background px-4 py-3 text-sm focus:outline-none focus:border-foreground resize-none"
            />
          </div>

          <div>
            <label className="font-mono text-[10px] tracking-wider text-muted-foreground block mb-2">
              ASSUMPTIONS
            </label>
            <div className="flex gap-2 mb-2">
              <input
                type="text"
                value={assumptionInput}
                onChange={(e) => setAssumptionInput(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && (e.preventDefault(), addAssumption())}
                placeholder="Add an assumption"
                className="flex-1 border border-border bg-background px-4 py-2 text-sm focus:outline-none focus:border-foreground"
              />
              <button
                type="button"
                onClick={addAssumption}
                className="font-mono text-[10px] tracking-wider border border-border px-4 py-2 hover:bg-secondary transition-colors"
              >
                ADD
              </button>
            </div>
            {form.assumptions.length > 0 && (
              <ul className="space-y-1">
                {form.assumptions.map((assumption, i) => (
                  <li key={i} className="flex items-center gap-2 text-sm text-foreground/80">
                    <span className="text-muted-foreground">—</span>
                    <span className="flex-1">{assumption}</span>
                    <button
                      onClick={() => removeAssumption(i)}
                      className="text-muted-foreground hover:text-foreground"
                    >
                      <X size={12} />
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>

          <div className="flex justify-end gap-3 pt-2">
            <button
              onClick={() => setShowForm(false)}
              className="font-mono text-xs tracking-wider border border-border px-5 py-2.5 hover:bg-secondary transition-colors"
            >
              CANCEL
            </button>
            <button
              onClick={form.submit}
              disabled={form.isSubmitting}
              className="font-mono text-xs tracking-wider bg-foreground text-background px-5 py-2.5 hover:bg-foreground/90 transition-colors disabled:opacity-50 flex items-center gap-2"
            >
              {form.isSubmitting && <Loader2 size={14} className="animate-spin" />}
              {form.isSubmitting ? 'SUBMITTING...' : 'START APPROACH'}
            </button>
          </div>
        </div>
      )}

      {/* Empty State */}
      {approaches.length === 0 && !showForm && (
        <div className="border border-dashed border-border p-8 text-center">
          <p className="text-muted-foreground font-mono text-sm mb-4">
            No approaches yet. Be the first to propose a solution!
          </p>
          <button
            onClick={() => setShowForm(true)}
            className="font-mono text-xs tracking-wider bg-foreground text-background px-5 py-2.5 hover:bg-foreground/90 transition-colors"
          >
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
