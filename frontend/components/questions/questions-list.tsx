"use client";

import Link from "next/link";
import { Bot, User, MessageSquare, Check, Clock, Loader2 } from "lucide-react";
import { useQuestions, QuestionListItem, UseQuestionsOptions } from "@/hooks/use-questions";
import { useSearch } from "@/hooks/use-posts";
import { VoteButton } from "@/components/ui/vote-button";

const statusConfig: Record<string, { label: string; icon: typeof Clock; className: string }> = {
  open: { label: "AWAITING", icon: Clock, className: "text-muted-foreground" },
  answered: { label: "ANSWERED", icon: MessageSquare, className: "text-foreground" },
  solved: { label: "ACCEPTED", icon: Check, className: "text-foreground font-medium" },
};

interface QuestionsListProps {
  status?: string;
  tags?: string[];
  sort?: 'newest' | 'votes' | 'answers';
  searchQuery?: string;
}

export function QuestionsList({ status, tags, sort, searchQuery }: QuestionsListProps) {
  // Use search when there's a query, otherwise use regular questions fetch
  const isSearching = Boolean(searchQuery?.trim());

  const options: UseQuestionsOptions = { status, tags, sort };
  const questionsResult = useQuestions(options);
  const searchResult = useSearch(searchQuery || '', 'question');

  // Select appropriate result based on whether we're searching
  const { questions, loading, error, hasMore, loadMore } = isSearching
    ? {
        questions: searchResult.posts.map(post => ({
          id: post.id,
          title: post.title,
          snippet: post.snippet,
          status: post.status,
          tags: post.tags,
          voteScore: post.votes,
          answersCount: post.responses,
          viewCount: post.views,
          author: post.author,
          timestamp: post.time,
        })),
        loading: searchResult.loading,
        error: searchResult.error,
        hasMore: false,
        loadMore: () => {},
      }
    : questionsResult;

  if (loading && questions.length === 0) {
    return (
      <div className="flex items-center justify-center py-20">
        <Loader2 className="animate-spin text-muted-foreground" size={24} />
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center justify-center py-20">
        <p className="font-mono text-sm text-muted-foreground">
          Failed to load questions. Please try again.
        </p>
      </div>
    );
  }

  if (questions.length === 0) {
    return (
      <div className="flex items-center justify-center py-20">
        <p className="font-mono text-sm text-muted-foreground">
          No questions found.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {questions.map((question) => (
        <QuestionCard key={question.id} question={question} />
      ))}

      {/* Load More */}
      {hasMore && (
        <div className="flex justify-center pt-4">
          <button
            onClick={loadMore}
            disabled={loading}
            className="font-mono text-xs tracking-wider border border-border px-8 py-3 hover:bg-foreground hover:text-background hover:border-foreground transition-colors disabled:opacity-50"
          >
            {loading ? "LOADING..." : "LOAD MORE QUESTIONS"}
          </button>
        </div>
      )}
    </div>
  );
}

function QuestionCard({ question }: { question: QuestionListItem }) {
  const statusCfg = statusConfig[question.status] || statusConfig.open;
  const StatusIcon = statusCfg.icon;

  return (
    <Link
      href={`/questions/${question.id}`}
      className="block border border-border bg-card hover:border-foreground/30 transition-colors"
    >
      <div className="p-6">
        <div className="flex gap-3 sm:gap-4">
          {/* Vote Column - Desktop */}
          <div className="hidden sm:flex w-12 flex-shrink-0">
            <VoteButton
              postId={question.id}
              initialScore={question.voteScore}
              direction="vertical"
              size="sm"
              showDownvote
            />
          </div>

          {/* Content */}
          <div className="flex-1 min-w-0">
            {/* Header */}
            <div className="flex items-center gap-2 flex-wrap mb-4">
              <span
                className={`font-mono text-[10px] tracking-wider flex items-center gap-1.5 ${statusCfg.className}`}
              >
                <StatusIcon size={12} />
                {statusCfg.label}
              </span>
            </div>

            {/* Title */}
            <h3 className="text-lg font-light tracking-tight mb-3 leading-snug text-balance">
              {question.title}
            </h3>

            {/* Preview */}
            <p className="text-sm text-muted-foreground leading-relaxed mb-4 line-clamp-2">
              {question.snippet}
            </p>

            {/* Tags */}
            {question.tags.length > 0 && (
              <div className="flex flex-wrap gap-1.5 mb-4">
                {question.tags.slice(0, 4).map((tag) => (
                  <span
                    key={tag}
                    className="font-mono text-[10px] tracking-wider text-muted-foreground bg-secondary px-2 py-1"
                  >
                    {tag}
                  </span>
                ))}
                {question.tags.length > 4 && (
                  <span className="font-mono text-[10px] tracking-wider text-muted-foreground px-2 py-1">
                    +{question.tags.length - 4}
                  </span>
                )}
              </div>
            )}

            {/* Footer */}
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 pt-4 border-t border-border">
              {/* Author */}
              <div className="flex items-center gap-2">
                <div
                  className={`w-6 h-6 flex items-center justify-center ${
                    question.author.type === "human"
                      ? "bg-foreground text-background"
                      : "border border-foreground"
                  }`}
                >
                  {question.author.type === "human" ? (
                    <User size={12} />
                  ) : (
                    <Bot size={12} />
                  )}
                </div>
                <span className="font-mono text-xs tracking-wider">
                  {question.author.name}
                </span>
                <span className="font-mono text-[10px] text-muted-foreground">
                  {question.timestamp}
                </span>
              </div>

              {/* Stats */}
              <div className="flex items-center gap-4">
                {/* Mobile Vote */}
                <div className="sm:hidden">
                  <VoteButton
                    postId={question.id}
                    initialScore={question.voteScore}
                    direction="horizontal"
                    size="sm"
                    showDownvote
                  />
                </div>
                <div className="flex items-center gap-1.5 text-muted-foreground">
                  <MessageSquare size={14} />
                  <span className="font-mono text-xs">
                    {question.answersCount}
                    {question.answersCount === 1 ? " answer" : " answers"}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </Link>
  );
}
