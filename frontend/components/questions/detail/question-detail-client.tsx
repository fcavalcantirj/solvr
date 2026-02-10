"use client";

import { useQuestion } from "@/hooks/use-question";
import { useViewTracking } from "@/hooks/use-view-tracking";
import { QuestionHeader } from "./question-header";
import { QuestionContent } from "./question-content";
import { AnswersList } from "./answers-list";
import { QuestionSidePanel } from "./question-side-panel";
import { CommentsList } from "@/components/shared/comments-list";
import { Spinner } from "@/components/ui/spinner";
import { ErrorState } from "@/components/ui/error-state";

interface QuestionDetailClientProps {
  id: string;
}

export function QuestionDetailClient({ id }: QuestionDetailClientProps) {
  const { question, answers, loading, error, refetch } = useQuestion(id);

  // Track view when question is loaded
  useViewTracking(id, question?.views ?? 0, { enabled: !!question });

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <Spinner className="w-8 h-8" />
      </div>
    );
  }

  if (error) {
    return <ErrorState error={error} onRetry={refetch} resourceName="question" />;
  }

  if (!question) {
    return <ErrorState error="not found" resourceName="question" />;
  }

  return (
    <>
      <QuestionHeader question={question} />
      <div className="mt-8 grid grid-cols-1 lg:grid-cols-3 gap-8">
        <div className="lg:col-span-2 space-y-8">
          <QuestionContent question={question} />
          <AnswersList answers={answers} questionId={question.id} onAnswerPosted={refetch} />
          <CommentsList targetType="post" targetId={question.id} onCommentPosted={refetch} />
        </div>
        <div className="lg:col-span-1">
          <QuestionSidePanel question={question} answersCount={answers.length} />
        </div>
      </div>
    </>
  );
}
