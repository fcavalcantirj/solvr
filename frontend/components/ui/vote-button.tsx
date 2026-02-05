"use client";

import { ArrowUp, ArrowDown } from "lucide-react";
import { useVote } from "@/hooks/use-vote";
import { cn } from "@/lib/utils";

interface VoteButtonProps {
  postId: string;
  initialScore: number;
  direction?: 'vertical' | 'horizontal';
  size?: 'sm' | 'md' | 'lg';
  showDownvote?: boolean;
  className?: string;
}

export function VoteButton({
  postId,
  initialScore,
  direction = 'vertical',
  size = 'md',
  showDownvote = false,
  className,
}: VoteButtonProps) {
  const { score, isVoting, error, upvote, downvote } = useVote(postId, initialScore);

  const sizeConfig = {
    sm: { button: 'w-6 h-6', icon: 12, text: 'text-xs' },
    md: { button: 'w-10 h-10', icon: 16, text: 'text-sm' },
    lg: { button: 'w-12 h-12', icon: 20, text: 'text-lg' },
  };

  const handleUpvote = (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    upvote();
  };

  const handleDownvote = (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    downvote();
  };

  return (
    <div
      className={cn(
        'flex items-center gap-1',
        direction === 'vertical' ? 'flex-col' : 'flex-row',
        className
      )}
    >
      <button
        onClick={handleUpvote}
        disabled={isVoting}
        className={cn(
          sizeConfig[size].button,
          'flex items-center justify-center border border-border hover:bg-foreground hover:text-background hover:border-foreground transition-colors disabled:opacity-50',
          isVoting && 'cursor-wait'
        )}
        title={error || 'Upvote'}
      >
        <ArrowUp size={sizeConfig[size].icon} />
      </button>
      <span className={cn('font-mono font-medium', sizeConfig[size].text)}>
        {score}
      </span>
      {showDownvote && (
        <button
          onClick={handleDownvote}
          disabled={isVoting}
          className={cn(
            sizeConfig[size].button,
            'flex items-center justify-center border border-border hover:bg-foreground hover:text-background hover:border-foreground transition-colors disabled:opacity-50',
            isVoting && 'cursor-wait'
          )}
          title={error || 'Downvote'}
        >
          <ArrowDown size={sizeConfig[size].icon} />
        </button>
      )}
    </div>
  );
}
