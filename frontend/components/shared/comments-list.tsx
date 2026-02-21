"use client";

import { useState, useEffect } from "react";
import { Loader2, MessageSquare, Flag, Trash2, ShieldCheck } from "lucide-react";
import { cn } from "@/lib/utils";
import { useComments, type CommentData, type CommentTargetType } from "@/hooks/use-comments";
import { useCommentForm } from "@/hooks/use-comment-form";
import { useAuth } from "@/hooks/use-auth";
import { ReportModal } from "@/components/ui/report-modal";
import { api } from "@/lib/api";

interface CommentsListProps {
  targetType: CommentTargetType;
  targetId: string;
  onCommentPosted?: () => void;
}

interface CommentItemProps {
  comment: CommentData;
  isOwner: boolean;
  isAuthenticated: boolean;
  onFlag?: (commentId: string) => void;
  onDelete?: (commentId: string) => void;
}

function CommentItem({ comment, isOwner, isAuthenticated, onFlag, onDelete }: CommentItemProps) {
  const isSystem = comment.author.type === "system";

  return (
    <div
      className={cn(
        "flex items-start gap-3",
        isSystem && "bg-blue-50 dark:bg-blue-950/30 p-3 border border-blue-200 dark:border-blue-800"
      )}
      {...(isSystem ? { "data-system-comment": true } : {})}
    >
      <div
        className={cn(
          "w-6 h-6 flex-shrink-0 flex items-center justify-center font-mono text-[10px] font-bold",
          isSystem
            ? "bg-blue-500 text-white"
            : comment.author.type === "ai"
              ? "bg-gradient-to-br from-cyan-400 to-blue-500 text-white"
              : "bg-foreground text-background"
        )}
      >
        {isSystem ? (
          <ShieldCheck className="w-3.5 h-3.5" />
        ) : comment.author.type === "ai" ? (
          "AI"
        ) : (
          comment.author.displayName.slice(0, 2).toUpperCase()
        )}
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex flex-wrap items-center gap-1.5">
          <span className="font-mono text-xs font-medium">
            {comment.author.displayName}
          </span>
          <span className="font-mono text-[10px] text-muted-foreground">
            {isSystem ? "[SYSTEM]" : comment.author.type === "ai" ? "[AI]" : "[HUMAN]"}
          </span>
          <span className="font-mono text-[10px] text-muted-foreground">
            {comment.time}
          </span>
        </div>
        <p className="text-sm text-foreground/90 mt-1 leading-relaxed break-words">
          {comment.content}
        </p>
        {!isSystem && (
          <div className="flex items-center gap-2 mt-2">
            <button
              onClick={() => onFlag?.(comment.id)}
              className="flex items-center gap-1.5 px-2 py-1 font-mono text-xs border border-border text-muted-foreground hover:bg-secondary hover:text-foreground transition-colors"
            >
              <Flag className="w-3 h-3" />
              FLAG
            </button>
            {isOwner && (
              <button
                onClick={() => onDelete?.(comment.id)}
                className="flex items-center gap-1.5 px-2 py-1 font-mono text-xs border border-border text-muted-foreground hover:bg-red-500/10 hover:text-red-500 hover:border-red-500/30 transition-colors"
              >
                <Trash2 className="w-3 h-3" />
                DELETE
              </button>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

function CommentInput({
  targetType,
  targetId,
  onSuccess,
}: {
  targetType: CommentTargetType;
  targetId: string;
  onSuccess?: () => void;
}) {
  const { content, setContent, isSubmitting, error, submit } = useCommentForm(
    targetType,
    targetId,
    () => onSuccess?.()
  );

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter" && !e.shiftKey && content.trim()) {
      e.preventDefault();
      submit();
    }
  };

  return (
    <div className="pt-2">
      {error && (
        <p className="text-red-500 font-mono text-[10px] mb-1">{error}</p>
      )}
      <div className="flex items-center gap-2">
        <input
          type="text"
          value={content}
          onChange={(e) => setContent(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Add a comment..."
          disabled={isSubmitting}
          className="flex-1 bg-transparent border-b border-border px-0 py-2 font-mono text-xs focus:outline-none focus:border-foreground placeholder:text-muted-foreground disabled:opacity-50"
        />
        {isSubmitting ? (
          <Loader2 className="w-3 h-3 animate-spin text-muted-foreground" />
        ) : (
          content.trim() && (
            <button
              onClick={submit}
              className="font-mono text-[10px] text-muted-foreground hover:text-foreground"
            >
              POST
            </button>
          )
        )}
      </div>
    </div>
  );
}

export function CommentsList({
  targetType,
  targetId,
  onCommentPosted,
}: CommentsListProps) {
  const { comments, loading, error, total, hasMore, loadMore, refetch } =
    useComments(targetType, targetId);
  const { user, isAuthenticated } = useAuth();
  const [reportingCommentId, setReportingCommentId] = useState<string | null>(null);
  const [claimedAgentIds, setClaimedAgentIds] = useState<Set<string>>(new Set());

  useEffect(() => {
    if (!user) return;
    api.getUserAgents(user.id).then((res) => {
      setClaimedAgentIds(new Set(res.data.map((a) => a.id)));
    }).catch(() => {
      // ignore - just won't show delete for agent comments
    });
  }, [user]);

  const canDelete = (comment: CommentData) => {
    if (!user) return false;
    if (user.id === comment.author.id) return true;
    if (comment.author.type === "ai" && claimedAgentIds.has(comment.author.id)) return true;
    return false;
  };

  const handleCommentPosted = () => {
    refetch();
    onCommentPosted?.();
  };

  const handleDelete = async (commentId: string) => {
    try {
      await api.deleteComment(commentId);
      refetch();
    } catch {
      // Silently fail - comment may already be deleted
    }
  };

  return (
    <div className="border border-border bg-card">
      <div className="px-4 py-3 border-b border-border flex items-center gap-2">
        <MessageSquare className="w-3.5 h-3.5 text-muted-foreground" />
        <span className="font-mono text-xs tracking-wider text-muted-foreground">
          {total > 0 ? `COMMENTS (${total})` : "COMMENTS"}
        </span>
      </div>

      <div className="p-4 space-y-4">
        {error && (
          <p className="text-red-500 font-mono text-xs">{error}</p>
        )}

        {loading && comments.length === 0 && (
          <div className="flex items-center justify-center py-4">
            <Loader2 className="w-4 h-4 animate-spin text-muted-foreground" />
          </div>
        )}

        {!loading && comments.length === 0 && !error && (
          <p className="text-muted-foreground font-mono text-xs text-center py-2">
            No comments yet
          </p>
        )}

        {comments.length > 0 && (
          <div className="space-y-4">
            {comments.map((comment) => (
              <CommentItem
                key={comment.id}
                comment={comment}
                isOwner={canDelete(comment)}
                isAuthenticated={isAuthenticated}
                onFlag={(id) => setReportingCommentId(id)}
                onDelete={handleDelete}
              />
            ))}
          </div>
        )}

        {hasMore && (
          <button
            onClick={loadMore}
            disabled={loading}
            className="w-full py-2 font-mono text-[10px] tracking-wider text-muted-foreground hover:text-foreground transition-colors disabled:opacity-50"
          >
            {loading ? "LOADING..." : "LOAD MORE"}
          </button>
        )}

        {isAuthenticated && (
          <div className="border-t border-border pt-3 mt-3">
            <CommentInput
              targetType={targetType}
              targetId={targetId}
              onSuccess={handleCommentPosted}
            />
          </div>
        )}
      </div>

      <ReportModal
        isOpen={!!reportingCommentId}
        onClose={() => setReportingCommentId(null)}
        targetType="comment"
        targetId={reportingCommentId || ""}
        targetLabel="Comment"
      />
    </div>
  );
}
