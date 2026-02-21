"use client";

import { useState } from "react";
import Link from "next/link";
import { ArrowLeft, Share2, Bookmark, Bot, User, Check, Pencil } from "lucide-react";
import { VoteButton } from "@/components/ui/vote-button";
import { ReportModal } from "@/components/ui/report-modal";
import { ModerationBanner } from "@/components/shared/moderation-banner";
import { useShare } from "@/hooks/use-share";
import { useBookmarks } from "@/hooks/use-bookmarks";
import { useAuth } from "@/hooks/use-auth";
import { IdeaData } from "@/hooks/use-idea";

interface IdeaHeaderProps {
  idea: IdeaData;
}

function getIdeaStatusColor(status: string) {
  switch (status.toLowerCase()) {
    case 'under review':
      return 'bg-yellow-500/10 text-yellow-600 border-yellow-500/20';
    case 'rejected':
      return 'bg-red-500/10 text-red-600 border-red-500/20';
    default:
      return 'bg-blue-500/10 text-blue-600 border-blue-500/20';
  }
}

export function IdeaHeader({ idea }: IdeaHeaderProps) {
  const { share, shared } = useShare();
  const { bookmarkedPosts, toggleBookmark } = useBookmarks();
  const isBookmarked = bookmarkedPosts.has(idea.id);
  const [showReportModal, setShowReportModal] = useState(false);
  const { user } = useAuth();
  const isAuthor = user?.id === idea.author.id;

  return (
    <div>
      <Link
        href="/ideas"
        className="inline-flex items-center gap-2 font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors mb-6"
      >
        <ArrowLeft className="w-3 h-3" />
        BACK TO IDEAS
      </Link>

      {/* Moderation Banner */}
      <ModerationBanner status={idea.status} postId={idea.id} postType="ideas" />

      <div className="flex items-start justify-between gap-6">
        <div className="flex-1">
          <div className="flex items-center gap-3 mb-4">
            <span className={`px-2 py-1 font-mono text-[10px] tracking-wider border ${getIdeaStatusColor(idea.status)}`}>
              {idea.status}
            </span>
            <span className="font-mono text-xs text-muted-foreground">
              {idea.id.slice(0, 8)}
            </span>
          </div>

          <h1 className="font-mono text-2xl md:text-3xl font-medium tracking-tight text-foreground leading-tight text-balance">
            {idea.title}
          </h1>

          <div className="flex items-center gap-4 mt-4 text-muted-foreground">
            <div className="flex items-center gap-2">
              <div className={`w-6 h-6 flex items-center justify-center ${
                idea.author.type === 'human'
                  ? 'bg-foreground text-background'
                  : 'bg-gradient-to-br from-cyan-400 to-blue-500 text-white'
              }`}>
                {idea.author.type === 'human' ? <User className="w-3 h-3" /> : <Bot className="w-3 h-3" />}
              </div>
              <span className="font-mono text-xs">{idea.author.displayName}</span>
              <span className="font-mono text-[10px] text-muted-foreground">
                [{idea.author.type === 'human' ? 'HUMAN' : 'AI'}]
              </span>
            </div>
            <span className="font-mono text-xs">sparked {idea.time}</span>
          </div>
        </div>

        <div className="flex items-center gap-2">
          {isAuthor && (
            <Link
              href={`/ideas/${idea.id}/edit`}
              data-testid="edit-post-button"
              className="p-2 border border-border text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
            >
              <Pencil size={16} />
            </Link>
          )}
          <VoteButton
            postId={idea.id}
            initialScore={idea.voteScore}
            direction="horizontal"
            size="md"
            showDownvote
          />

          <button
            data-testid="bookmark-button"
            onClick={() => toggleBookmark(idea.id)}
            className={`p-2 border border-border hover:bg-secondary transition-colors ${
              isBookmarked ? "text-foreground" : "text-muted-foreground hover:text-foreground"
            }`}
          >
            <Bookmark size={16} fill={isBookmarked ? "currentColor" : "none"} />
          </button>
          <button
            data-testid="share-button"
            onClick={() => share(idea.title, `${window.location.origin}/ideas/${idea.id}`)}
            className={`p-2 border border-border hover:bg-secondary transition-colors ${
              shared ? "text-emerald-500" : "text-muted-foreground hover:text-foreground"
            }`}
          >
            {shared ? <Check size={16} /> : <Share2 size={16} />}
          </button>
        </div>
      </div>

      <ReportModal
        isOpen={showReportModal}
        onClose={() => setShowReportModal(false)}
        targetType="post"
        targetId={idea.id}
        targetLabel="idea"
      />
    </div>
  );
}
