"use client";

import { useQuestion } from "@/hooks/use-question";
import { QuestionHeader } from "./question-header";
import { QuestionContent } from "./question-content";
import { AnswersList } from "./answers-list";
import { QuestionSidePanel } from "./question-side-panel";
import { Spinner } from "@/components/ui/spinner";

interface QuestionDetailClientProps {
  id: string;
}

export function QuestionDetailClient({ id }: QuestionDetailClientProps) {
  const { question, answers, loading, error, refetch } = useQuestion(id);

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <Spinner className="w-8 h-8" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="py-20 text-center">
        <p className="text-red-500 font-mono text-sm mb-4">{error}</p>
        <button
          onClick={refetch}
          className="px-4 py-2 bg-primary text-primary-foreground font-mono text-xs hover:bg-primary/90 transition-colors"
        >
          TRY AGAIN
        </button>
      </div>
    );
  }

  if (!question) {
    return (
      <div className="py-20 text-center">
        <p className="text-muted-foreground font-mono text-sm">Question not found</p>
      </div>
    );
  }

  return (
    <>
      <QuestionHeader question={question} />
      <div className="mt-8 grid grid-cols-1 lg:grid-cols-3 gap-8">
        <div className="lg:col-span-2 space-y-8">
          <QuestionContent question={question} />
          <AnswersList answers={answers} questionId={question.id} onAnswerPosted={refetch} />
        </div>
        <div className="lg:col-span-1">
          <QuestionSidePanel question={question} answersCount={answers.length} />
        </div>
      </div>
    </>
  );
}
