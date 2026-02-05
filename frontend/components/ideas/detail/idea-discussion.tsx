"use client";

import { useState } from "react";
import { MessageSquare, ThumbsUp, Reply, ChevronDown, ChevronUp, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { useResponseForm } from "@/hooks/use-response-form";
import { useIdeaResponses, IdeaResponseData } from "@/hooks/use-idea-responses";

interface IdeaDiscussionProps {
  ideaId: string;
  onResponsePosted?: () => void;
}

function getAuthorInitials(name: string): string {
  return name.slice(0, 2).toUpperCase();
}

function ResponseItem({ response }: { response: IdeaResponseData }) {
  return (
    <div className="border-b border-border pb-6 last:border-0 last:pb-0">
      <div className="flex items-start gap-4">
        <div
          className={cn(
            "w-8 h-8 flex items-center justify-center font-mono text-xs font-bold flex-shrink-0",
            response.author.type === "ai"
              ? "bg-gradient-to-br from-cyan-400 to-blue-500 text-white"
              : "bg-foreground text-background"
          )}
        >
          {response.author.type === "ai" ? "AI" : getAuthorInitials(response.author.displayName)}
        </div>

        <div className="flex-1">
          <div className="flex items-center gap-2 mb-2">
            <span className="font-mono text-sm font-medium">{response.author.displayName}</span>
            <span className="font-mono text-[10px] text-muted-foreground">
              {response.author.type === "ai" ? "[AI]" : "[HUMAN]"}
            </span>
            <span className="font-mono text-[10px] text-muted-foreground">
              {response.time}
            </span>
          </div>

          <div className="text-sm text-foreground whitespace-pre-wrap leading-relaxed">
            {response.content}
          </div>

          <div className="flex items-center gap-4 mt-3">
            <button className="flex items-center gap-1.5 font-mono text-xs text-muted-foreground hover:text-emerald-600 transition-colors">
              <ThumbsUp className="w-3 h-3" />
              {response.voteScore}
            </button>
            <button className="flex items-center gap-1.5 font-mono text-xs text-muted-foreground hover:text-foreground transition-colors">
              <Reply className="w-3 h-3" />
              REPLY
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

export function IdeaDiscussion({ ideaId, onResponsePosted }: IdeaDiscussionProps) {
  const { responses, loading, error, total, hasMore, loadMore, refetch } = useIdeaResponses(ideaId);
  const form = useResponseForm(ideaId, () => {
    refetch();
    onResponsePosted?.();
  });

  if (loading && responses.length === 0) {
    return (
      <div className="bg-card border border-border p-6">
        <div className="flex items-center justify-center py-12">
          <Loader2 className="w-5 h-5 animate-spin text-muted-foreground" />
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-card border border-border p-6">
        <p className="font-mono text-xs text-destructive">{error}</p>
      </div>
    );
  }

  return (
    <div className="bg-card border border-border p-6">
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-2">
          <MessageSquare className="w-4 h-4 text-muted-foreground" />
          <h2 className="font-mono text-sm tracking-wider text-muted-foreground">
            DISCUSSION ({total})
          </h2>
        </div>
        <select className="bg-transparent border border-border px-3 py-1.5 font-mono text-xs focus:outline-none focus:border-foreground">
          <option>NEWEST</option>
          <option>MOST LIKED</option>
          <option>OLDEST</option>
        </select>
      </div>

      <div className="space-y-6">
        {responses.length === 0 ? (
          <p className="font-mono text-xs text-muted-foreground text-center py-8">
            No responses yet. Be the first to share your thoughts!
          </p>
        ) : (
          responses.map((response) => (
            <ResponseItem key={response.id} response={response} />
          ))
        )}
      </div>

      {hasMore && (
        <div className="mt-6 text-center">
          <Button
            variant="outline"
            size="sm"
            onClick={loadMore}
            disabled={loading}
            className="font-mono text-xs"
          >
            {loading ? (
              <>
                <Loader2 className="w-3 h-3 mr-2 animate-spin" />
                Loading...
              </>
            ) : (
              'Load more responses'
            )}
          </Button>
        </div>
      )}

      {/* Add Comment */}
      <div className="mt-6 pt-6 border-t border-border">
        {form.error && (
          <div className="bg-destructive/10 border border-destructive text-destructive px-4 py-2 text-sm mb-3">
            {form.error}
          </div>
        )}
        <textarea
          value={form.content}
          onChange={(e) => form.setContent(e.target.value)}
          placeholder="Share your thoughts, build on this idea..."
          disabled={form.isSubmitting}
          className="w-full h-24 bg-secondary/50 border border-border p-4 font-mono text-sm resize-none focus:outline-none focus:border-foreground placeholder:text-muted-foreground disabled:opacity-50"
        />
        <div className="flex items-center justify-between mt-3">
          <span className="font-mono text-[10px] text-muted-foreground">
            MARKDOWN SUPPORTED
          </span>
          <Button
            onClick={form.submit}
            disabled={form.isSubmitting}
            className="font-mono text-xs tracking-wider"
          >
            {form.isSubmitting && <Loader2 className="w-3 h-3 mr-2 animate-spin" />}
            {form.isSubmitting ? 'POSTING...' : 'POST COMMENT'}
          </Button>
        </div>
      </div>
    </div>
  );
}
