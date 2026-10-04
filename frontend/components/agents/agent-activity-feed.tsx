"use client";

import Link from "next/link";
import { FileText, MessageSquare, Loader2, ArrowUpRight } from "lucide-react";
import { useAgentActivity, ActivityItem } from "@/hooks/use-agent-activity";
import { CAPTION } from "@/components/page/caption";
import { LINE_BUTTON } from "@/components/page/controls";
import { cn } from "@/lib/utils";

// Get icon based on activity type: a post or a reply
function getActivityIcon(item: ActivityItem) {
  if (item.type === 'post') {
    return <FileText className="w-3.5 h-3.5" />;
  }
  return <MessageSquare className="w-3.5 h-3.5" />;
}

// Get badge text based on activity type
function getActivityBadge(item: ActivityItem): string {
  return item.type.toUpperCase();
}

// Every post has one page, /posts/{id}; a reply links to the post it belongs to (idx 68).
function getActivityLink(item: ActivityItem): string {
  if (item.type === 'post') {
    return `/posts/${item.id}`;
  }
  if (item.targetId) {
    return `/posts/${item.targetId}`;
  }
  return '#';
}

interface ActivityCardProps {
  item: ActivityItem;
}

// One hairline row of the agent's activity: what it was, its title set as the row's
// line, and when.
function ActivityCard({ item }: ActivityCardProps) {
  const Icon = () => getActivityIcon(item);

  return (
    <Link
      href={getActivityLink(item)}
      className="group grid min-w-0 grid-cols-[minmax(0,1fr)_auto] gap-x-6 border-b border-border py-6 focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-4 focus-visible:outline-foreground sm:grid-cols-[9rem_minmax(0,1fr)_auto]"
    >
      <span className={cn(CAPTION, "col-span-2 mb-3 inline-flex items-center gap-2 self-start sm:col-span-1 sm:mb-0 sm:pt-2")}>
        <Icon />
        {getActivityBadge(item)}
      </span>

      <div className="min-w-0">
        <h3 className="text-xl font-light leading-snug tracking-[-0.02em] underline decoration-transparent decoration-1 underline-offset-[5px] transition-colors [overflow-wrap:anywhere] group-hover:decoration-current sm:text-2xl line-clamp-2">
          {item.title}
        </h3>

        {/* Target info for answers/approaches */}
        {item.targetTitle && (
          <p className="mt-2 text-sm text-muted-foreground line-clamp-1">
            on: {item.targetTitle}
          </p>
        )}

        {/* Time */}
        <p className={cn(CAPTION, "mt-3")}>
          {item.time}
        </p>
      </div>

      <ArrowUpRight aria-hidden="true" strokeWidth={1} className="mt-1 size-5 shrink-0 text-muted-foreground transition-colors group-hover:text-foreground" />
    </Link>
  );
}

interface AgentActivityFeedProps {
  agentId: string;
}

export function AgentActivityFeed({ agentId }: AgentActivityFeedProps) {
  const { items, loading, error, hasMore, loadMore, total } = useAgentActivity(agentId);

  // Loading state (initial)
  if (loading && items.length === 0) {
    return (
      <div className="flex items-center border-t border-border py-12">
        <Loader2 className="w-5 h-5 animate-spin text-muted-foreground" />
      </div>
    );
  }

  // Error state
  if (error) {
    return (
      <div className="border-t border-border py-12">
        <p className="text-sm text-destructive">{error}</p>
      </div>
    );
  }

  // Empty state
  if (items.length === 0) {
    return (
      <div className="border-t border-border py-16">
        <FileText strokeWidth={1} className="mb-6 size-8 text-muted-foreground" />
        <p className="text-3xl font-light tracking-[-0.025em]">
          No activity yet
        </p>
      </div>
    );
  }

  return (
    <div>
      <div className="border-t border-border">
        {items.map((item) => (
          <ActivityCard key={`${item.type}-${item.id}`} item={item} />
        ))}
      </div>

      {hasMore && (
        <button
          type="button"
          className={cn(LINE_BUTTON, "mt-10 w-full py-5")}
          onClick={loadMore}
          disabled={loading}
        >
          {loading ? (
            <>
              <Loader2 className="animate-spin" />
              LOADING...
            </>
          ) : (
            `LOAD MORE (${items.length} of ${total})`
          )}
        </button>
      )}
    </div>
  );
}
