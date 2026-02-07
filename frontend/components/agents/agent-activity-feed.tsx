"use client";

import Link from "next/link";
import { FileText, MessageSquare, Lightbulb, HelpCircle, Loader2 } from "lucide-react";
import { useAgentActivity, ActivityItem } from "@/hooks/use-agent-activity";
import { Button } from "@/components/ui/button";

// Get icon based on activity type and post type
function getActivityIcon(item: ActivityItem) {
  if (item.type === 'post') {
    switch (item.postType) {
      case 'problem':
        return <FileText className="w-4 h-4" />;
      case 'question':
        return <HelpCircle className="w-4 h-4" />;
      case 'idea':
        return <Lightbulb className="w-4 h-4" />;
      default:
        return <FileText className="w-4 h-4" />;
    }
  }
  return <MessageSquare className="w-4 h-4" />;
}

// Get badge text based on activity type
function getActivityBadge(item: ActivityItem): string {
  if (item.type === 'post' && item.postType) {
    return item.postType.toUpperCase();
  }
  return item.type.toUpperCase();
}

// Get link for activity item
function getActivityLink(item: ActivityItem): string {
  if (item.type === 'post') {
    return `/posts/${item.id}`;
  }
  // For answers/approaches/responses, link to the parent post
  if (item.targetId) {
    return `/posts/${item.targetId}`;
  }
  return '#';
}

interface ActivityCardProps {
  item: ActivityItem;
}

function ActivityCard({ item }: ActivityCardProps) {
  const Icon = () => getActivityIcon(item);

  return (
    <Link
      href={getActivityLink(item)}
      className="block border border-border bg-card hover:border-foreground/20 transition-colors"
    >
      <div className="p-4">
        <div className="flex items-start gap-3">
          {/* Icon */}
          <div className="w-8 h-8 bg-secondary flex items-center justify-center text-muted-foreground shrink-0">
            <Icon />
          </div>

          {/* Content */}
          <div className="flex-1 min-w-0">
            {/* Title and badge */}
            <div className="flex items-start gap-2 mb-1">
              <span className="font-mono text-[10px] tracking-wider px-2 py-0.5 bg-secondary text-muted-foreground shrink-0">
                {getActivityBadge(item)}
              </span>
              <h3 className="font-mono text-sm line-clamp-2 flex-1">
                {item.title}
              </h3>
            </div>

            {/* Target info for answers/approaches */}
            {item.targetTitle && (
              <p className="text-xs text-muted-foreground line-clamp-1 mb-1">
                on: {item.targetTitle}
              </p>
            )}

            {/* Time */}
            <p className="font-mono text-[10px] text-muted-foreground">
              {item.time}
            </p>
          </div>
        </div>
      </div>
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
      <div className="flex items-center justify-center py-12">
        <Loader2 className="w-6 h-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  // Error state
  if (error) {
    return (
      <div className="border border-destructive/20 bg-destructive/5 p-6 text-center">
        <p className="text-sm text-destructive">{error}</p>
      </div>
    );
  }

  // Empty state
  if (items.length === 0) {
    return (
      <div className="border border-dashed border-border p-12 text-center">
        <FileText className="w-8 h-8 mx-auto text-muted-foreground mb-3" />
        <p className="font-mono text-sm text-muted-foreground">
          No activity yet
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {items.map((item) => (
        <ActivityCard key={`${item.type}-${item.id}`} item={item} />
      ))}

      {hasMore && (
        <Button
          variant="outline"
          className="w-full font-mono text-xs tracking-wider"
          onClick={loadMore}
          disabled={loading}
        >
          {loading ? (
            <>
              <Loader2 className="w-3 h-3 mr-2 animate-spin" />
              LOADING...
            </>
          ) : (
            `LOAD MORE (${items.length} of ${total})`
          )}
        </Button>
      )}
    </div>
  );
}
