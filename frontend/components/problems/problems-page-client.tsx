"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { ProblemsFilters } from "@/components/problems/problems-filters";
import { ProblemsList } from "@/components/problems/problems-list";
import { ProblemsSidebar } from "@/components/problems/problems-sidebar";
import { useAuth } from "@/hooks/use-auth";

export function ProblemsPageClient() {
  const router = useRouter();
  const { isAuthenticated } = useAuth();
  const [status, setStatus] = useState<string | undefined>(undefined);
  const [sort, setSort] = useState<'newest' | 'votes' | 'approaches'>('votes');
  const [tags, setTags] = useState<string[]>([]);
  const [searchQuery, setSearchQuery] = useState("");

  const handlePostProblem = () => {
    if (isAuthenticated) {
      router.push('/problems/new');
    } else {
      router.push('/login?next=/problems/new');
    }
  };

  return (
    <>
      {/* Filters */}
      <ProblemsFilters
        status={status}
        sort={sort}
        tags={tags}
        searchQuery={searchQuery}
        onStatusChange={setStatus}
        onSortChange={setSort}
        onTagsChange={setTags}
        onSearchQueryChange={setSearchQuery}
      />

      {/* Main Content */}
      <div className="max-w-7xl mx-auto px-6 lg:px-12 py-8">
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
          <div className="lg:col-span-2">
            <ProblemsList status={status} sort={sort} tags={tags} searchQuery={searchQuery} />
          </div>
          <div className="lg:col-span-1">
            <ProblemsSidebar onTagClick={(tag) => {
              if (!tags.includes(tag)) setTags([...tags, tag]);
            }} />
          </div>
        </div>
      </div>

      {/* Mobile CTA */}
      <div className="md:hidden fixed bottom-6 left-6 right-6">
        <button onClick={handlePostProblem} className="w-full font-mono text-xs tracking-wider bg-foreground text-background px-6 py-4 hover:bg-foreground/90 transition-colors">
          POST A PROBLEM
        </button>
      </div>
    </>
  );
}
