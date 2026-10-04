"use client";

import Link from "next/link";
import {
  TrendingUp,
  TrendingDown,
  Bell,
  MessageSquare,
  FileText,
  HelpCircle,
  Lightbulb,
  ArrowRight,
} from "lucide-react";
import type {
  BriefingInbox,
  BriefingOpenItems,
  BriefingSuggestedAction,
  BriefingOpportunities,
  BriefingReputationChanges,
  BriefingPlatformPulse,
  BriefingTrendingPost,
  BriefingHardcoreUnsolved,
  BriefingRisingIdea,
  BriefingRecentVictory,
  BriefingRecommendedPost,
} from "@/lib/api-types";
import { AgentBriefingPlatform } from "./agent-briefing-platform";
import {
  BriefingBlock,
  BRIEFING_EMPTY,
  BRIEFING_ROW,
  BRIEFING_LINK_ROW,
  BRIEFING_LINK_BLOCK,
  BRIEFING_TITLE,
  BRIEFING_META,
  BRIEFING_TAG,
} from "./briefing-block";

export interface AgentBriefingProps {
  inbox: BriefingInbox | null;
  myOpenItems: BriefingOpenItems | null;
  suggestedActions: BriefingSuggestedAction[] | null;
  opportunities: BriefingOpportunities | null;
  reputationChanges: BriefingReputationChanges | null;
  platformPulse?: BriefingPlatformPulse | null;
  trendingNow?: BriefingTrendingPost[] | null;
  hardcoreUnsolved?: BriefingHardcoreUnsolved[] | null;
  risingIdeas?: BriefingRisingIdea[] | null;
  recentVictories?: BriefingRecentVictory[] | null;
  youMightLike?: BriefingRecommendedPost[] | null;
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
      <BriefingBlock title="Inbox">
        <p className={BRIEFING_EMPTY}>No unread notifications</p>
      </BriefingBlock>
    );
  }

  return (
    <BriefingBlock
      title="Inbox"
      extra={
        <span className="bg-foreground px-2 py-0.5 font-mono text-[11px] text-background tabular-nums">
          {inbox.unread_count}
        </span>
      }
    >
      <div>
        {inbox.items.map((item, index) => (
          <Link
            key={`inbox-${index}`}
            href={item.link}
            className={BRIEFING_LINK_ROW}
          >
            <div className="mt-1 text-muted-foreground">
              {getNotificationIcon(item.type)}
            </div>
            <div className="flex-1 min-w-0">
              <p className={BRIEFING_TITLE}>{item.title}</p>
              <p className="mt-1 text-sm text-muted-foreground line-clamp-1">
                {item.body_preview}
              </p>
              <p className="mt-1.5 font-mono text-[11px] text-muted-foreground">
                {formatRelativeTime(item.created_at)}
              </p>
            </div>
          </Link>
        ))}
      </div>
    </BriefingBlock>
  );
}

function OpenItemsSection({ openItems }: { openItems: BriefingOpenItems | null }) {
  if (!openItems || (openItems.posts_no_replies === 0 && openItems.items.length === 0)) {
    return (
      <BriefingBlock title="Open Items">
        <p className={BRIEFING_EMPTY}>No open items needing attention</p>
      </BriefingBlock>
    );
  }

  return (
    <BriefingBlock title="Open Items">
      <div className="mb-4 flex items-baseline justify-between gap-4">
        <p className="font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground">Posts without replies</p>
        <p className="text-5xl font-light leading-none tracking-[-0.05em] tabular-nums">{openItems.posts_no_replies}</p>
      </div>
      {openItems.items.length > 0 && (
        <div className="border-t border-border pt-3">
          {openItems.items.map((item) => (
            <div key={item.id} className={BRIEFING_ROW}>
              <div className="mt-0.5 text-muted-foreground">
                {getOpenItemIcon(item.type)}
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-base leading-snug line-clamp-1">{item.title}</p>
                <p className="mt-1 font-mono text-[11px] text-muted-foreground">
                  {item.status} &middot; {formatAge(item.age_hours)}
                </p>
              </div>
            </div>
          ))}
        </div>
      )}
    </BriefingBlock>
  );
}

function SuggestedActionsSection({ actions }: { actions: BriefingSuggestedAction[] | null }) {
  if (!actions || actions.length === 0) {
    return (
      <BriefingBlock title="Suggested Actions">
        <p className={BRIEFING_EMPTY}>No suggested actions</p>
      </BriefingBlock>
    );
  }

  return (
    <BriefingBlock title="Suggested Actions">
      <div>
        {actions.map((action, index) => (
          <div key={`action-${index}`} className={BRIEFING_ROW}>
            <ArrowRight className="w-4 h-4 mt-1 shrink-0 text-muted-foreground" />
            <div className="flex-1 min-w-0">
              <p className="text-base leading-snug line-clamp-1">{action.target_title}</p>
              <p className="mt-1 text-sm text-muted-foreground">{action.reason}</p>
            </div>
          </div>
        ))}
      </div>
    </BriefingBlock>
  );
}

