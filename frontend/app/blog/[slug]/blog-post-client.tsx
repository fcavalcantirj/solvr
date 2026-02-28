"use client";

import { useState, useEffect, useCallback } from "react";
import { api } from "@/lib/api";
import { useAuth } from "@/hooks/use-auth";
import {
  ThumbsUp,
  ThumbsDown,
  Share2,
  Eye,
} from "lucide-react";
import { cn } from "@/lib/utils";

interface BlogPostClientProps {
  slug: string;
  initialVoteScore: number;
  initialUserVote: "up" | "down" | null;
  viewCount: number;
}

export function BlogPostClient({
  slug,
  initialVoteScore,
  initialUserVote,
  viewCount,
}: BlogPostClientProps) {
  const { isAuthenticated, setShowAuthModal } = useAuth();
  const [voteScore, setVoteScore] = useState(initialVoteScore);
  const [userVote, setUserVote] = useState<"up" | "down" | null>(initialUserVote);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    api.recordBlogView(slug).catch(() => {});
  }, [slug]);

  const handleVote = useCallback(
    async (direction: "up" | "down") => {
      if (!isAuthenticated) {
        setShowAuthModal(true);
        return;
      }
      try {
        const response = await api.voteBlogPost(slug, direction);
        if (response?.data) {
          setVoteScore(response.data.vote_score);
          setUserVote(response.data.user_vote);
        }
      } catch {
        // Vote failed silently
      }
    },
    [slug, isAuthenticated, setShowAuthModal]
  );

  const handleShare = useCallback(() => {
    const url = window.location.href;
    navigator.clipboard.writeText(url).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }).catch(() => {});
  }, []);

  return (
    <div className="flex items-center gap-4 sm:gap-6">
      {/* Vote buttons */}
      <div className="flex items-center gap-2">
        <button
          data-testid="vote-up"
          onClick={() => handleVote("up")}
          className={cn(
            "p-2 transition-colors",
            userVote === "up"
              ? "text-green-400"
              : "text-muted-foreground hover:text-foreground"
          )}
          aria-label="Vote up"
        >
          <ThumbsUp size={16} />
        </button>
        <span
          data-testid="vote-score"
          className="font-mono text-sm tabular-nums"
        >
          {voteScore}
        </span>
        <button
          data-testid="vote-down"
          onClick={() => handleVote("down")}
          className={cn(
            "p-2 transition-colors",
            userVote === "down"
              ? "text-red-400"
              : "text-muted-foreground hover:text-foreground"
          )}
          aria-label="Vote down"
        >
          <ThumbsDown size={16} />
        </button>
      </div>

      {/* View count */}
      <div className="flex items-center gap-1.5 text-muted-foreground">
        <Eye size={14} />
        <span className="font-mono text-xs">{viewCount}</span>
      </div>

      {/* Share button */}
      <button
        data-testid="share-button"
        onClick={handleShare}
        className="flex items-center gap-1.5 p-2 text-muted-foreground hover:text-foreground transition-colors"
        aria-label="Share"
      >
        <Share2 size={14} />
        <span className="font-mono text-xs">
          {copied ? "Copied!" : "Share"}
        </span>
      </button>
    </div>
  );
}
