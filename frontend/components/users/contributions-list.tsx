"use client";

import Link from 'next/link';
import { Loader2, MessageSquare, ArrowUpRight } from 'lucide-react';
import { useContributions } from '@/hooks/use-contributions';
import { CAPTION } from '@/components/page/caption';
import { LINE_BUTTON } from '@/components/page/controls';
import { cn } from '@/lib/utils';

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
        <div className="flex items-center gap-3 border-t border-border py-12">
          <Loader2 size={18} className="animate-spin text-muted-foreground" />
          <p className={CAPTION}>Loading contributions...</p>
        </div>
      )}

      {/* Error state */}
      {error && (
        <div className="border-t border-border py-12">
          <p className="text-sm text-destructive">{error}</p>
        </div>
      )}

      {/* Empty state */}
      {!loading && !error && contributions.length === 0 && (
        <div className="border-t border-border py-16">
          <MessageSquare size={32} strokeWidth={1} className="mb-6 text-muted-foreground" />
          <p className="text-3xl font-light tracking-[-0.025em]">No contributions yet</p>
        </div>
      )}

      {/* Contributions list */}
      {contributions.length > 0 && (
        <div className="border-t border-border">
          {contributions.map((contribution) => (
            <Link
              key={contribution.id}
              href={`/posts/${contribution.postId}#${contribution.id}`}
              className="group grid min-w-0 grid-cols-[minmax(0,1fr)_auto] gap-x-6 border-b border-border py-6 focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-4 focus-visible:outline-foreground"
            >
              <div className="min-w-0">
                <div className="mb-3 flex flex-wrap items-center gap-x-5 gap-y-1">
                  <span className={cn(CAPTION, "text-foreground")}>
                    {(contribution.legacyType ?? 'reply').toUpperCase()}
                  </span>
                  <span className={CAPTION}>
                    {contribution.timestamp}
                  </span>
                </div>

                <p className="text-sm text-muted-foreground">Replied to:</p>
                <h3 className="mt-1 truncate text-xl font-light tracking-[-0.02em] underline decoration-transparent decoration-1 underline-offset-[5px] transition-colors group-hover:decoration-current sm:text-2xl">
                  {contribution.postTitle}
                </h3>

                <p className="mt-3 max-w-[70ch] text-sm leading-relaxed text-muted-foreground line-clamp-2">
                  {contribution.body}
                </p>
              </div>

              <ArrowUpRight aria-hidden="true" strokeWidth={1} className="mt-1 size-5 shrink-0 text-muted-foreground transition-colors group-hover:text-foreground" />
            </Link>
          ))}
        </div>
      )}

      {/* Load more button */}
      {hasMore && (
        <button
          type="button"
          onClick={loadMore}
          disabled={loading}
          className={cn(LINE_BUTTON, "mt-10 w-full py-5")}
        >
          {loading ? 'LOADING...' : 'LOAD MORE'}
        </button>
      )}
    </div>
  );
}
