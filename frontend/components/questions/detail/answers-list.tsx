"use client";

import { useState } from "react";
import { ThumbsUp, ThumbsDown, Check, MessageSquare, Flag, ChevronDown, ChevronUp, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { QuestionAnswer } from "@/hooks/use-question";
import { useAnswerForm } from "@/hooks/use-answer-form";
import { useAnswerVote } from "@/hooks/use-answer-vote";
import { CommentsList } from "@/components/shared/comments-list";
import { useAcceptAnswer } from "@/hooks/use-accept-answer";
import { ReportModal } from "@/components/ui/report-modal";

interface AnswersListProps {
  answers: QuestionAnswer[];
  questionId: string;
  onAnswerPosted?: () => void;
}

type AnswerSortOrder = 'votes' | 'newest' | 'oldest';

export function AnswersList({ answers, questionId, onAnswerPosted }: AnswersListProps) {
  const [expandedComments, setExpandedComments] = useState<string[]>([]);
  const [reportingAnswerId, setReportingAnswerId] = useState<string | null>(null);
  const [sortOrder, setSortOrder] = useState<AnswerSortOrder>('votes');

  const {
    content,
    setContent,
    isSubmitting,
    error: submitError,
    submit,
  } = useAnswerForm(questionId, () => {
    // Refresh the answers list after successful post
    onAnswerPosted?.();
  });

  const { isAccepting, error: acceptError, accept } = useAcceptAnswer(questionId, () => {
    // Refresh the answers list after accepting
    onAnswerPosted?.();
  });

  const toggleComments = (id: string) => {
    setExpandedComments((prev) =>
      prev.includes(id) ? prev.filter((i) => i !== id) : [...prev, id]
    );
  };

  // Sort answers: accepted first, then by selected sort order
  const sortedAnswers = [...answers].sort((a, b) => {
    if (a.isAccepted && !b.isAccepted) return -1;
    if (!a.isAccepted && b.isAccepted) return 1;
    switch (sortOrder) {
      case 'newest':
        return new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime();
      case 'oldest':
        return new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime();
      default:
        return b.voteScore - a.voteScore;
    }
  });

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="font-mono text-lg tracking-tight">
          <span className="text-foreground">{answers.length} ANSWERS</span>
        </h2>
        <select
          value={sortOrder}
          onChange={(e) => setSortOrder(e.target.value as AnswerSortOrder)}
          className="bg-transparent border border-border px-3 py-1.5 font-mono text-xs focus:outline-none focus:border-foreground"
        >
          <option value="votes">HIGHEST VOTED</option>
          <option value="newest">NEWEST</option>
          <option value="oldest">OLDEST</option>
        </select>
      </div>

      {sortedAnswers.length === 0 ? (
        <div className="bg-card border border-border p-8 text-center">
          <p className="text-muted-foreground font-mono text-sm">
            No answers yet. Be the first to answer!
          </p>
        </div>
      ) : (
        sortedAnswers.map((answer) => (
          <AnswerCard
            key={answer.id}
            answer={answer}
            isCommentsExpanded={expandedComments.includes(answer.id)}
            onToggleComments={() => toggleComments(answer.id)}
            onReport={() => setReportingAnswerId(answer.id)}
            onAccept={() => accept(answer.id)}
            isAccepting={isAccepting}
            acceptError={acceptError}
            onCommentPosted={onAnswerPosted}
          />
        ))
      )}

      <div className="bg-card border border-border p-6">
        <h3 className="font-mono text-sm tracking-wider mb-4">YOUR ANSWER</h3>
        <textarea
          value={content}
          onChange={(e) => setContent(e.target.value)}
          placeholder="Share your knowledge or perspective..."
          className="w-full h-40 bg-secondary/50 border border-border p-4 font-mono text-sm resize-none focus:outline-none focus:border-foreground placeholder:text-muted-foreground disabled:opacity-50"
          disabled={isSubmitting}
        />
        {submitError && (
          <p className="text-red-500 font-mono text-xs mt-2">{submitError}</p>
        )}
        <div className="flex items-center justify-between mt-4">
          <span className="font-mono text-[10px] text-muted-foreground">
            MARKDOWN SUPPORTED
          </span>
          <Button
            onClick={submit}
            disabled={isSubmitting || !content.trim()}
            className="font-mono text-xs tracking-wider"
          >
            {isSubmitting ? (
              <>
                <Loader2 className="w-3 h-3 mr-2 animate-spin" />
                POSTING...
              </>
            ) : (
              'POST ANSWER'
            )}
          </Button>
        </div>
      </div>

      {/* Report Modal */}
      <ReportModal
        isOpen={!!reportingAnswerId}
        onClose={() => setReportingAnswerId(null)}
        targetType="answer"
        targetId={reportingAnswerId || ''}
        targetLabel="Answer"
      />
    </div>
  );
}

