"use client";

import { useMemo, useState } from "react";
import { QuestionsFilters } from "@/components/questions/questions-filters";
import { QuestionsList } from "@/components/questions/questions-list";
import { QuestionsSidebar } from "@/components/questions/questions-sidebar";
import { transformQuestion } from "@/hooks/use-questions";
import type { APIPost } from "@/lib/api-types";

interface QuestionsPageClientProps {
  initialPosts: APIPost[];
}

export function QuestionsPageClient({ initialPosts }: QuestionsPageClientProps) {
  const [status, setStatus] = useState<string | undefined>(undefined);
  const [hasAnswer, setHasAnswer] = useState<boolean | undefined>(undefined);
  const [sort, setSort] = useState<'newest' | 'votes' | 'answers'>('votes');
  const [tags, setTags] = useState<string[]>([]);
  const [searchQuery, setSearchQuery] = useState("");

  const initialQuestions = useMemo(() => initialPosts.map(transformQuestion), [initialPosts]);

  return (
    <>
      {/* Filters */}
      <QuestionsFilters
        status={status}
        hasAnswer={hasAnswer}
        sort={sort}
        tags={tags}
        searchQuery={searchQuery}
        onStatusChange={setStatus}
        onHasAnswerChange={setHasAnswer}
        onSortChange={setSort}
        onTagsChange={setTags}
        onSearchQueryChange={setSearchQuery}
      />

      {/* Main Content */}
      <div className="max-w-7xl mx-auto px-6 lg:px-12 py-8">
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
          <div className="lg:col-span-2">
            <QuestionsList
              status={status}
              hasAnswer={hasAnswer}
              sort={sort}
              tags={tags}
              searchQuery={searchQuery}
              initialQuestions={initialQuestions}
            />
          </div>
          <div className="lg:col-span-1">
            <QuestionsSidebar onTagClick={(tag) => {
              if (!tags.includes(tag)) setTags([...tags, tag]);
            }} />
          </div>
        </div>
      </div>
    </>
  );
}
