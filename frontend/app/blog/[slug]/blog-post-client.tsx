"use client";

import { useState, useEffect, useCallback } from "react";
import { api } from "@/lib/api";
import { track } from "@/lib/analytics";
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
  const { isAuthenticated, showAuthWall } = useAuth();
  const [voteScore, setVoteScore] = useState(initialVoteScore);
  const [userVote, setUserVote] = useState<"up" | "down" | null>(initialUserVote);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    api.recordBlogView(slug).catch(() => {});
  }, [slug]);

  const handleVote = useCallback(
    async (direction: "up" | "down") => {
      if (!isAuthenticated) {
        showAuthWall("blog_vote");
        return;
      }
      try {
        const response = await api.voteBlogPost(slug, direction);
        if (response?.data) {
          // The answer carries the post's counts after the vote (SPEC.md 27.1); when the API
          // could not read the post back it carries none, and the page keeps what it shows.
          const { vote_score, user_vote } = response.data;
          if (typeof vote_score === "number") setVoteScore(vote_score);
          if (user_vote !== undefined) setUserVote(user_vote);
          // The API counted the vote (SPEC.md 27.7).
          track("blog_vote", { direction });
        }
      } catch {
        // Vote failed silently
      }
    },
    [slug, isAuthenticated, showAuthWall]
  );

  const handleShare = useCallback(() => {
    const url = window.location.href;
    navigator.clipboard.writeText(url).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
      // The link is on the clipboard (SPEC.md 27.7). The link itself is not sent.
      track("blog_share", { method: "clipboard" });
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
