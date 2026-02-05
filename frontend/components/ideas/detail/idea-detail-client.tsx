"use client";

import { useIdea } from "@/hooks/use-idea";
import { IdeaHeader } from "./idea-header";
import { IdeaContent } from "./idea-content";
import { IdeaDiscussion } from "./idea-discussion";
import { IdeaSidePanel } from "./idea-side-panel";
import { Spinner } from "@/components/ui/spinner";

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

  if (!idea) {
    return (
      <div className="py-20 text-center">
        <p className="text-muted-foreground font-mono text-sm">Idea not found</p>
      </div>
    );
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
