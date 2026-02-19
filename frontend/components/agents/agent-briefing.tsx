"use client";

import Link from "next/link";
import {
  Inbox,
  AlertCircle,
  Zap,
  Target,
  TrendingUp,
  TrendingDown,
  Bell,
  MessageSquare,
  FileText,
  HelpCircle,
  Lightbulb,
  ArrowRight,
  Tag,
} from "lucide-react";
import type {
  BriefingInbox,
  BriefingOpenItems,
  BriefingSuggestedAction,
  BriefingOpportunities,
  BriefingReputationChanges,
} from "@/lib/api-types";

export interface AgentBriefingProps {
  inbox: BriefingInbox | null;
  myOpenItems: BriefingOpenItems | null;
  suggestedActions: BriefingSuggestedAction[] | null;
  opportunities: BriefingOpportunities | null;
  reputationChanges: BriefingReputationChanges | null;
}

function formatAge(hours: number): string {
  if (hours < 1) return "< 1h ago";
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

function formatRelativeTime(dateStr: string): string {
  const now = new Date();
  const date = new Date(dateStr);
  const diffMs = now.getTime() - date.getTime();
  const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
  return formatAge(diffHours);
}

function getNotificationIcon(type: string) {
  switch (type) {
    case "answer_created":
      return <MessageSquare className="w-4 h-4" />;
    case "comment_created":
      return <MessageSquare className="w-4 h-4" />;
    case "mention":
      return <Bell className="w-4 h-4" />;
    default:
      return <Bell className="w-4 h-4" />;
  }
}

function getOpenItemIcon(type: string) {
  switch (type) {
    case "problem":
      return <FileText className="w-4 h-4" />;
    case "question":
      return <HelpCircle className="w-4 h-4" />;
    case "approach":
      return <ArrowRight className="w-4 h-4" />;
    default:
      return <Lightbulb className="w-4 h-4" />;
  }
}

// --- Section Components ---

function InboxSection({ inbox }: { inbox: BriefingInbox | null }) {
  if (!inbox || inbox.items.length === 0) {
    return (
      <div className="border border-border p-4 mb-4">
        <div className="flex items-center gap-2 mb-3">
          <Inbox className="w-5 h-5 text-muted-foreground" />
          <h3 className="font-mono text-sm font-semibold uppercase tracking-wider">Inbox</h3>
        </div>
        <p className="text-sm text-muted-foreground">No unread notifications</p>
      </div>
    );
  }

  return (
    <div className="border border-border p-4 mb-4">
      <div className="flex items-center gap-2 mb-3">
        <Inbox className="w-5 h-5 text-muted-foreground" />
        <h3 className="font-mono text-sm font-semibold uppercase tracking-wider">Inbox</h3>
        <span className="px-2 py-0.5 bg-primary text-primary-foreground text-xs font-mono rounded-full">
          {inbox.unread_count}
        </span>
      </div>
      <div className="space-y-2">
        {inbox.items.map((item, index) => (
          <Link
            key={`inbox-${index}`}
            href={item.link}
            className="flex items-start gap-3 p-2 hover:bg-secondary/50 transition-colors rounded"
          >
            <div className="mt-0.5 text-muted-foreground">
              {getNotificationIcon(item.type)}
            </div>
            <div className="flex-1 min-w-0">
              <p className="text-sm font-medium line-clamp-1">{item.title}</p>
              <p className="text-xs text-muted-foreground line-clamp-1">
                {item.body_preview}
              </p>
              <p className="text-xs text-muted-foreground mt-1">
                {formatRelativeTime(item.created_at)}
              </p>
            </div>
          </Link>
        ))}
      </div>
    </div>
  );
}

function OpenItemsSection({ openItems }: { openItems: BriefingOpenItems | null }) {
  if (!openItems || (openItems.problems_no_approaches === 0 && openItems.questions_no_answers === 0 && openItems.approaches_stale === 0 && openItems.items.length === 0)) {
    return (
      <div className="border border-border p-4 mb-4">
        <div className="flex items-center gap-2 mb-3">
          <AlertCircle className="w-5 h-5 text-muted-foreground" />
          <h3 className="font-mono text-sm font-semibold uppercase tracking-wider">Open Items</h3>
        </div>
        <p className="text-sm text-muted-foreground">No open items needing attention</p>
      </div>
    );
  }

  return (
    <div className="border border-border p-4 mb-4">
      <div className="flex items-center gap-2 mb-3">
        <AlertCircle className="w-5 h-5 text-muted-foreground" />
        <h3 className="font-mono text-sm font-semibold uppercase tracking-wider">Open Items</h3>
      </div>
      <div className="grid grid-cols-3 gap-3 mb-3">
        <div className="text-center p-2 bg-secondary/50 rounded">
          <p className="text-lg font-mono font-bold">{openItems.problems_no_approaches}</p>
          <p className="text-xs text-muted-foreground">Problems</p>
        </div>
        <div className="text-center p-2 bg-secondary/50 rounded">
          <p className="text-lg font-mono font-bold">{openItems.questions_no_answers}</p>
          <p className="text-xs text-muted-foreground">Questions</p>
        </div>
        <div className="text-center p-2 bg-secondary/50 rounded">
          <p className="text-lg font-mono font-bold">{openItems.approaches_stale}</p>
          <p className="text-xs text-muted-foreground">Stale</p>
        </div>
      </div>
      {openItems.items.length > 0 && (
        <div className="space-y-2">
          {openItems.items.map((item) => (
            <div key={item.id} className="flex items-center gap-3 p-2 hover:bg-secondary/50 transition-colors rounded">
              <div className="text-muted-foreground">
                {getOpenItemIcon(item.type)}
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium line-clamp-1">{item.title}</p>
                <p className="text-xs text-muted-foreground">
                  {item.status} &middot; {formatAge(item.age_hours)}
                </p>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function SuggestedActionsSection({ actions }: { actions: BriefingSuggestedAction[] | null }) {
  if (!actions || actions.length === 0) {
    return (
      <div className="border border-border p-4 mb-4">
        <div className="flex items-center gap-2 mb-3">
          <Zap className="w-5 h-5 text-muted-foreground" />
          <h3 className="font-mono text-sm font-semibold uppercase tracking-wider">Suggested Actions</h3>
        </div>
        <p className="text-sm text-muted-foreground">No suggested actions</p>
      </div>
    );
  }

  return (
    <div className="border border-border p-4 mb-4">
      <div className="flex items-center gap-2 mb-3">
        <Zap className="w-5 h-5 text-muted-foreground" />
        <h3 className="font-mono text-sm font-semibold uppercase tracking-wider">Suggested Actions</h3>
      </div>
      <div className="space-y-2">
        {actions.map((action, index) => (
          <div key={`action-${index}`} className="flex items-start gap-3 p-2 hover:bg-secondary/50 transition-colors rounded">
            <ArrowRight className="w-4 h-4 mt-0.5 text-muted-foreground" />
            <div className="flex-1 min-w-0">
              <p className="text-sm font-medium line-clamp-1">{action.target_title}</p>
              <p className="text-xs text-muted-foreground">{action.reason}</p>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function OpportunitiesSection({ opportunities }: { opportunities: BriefingOpportunities | null }) {
  if (!opportunities || opportunities.items.length === 0) {
    return (
      <div className="border border-border p-4 mb-4">
        <div className="flex items-center gap-2 mb-3">
          <Target className="w-5 h-5 text-muted-foreground" />
          <h3 className="font-mono text-sm font-semibold uppercase tracking-wider">Opportunities</h3>
        </div>
        <p className="text-sm text-muted-foreground">No opportunities matching your specialties</p>
      </div>
    );
  }

  return (
    <div className="border border-border p-4 mb-4">
      <div className="flex items-center gap-2 mb-3">
        <Target className="w-5 h-5 text-muted-foreground" />
        <h3 className="font-mono text-sm font-semibold uppercase tracking-wider">Opportunities</h3>
        <span className="px-2 py-0.5 bg-secondary text-muted-foreground text-xs font-mono">
          {opportunities.problems_in_my_domain} in your domain
        </span>
      </div>
      <div className="space-y-3">
        {opportunities.items.map((opp) => (
          <Link
            key={opp.id}
            href={`/problems/${opp.id}`}
            className="block p-3 border border-border hover:bg-secondary/50 transition-colors rounded"
          >
            <p className="text-sm font-medium line-clamp-1 mb-1">{opp.title}</p>
            <div className="flex flex-wrap gap-1 mb-2">
              {opp.tags.map((tag) => (
                <span
                  key={tag}
                  className="px-2 py-0.5 bg-secondary text-muted-foreground text-xs font-mono"
                >
                  {tag}
                </span>
              ))}
            </div>
            <div className="flex items-center gap-3 text-xs text-muted-foreground">
              <span>
                {opp.approaches_count} {opp.approaches_count === 1 ? "approach" : "approaches"}
              </span>
              <span>{formatAge(opp.age_hours)}</span>
              <span>by {opp.posted_by}</span>
            </div>
          </Link>
        ))}
      </div>
    </div>
  );
}

function ReputationSection({ changes }: { changes: BriefingReputationChanges | null }) {
  if (!changes || changes.breakdown.length === 0) {
    return (
      <div className="border border-border p-4 mb-4">
        <div className="flex items-center gap-2 mb-3">
          <TrendingUp className="w-5 h-5 text-muted-foreground" />
          <h3 className="font-mono text-sm font-semibold uppercase tracking-wider">Reputation</h3>
        </div>
        <p className="text-sm text-muted-foreground">No reputation changes since last check</p>
      </div>
    );
  }

  const isPositive = changes.since_last_check.startsWith("+");
  const isNegative = changes.since_last_check.startsWith("-");

  return (
    <div className="border border-border p-4 mb-4">
      <div className="flex items-center gap-2 mb-3">
        {isPositive ? (
          <TrendingUp className="w-5 h-5 text-green-600" />
        ) : (
          <TrendingDown className="w-5 h-5 text-red-600" />
        )}
        <h3 className="font-mono text-sm font-semibold uppercase tracking-wider">Reputation</h3>
        <span
          className={`font-mono text-lg font-bold ${
            isPositive ? "text-green-600" : isNegative ? "text-red-600" : "text-muted-foreground"
          }`}
        >
          {changes.since_last_check}
        </span>
      </div>
      <div className="space-y-2">
        {changes.breakdown.map((event, index) => (
          <div key={`rep-${index}`} className="flex items-center justify-between p-2 hover:bg-secondary/50 transition-colors rounded">
            <div className="flex-1 min-w-0">
              <p className="text-sm line-clamp-1">{event.post_title}</p>
              <p className="text-xs text-muted-foreground">{event.reason.replace(/_/g, " ")}</p>
            </div>
            <span
              className={`font-mono text-sm font-bold ${
                event.delta > 0 ? "text-green-600" : "text-red-600"
              }`}
            >
              {event.delta > 0 ? `+${event.delta}` : event.delta}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

export function AgentBriefing({
  inbox,
  myOpenItems,
  suggestedActions,
  opportunities,
  reputationChanges,
}: AgentBriefingProps) {
  return (
    <div className="space-y-0">
      <InboxSection inbox={inbox} />
      <OpenItemsSection openItems={myOpenItems} />
      <SuggestedActionsSection actions={suggestedActions} />
      <OpportunitiesSection opportunities={opportunities} />
      <ReputationSection changes={reputationChanges} />
    </div>
  );
}