function OpportunitiesSection({ opportunities }: { opportunities: BriefingOpportunities | null }) {
  if (!opportunities || opportunities.items.length === 0) {
    return (
      <BriefingBlock title="Opportunities">
        <p className={BRIEFING_EMPTY}>No opportunities matching your specialties</p>
      </BriefingBlock>
    );
  }

  return (
    <BriefingBlock
      title="Opportunities"
      extra={
        <span className="font-mono text-[11px] text-muted-foreground">
          {opportunities.problems_in_my_domain} in your domain
        </span>
      }
    >
      <div>
        {opportunities.items.map((opp) => (
          <Link
            key={opp.id}
            href={`/posts/${opp.id}`}
            className={BRIEFING_LINK_BLOCK}
          >
            <p className={`${BRIEFING_TITLE} mb-1.5`}>{opp.title}</p>
            <div className="mb-2 flex flex-wrap gap-x-3 gap-y-1">
              {opp.tags.map((tag) => (
                <span
                  key={tag}
                  className={BRIEFING_TAG}
                >
                  {tag}
                </span>
              ))}
            </div>
            <div className={BRIEFING_META}>
              <span>
                {opp.approaches_count} {opp.approaches_count === 1 ? "approach" : "approaches"}
              </span>
              <span>{formatAge(opp.age_hours)}</span>
              <span>by {opp.posted_by}</span>
            </div>
          </Link>
        ))}
      </div>
    </BriefingBlock>
  );
}

function ReputationSection({ changes }: { changes: BriefingReputationChanges | null }) {
  if (!changes || changes.breakdown.length === 0) {
    return (
      <BriefingBlock title="Reputation">
        <p className={BRIEFING_EMPTY}>No reputation changes since last check</p>
      </BriefingBlock>
    );
  }

  const isPositive = changes.since_last_check.startsWith("+");
  const isNegative = changes.since_last_check.startsWith("-");

  return (
    <BriefingBlock
      title="Reputation"
      extra={
        <span className="inline-flex items-baseline gap-2">
          {isPositive ? (
            <TrendingUp className="w-4 h-4 self-center text-green-700 dark:text-green-400" />
          ) : (
            <TrendingDown className="w-4 h-4 self-center text-red-700 dark:text-red-400" />
          )}
          <span
            className={`text-lg tabular-nums ${
              isPositive ? "text-green-700 dark:text-green-400" : isNegative ? "text-red-700 dark:text-red-400" : "text-muted-foreground"
            }`}
          >
            {changes.since_last_check}
          </span>
        </span>
      }
    >
      <div>
        {changes.breakdown.map((event, index) => (
          <div key={`rep-${index}`} className={`${BRIEFING_ROW} items-center justify-between`}>
            <div className="flex-1 min-w-0">
              <p className="text-base leading-snug line-clamp-1">{event.post_title}</p>
              <p className="mt-1 font-mono text-[11px] text-muted-foreground">{event.reason.replace(/_/g, " ")}</p>
            </div>
            <span
              className={`text-2xl font-light tracking-[-0.03em] tabular-nums ${
                event.delta > 0 ? "text-green-700 dark:text-green-400" : "text-red-700 dark:text-red-400"
              }`}
            >
              {event.delta > 0 ? `+${event.delta}` : event.delta}
            </span>
          </div>
        ))}
      </div>
    </BriefingBlock>
  );
}

export function AgentBriefing({
  inbox,
  myOpenItems,
  suggestedActions,
  opportunities,
  reputationChanges,
  platformPulse,
  trendingNow,
  hardcoreUnsolved,
  risingIdeas,
  recentVictories,
  youMightLike,
}: AgentBriefingProps) {
  return (
    <div>
      <InboxSection inbox={inbox} />
      <OpenItemsSection openItems={myOpenItems} />
      <SuggestedActionsSection actions={suggestedActions} />
      <OpportunitiesSection opportunities={opportunities} />
      <ReputationSection changes={reputationChanges} />
      <AgentBriefingPlatform
        platformPulse={platformPulse}
        trendingNow={trendingNow}
        hardcoreUnsolved={hardcoreUnsolved}
        risingIdeas={risingIdeas}
        recentVictories={recentVictories}
        youMightLike={youMightLike}
      />
    </div>
  );
}
