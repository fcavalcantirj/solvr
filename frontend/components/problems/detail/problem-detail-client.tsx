"use client";

import { useProblem } from "@/hooks/use-problem";
import { useViewTracking } from "@/hooks/use-view-tracking";
import { ProblemHeader } from "./problem-header";
import { ProblemDescription } from "./problem-description";
import { ApproachesList } from "./approaches-list";
import { ProblemSidePanel } from "./problem-side-panel";
import { CommentsList } from "@/components/shared/comments-list";
import { Spinner } from "@/components/ui/spinner";
import { ErrorState } from "@/components/ui/error-state";
import { APIPost } from "@/lib/api-types";

interface ProblemDetailClientProps {
  id: string;
  initialPost?: APIPost; // Server-side fetched post data — avoids loading spinner for SEO
}

export function ProblemDetailClient({ id, initialPost }: ProblemDetailClientProps) {
  const { problem, approaches, loading, error, refetch } = useProblem(id, initialPost);

  // Track view when problem is loaded
  useViewTracking(id, problem?.views ?? 0, { enabled: !!problem });

  if (loading && !initialPost) {
    return (
      <div className="flex items-center justify-center py-20">
        <Spinner className="w-8 h-8" />
      </div>
    );
  }

  if (error) {
    return <ErrorState error={error} onRetry={refetch} resourceName="problem" />;
  }

  if (!problem) {
    return <ErrorState error="not found" resourceName="problem" />;
  }

  return (
    <>
      <ProblemHeader problem={problem} />
      <div className="mt-8 grid grid-cols-1 lg:grid-cols-3 gap-8">
        <div className="lg:col-span-2 space-y-8">
          <ProblemDescription problem={problem} />
          <ApproachesList approaches={approaches} problemId={problem.id} onApproachPosted={refetch} />
          <CommentsList targetType="post" targetId={problem.id} onCommentPosted={refetch} />
        </div>
        <div className="lg:col-span-1">
          <ProblemSidePanel problem={problem} approachesCount={approaches.length} />
        </div>
      </div>
    </>
  );
}
