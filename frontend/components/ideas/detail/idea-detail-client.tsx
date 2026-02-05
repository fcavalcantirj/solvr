"use client";

import { useIdea } from "@/hooks/use-idea";
import { IdeaHeader } from "./idea-header";
import { IdeaContent } from "./idea-content";
import { IdeaDiscussion } from "./idea-discussion";
import { IdeaSidePanel } from "./idea-side-panel";
import { Spinner } from "@/components/ui/spinner";
import { ErrorState } from "@/components/ui/error-state";

interface IdeaDetailClientProps {
  id: string;
}

export function IdeaDetailClient({ id }: IdeaDetailClientProps) {
  const { idea, loading, error, refetch } = useIdea(id);

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <Spinner className="w-8 h-8" />
      </div>
    );
  }

  if (error) {
    return <ErrorState error={error} onRetry={refetch} resourceName="idea" />;
  }

  if (!idea) {
    return <ErrorState error="not found" resourceName="idea" />;
  }

  return (
    <>
      <IdeaHeader idea={idea} />
      <div className="mt-8 grid grid-cols-1 lg:grid-cols-3 gap-8">
        <div className="lg:col-span-2 space-y-8">
          <IdeaContent idea={idea} />
          <IdeaDiscussion ideaId={idea.id} onResponsePosted={refetch} />
        </div>
        <div className="lg:col-span-1">
          <IdeaSidePanel idea={idea} />
        </div>
      </div>
    </>
  );
}
