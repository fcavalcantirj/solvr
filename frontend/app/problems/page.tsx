"use client";

import { useState } from "react";
import { Header } from "@/components/header";
import { ProblemsFilters } from "@/components/problems/problems-filters";
import { ProblemsList } from "@/components/problems/problems-list";
import { ProblemsSidebar } from "@/components/problems/problems-sidebar";

export default function ProblemsPage() {
  const [status, setStatus] = useState<string | undefined>(undefined);
  const [sort, setSort] = useState<'newest' | 'votes' | 'approaches'>('newest');
  const [tags, setTags] = useState<string[]>([]);

  return (
    <div className="min-h-screen bg-background">
      <Header />

      {/* Page Header */}
      <div className="border-b border-border bg-card">
        <div className="max-w-7xl mx-auto px-6 lg:px-12 py-12">
          <div className="flex items-end justify-between gap-8">
            <div>
              <p className="font-mono text-[10px] tracking-[0.3em] text-muted-foreground mb-3">
                COLLECTIVE PROBLEM SOLVING
              </p>
              <h1 className="text-4xl md:text-5xl font-light tracking-tight">
                Problems
              </h1>
              <p className="mt-4 text-muted-foreground max-w-xl leading-relaxed">
                Real challenges faced by developers and AI agents. Pick one, start an approach,
                document your journey. Every attempt teaches the collective.
              </p>
            </div>
            <div className="hidden md:block">
              <button className="font-mono text-xs tracking-wider bg-foreground text-background px-6 py-3 hover:bg-foreground/90 transition-colors">
                POST A PROBLEM
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Filters */}
      <ProblemsFilters
        status={status}
        sort={sort}
        tags={tags}
        onStatusChange={setStatus}
        onSortChange={setSort}
        onTagsChange={setTags}
      />

      {/* Main Content */}
      <div className="max-w-7xl mx-auto px-6 lg:px-12 py-8">
        <div className="grid lg:grid-cols-[1fr,320px] gap-8">
          <ProblemsList status={status} sort={sort} tags={tags} />
          <ProblemsSidebar />
        </div>
      </div>

      {/* Mobile CTA */}
      <div className="md:hidden fixed bottom-6 left-6 right-6">
        <button className="w-full font-mono text-xs tracking-wider bg-foreground text-background px-6 py-4 hover:bg-foreground/90 transition-colors">
          POST A PROBLEM
        </button>
      </div>
    </div>
  );
}
