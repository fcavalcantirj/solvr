"use client";

import Link from 'next/link';
import { Loader2, MessageSquare, ArrowRight } from 'lucide-react';
import { useContributions } from '@/hooks/use-contributions';
import { cn } from '@/lib/utils';

function getBadgeStyle(legacyType: string | null): string {
  switch (legacyType) {
    case 'answer': return 'bg-emerald-500/10 text-emerald-500';
    case 'approach': return 'bg-blue-500/10 text-blue-500';
    case 'response': return 'bg-purple-500/10 text-purple-500';
    default: return 'bg-muted text-muted-foreground';
  }
}

interface ContributionsListProps {
  userId: string;
}

// The user's replies (GET /v1/replies), newest first; each links to the reply on its post.
export function ContributionsList({ userId }: ContributionsListProps) {
  const { contributions, loading, error, hasMore, loadMore } = useContributions(userId);

  return (
    <div>
      {/* Loading state */}
      {loading && contributions.length === 0 && (
        <div className="border border-dashed border-border p-12 text-center">
          <Loader2 size={24} className="animate-spin mx-auto mb-3 text-muted-foreground" />
          <p className="font-mono text-sm text-muted-foreground">Loading contributions...</p>
        </div>
      )}

      {/* Error state */}
      {error && (
        <div className="border border-destructive/50 bg-destructive/5 p-8 text-center">
          <p className="font-mono text-sm text-destructive">{error}</p>
        </div>
      )}

      {/* Empty state */}
      {!loading && !error && contributions.length === 0 && (
        <div className="border border-dashed border-border p-12 text-center">
          <MessageSquare size={32} className="mx-auto mb-4 text-muted-foreground" />
          <p className="font-mono text-sm text-muted-foreground">No contributions yet</p>
        </div>
      )}

      {/* Contributions list */}
      {contributions.length > 0 && (
        <div className="space-y-3">
          {contributions.map((contribution) => (
            <Link
              key={contribution.id}
              href={`/posts/${contribution.postId}#${contribution.id}`}
              className="block border border-border p-4 hover:bg-secondary/50 transition-colors group"
            >
              <div className="flex items-start gap-3">
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-1.5">
                    <span className={cn(
                      "inline-block px-2 py-0.5 font-mono text-[10px] tracking-wider",
                      getBadgeStyle(contribution.legacyType)
                    )}>
                      {(contribution.legacyType ?? 'reply').toUpperCase()}
                    </span>
                    <span className="font-mono text-[10px] text-muted-foreground ml-auto">
                      {contribution.timestamp}
                    </span>
                  </div>

                  <p className="font-mono text-xs text-muted-foreground mb-1">Replied to:</p>
                  <h3 className="font-mono text-sm font-medium truncate group-hover:text-foreground">
                    {contribution.postTitle}
                  </h3>

                  <p className="font-mono text-xs text-muted-foreground mt-2 line-clamp-2">
                    {contribution.body}
                  </p>
                </div>

                <ArrowRight size={14} className="text-muted-foreground mt-1 flex-shrink-0 opacity-0 group-hover:opacity-100 transition-opacity" />
              </div>
            </Link>
          ))}
        </div>
      )}

      {/* Load more button */}
      {hasMore && (
        <div className="mt-6 text-center">
          <button
            onClick={loadMore}
            disabled={loading}
            className="font-mono text-xs tracking-wider bg-foreground text-background px-6 py-2.5 hover:bg-foreground/90 transition-colors disabled:opacity-50"
          >
            {loading ? 'LOADING...' : 'LOAD MORE'}
          </button>
        </div>
      )}
    </div>
  );
}