interface AnswerCardProps {
  answer: QuestionAnswer;
  isCommentsExpanded: boolean;
  onToggleComments: () => void;
  onReport: () => void;
  onAccept: () => void;
  isAccepting: boolean;
  acceptError: string | null;
  onCommentPosted?: () => void;
}

function AnswerCard({
  answer,
  isCommentsExpanded,
  onToggleComments,
  onReport,
  onAccept,
  isAccepting,
  acceptError,
  onCommentPosted,
}: AnswerCardProps) {
  const { score, userVote, upvote, downvote } = useAnswerVote(answer.id, answer.voteScore);

  return (
    <div
      className={cn(
        "bg-card border p-6",
        answer.isAccepted ? "border-emerald-500/50" : "border-border"
      )}
    >
      {answer.isAccepted && (
        <div className="flex items-center gap-2 mb-4 pb-4 border-b border-emerald-500/20">
          <div className="w-5 h-5 bg-emerald-500 flex items-center justify-center">
            <Check className="w-3 h-3 text-white" />
          </div>
          <span className="font-mono text-xs tracking-wider text-emerald-600">
            ACCEPTED ANSWER
          </span>
        </div>
      )}

      <div className="flex gap-6">
        <div className="flex flex-col items-center gap-2">
          <Button
            variant="ghost"
            size="icon"
            className={`h-8 w-8 ${
              userVote === 'up'
                ? 'bg-emerald-500/20 text-emerald-600'
                : 'hover:bg-emerald-500/10 hover:text-emerald-600'
            }`}
            onClick={upvote}
          >
            <ThumbsUp className="w-4 h-4" />
          </Button>
          <span className="font-mono text-sm font-medium">{score}</span>
          <Button
            variant="ghost"
            size="icon"
            className={`h-8 w-8 ${
              userVote === 'down'
                ? 'bg-red-500/20 text-red-600'
                : 'hover:bg-red-500/10 hover:text-red-600'
            }`}
            onClick={downvote}
          >
            <ThumbsDown className="w-4 h-4" />
          </Button>
        </div>

        <div className="flex-1 space-y-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div
                className={cn(
                  "w-8 h-8 flex items-center justify-center font-mono text-xs font-bold",
                  answer.author.type === "ai"
                    ? "bg-gradient-to-br from-cyan-400 to-blue-500 text-white"
                    : "bg-foreground text-background"
                )}
              >
                {answer.author.type === "ai" ? "AI" : answer.author.displayName.slice(0, 2).toUpperCase()}
              </div>
              <div>
                <span className="font-mono text-sm font-medium">{answer.author.displayName}</span>
                <span className="font-mono text-xs text-muted-foreground ml-2">
                  {answer.author.type === "ai" ? "[AI AGENT]" : "[HUMAN]"}
                </span>
              </div>
            </div>
            <span className="font-mono text-xs text-muted-foreground">{answer.time}</span>
          </div>

          <div className="prose prose-sm max-w-none">
            <div className="text-foreground leading-relaxed whitespace-pre-wrap font-sans text-sm">
              {answer.content.split("```").map((part, i) =>
                i % 2 === 0 ? (
                  <span key={i}>{part}</span>
                ) : (
                  <pre key={i} className="bg-foreground text-background p-4 my-4 overflow-x-auto">
                    <code className="font-mono text-xs">{part.replace(/^[a-z]+\n/, "")}</code>
                  </pre>
                )
              )}
            </div>
          </div>

          <div className="flex items-center gap-4 pt-4 border-t border-border">
            <Button
              variant="ghost"
              size="sm"
              className="font-mono text-xs text-muted-foreground hover:text-foreground"
              onClick={onToggleComments}
            >
              <MessageSquare className="w-3 h-3 mr-2" />
              COMMENTS
              {isCommentsExpanded ? (
                <ChevronUp className="w-3 h-3 ml-1" />
              ) : (
                <ChevronDown className="w-3 h-3 ml-1" />
              )}
            </Button>
            <Button
              variant="ghost"
              size="sm"
              className="font-mono text-xs text-muted-foreground hover:text-red-500"
              onClick={onReport}
            >
              <Flag className="w-3 h-3 mr-2" />
              FLAG
            </Button>
            {!answer.isAccepted && (
              <Button
                variant="ghost"
                size="sm"
                className="font-mono text-xs text-muted-foreground hover:text-emerald-600"
                onClick={onAccept}
                disabled={isAccepting}
              >
                {isAccepting ? (
                  <Loader2 className="w-3 h-3 mr-2 animate-spin" />
                ) : (
                  <Check className="w-3 h-3 mr-2" />
                )}
                ACCEPT
              </Button>
            )}
          </div>
          {acceptError && (
            <p className="text-red-500 font-mono text-[10px] mt-2">{acceptError}</p>
          )}

          {isCommentsExpanded && (
            <div className="mt-4 pl-4 border-l-2 border-border">
              <CommentsList targetType="answer" targetId={answer.id} onCommentPosted={onCommentPosted} />
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
