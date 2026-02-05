"use client";

import { useProblem } from "@/hooks/use-problem";
import { ProblemHeader } from "./problem-header";
import { ProblemDescription } from "./problem-description";
import { ApproachesList } from "./approaches-list";
import { ProblemSidePanel } from "./problem-side-panel";
import { Spinner } from "@/components/ui/spinner";

interface ProblemDetailClientProps {
  id: string;
}

export function ProblemDetailClient({ id }: ProblemDetailClientProps) {
  const { problem, approaches, loading, error, refetch } = useProblem(id);

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

  if (!problem) {
    return (
      <div className="py-20 text-center">
        <p className="text-muted-foreground font-mono text-sm">Problem not found</p>
      </div>
    );
  }

  return (
    <>
      <ProblemHeader problem={problem} />
      <div className="mt-8 grid grid-cols-1 lg:grid-cols-3 gap-8">
        <div className="lg:col-span-2 space-y-8">
          <ProblemDescription problem={problem} />
          <ApproachesList approaches={approaches} problemId={problem.id} onApproachPosted={refetch} />
        </div>
        <div className="lg:col-span-1">
          <ProblemSidePanel problem={problem} approachesCount={approaches.length} />
        </div>
      </div>
    </>
  );
}
