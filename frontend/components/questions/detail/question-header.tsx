"use client";

import { useState } from "react";
import Link from "next/link";
import { ArrowLeft, Share2, Bookmark, MoreHorizontal, Check } from "lucide-react";
import { Button } from "@/components/ui/button";
import { VoteButton } from "@/components/ui/vote-button";
import { QuestionData } from "@/hooks/use-question";

interface QuestionHeaderProps {
  question: QuestionData;
}

export function QuestionHeader({ question }: QuestionHeaderProps) {
  const [copied, setCopied] = useState(false);
  const [bookmarked, setBookmarked] = useState(false);

  const handleShare = async () => {
    try {
      await navigator.clipboard.writeText(window.location.href);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Fallback: do nothing silently
    }
  };

  const handleBookmark = () => {
    setBookmarked(!bookmarked);
  };
  // Determine status badge color based on status
  const getStatusColor = (status: string) => {
    switch (status.toLowerCase()) {
      case 'answered':
        return 'bg-emerald-500/10 text-emerald-600 border-emerald-500/20';
      case 'open':
        return 'bg-blue-500/10 text-blue-600 border-blue-500/20';
      case 'closed':
        return 'bg-gray-500/10 text-gray-600 border-gray-500/20';
      default:
        return 'bg-gray-500/10 text-gray-600 border-gray-500/20';
    }
  };

  return (
    <div>
      <Link
        href="/questions"
        className="inline-flex items-center gap-2 font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors mb-6"
      >
        <ArrowLeft className="w-3 h-3" />
        BACK TO QUESTIONS
      </Link>

      <div className="flex items-start justify-between gap-6">
        <div className="flex-1">
          <div className="flex items-center gap-3 mb-4">
            <span className="px-2 py-1 bg-amber-500/10 text-amber-600 font-mono text-[10px] tracking-wider border border-amber-500/20">
              QUESTION
            </span>
            <span className={`px-2 py-1 font-mono text-[10px] tracking-wider border ${getStatusColor(question.status)}`}>
              {question.status}
            </span>
            <span className="font-mono text-xs text-muted-foreground">
              Q-{question.id.slice(0, 8)}
            </span>
          </div>

          <h1 className="font-mono text-2xl md:text-3xl font-medium tracking-tight text-foreground leading-tight text-balance">
            {question.title}
          </h1>

          <div className="flex items-center gap-4 mt-4 text-muted-foreground">
            <div className="flex items-center gap-2">
              <div className={`w-6 h-6 flex items-center justify-center ${
                question.author.type === 'ai'
                  ? 'bg-gradient-to-br from-cyan-400 to-blue-500'
                  : 'bg-gradient-to-br from-purple-400 to-pink-500'
              }`}>
                <span className="text-[10px] font-mono text-white font-bold">
                  {question.author.type === 'ai' ? 'AI' : 'H'}
                </span>
              </div>
              <span className="font-mono text-xs">{question.author.displayName}</span>
            </div>
            <span className="font-mono text-xs">asked {question.time}</span>
            {question.answersCount > 0 && (
              <span className="font-mono text-xs">{question.answersCount} answers</span>
            )}
          </div>
        </div>

        <div className="flex items-center gap-2">
          <VoteButton
            postId={question.id}
            initialScore={question.voteScore}
            direction="horizontal"
            size="md"
            showDownvote
          />
          <Button variant="outline" size="sm" className="font-mono text-xs bg-transparent" onClick={handleShare}>
            {copied ? (
              <>
                <Check className="w-3 h-3 mr-2 text-emerald-600" />
                COPIED
              </>
            ) : (
              <>
                <Share2 className="w-3 h-3 mr-2" />
                SHARE
              </>
            )}
          </Button>
          <Button
            variant="outline"
            size="sm"
            className={`font-mono text-xs bg-transparent ${bookmarked ? 'border-foreground' : ''}`}
            onClick={handleBookmark}
          >
            <Bookmark className={`w-3 h-3 mr-2 ${bookmarked ? 'fill-current' : ''}`} />
            {bookmarked ? 'SAVED' : 'SAVE'}
          </Button>
          <Button variant="ghost" size="icon" className="h-8 w-8">
            <MoreHorizontal className="w-4 h-4" />
          </Button>
        </div>
      </div>
    </div>
  );
}
