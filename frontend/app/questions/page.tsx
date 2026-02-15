"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Header } from "@/components/header";
import { QuestionsFilters } from "@/components/questions/questions-filters";
import { QuestionsList } from "@/components/questions/questions-list";
import { QuestionsSidebar } from "@/components/questions/questions-sidebar";
import { useAuth } from "@/hooks/use-auth";

export default function QuestionsPage() {
  const router = useRouter();
  const { isAuthenticated } = useAuth();
  const [status, setStatus] = useState<string | undefined>(undefined);
  const [sort, setSort] = useState<'newest' | 'votes' | 'answers'>('newest');
  const [tags, setTags] = useState<string[]>([]);

  const handleAskQuestion = () => {
    if (isAuthenticated) {
      router.push('/questions/new');
    } else {
      router.push('/login?next=/questions/new');
    }
  };

  return (
    <div className="min-h-screen bg-background">
      <Header />

      {/* Page Header */}
      <div className="border-b border-border bg-card">
        <div className="max-w-7xl mx-auto px-6 lg:px-12 py-12">
          <div className="flex items-end justify-between gap-8">
            <div>
              <p className="font-mono text-[10px] tracking-wider text-muted-foreground mb-3">
                QUICK KNOWLEDGE EXCHANGE
              </p>
              <h1 className="text-4xl md:text-5xl font-light tracking-tight">
                Questions
              </h1>
              <p className="mt-4 text-muted-foreground max-w-xl leading-relaxed">
                Direct questions seeking factual answers. Ask once, benefit the entire collective.
                Every answer is searchable forever.
              </p>
            </div>
            <div className="hidden md:block">
              <button onClick={handleAskQuestion} className="font-mono text-xs tracking-wider bg-foreground text-background px-6 py-3 hover:bg-foreground/90 transition-colors">
                ASK QUESTION
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Filters */}
      <QuestionsFilters
        status={status}
        sort={sort}
        tags={tags}
        onStatusChange={setStatus}
        onSortChange={setSort}
        onTagsChange={setTags}
      />

      {/* Main Content */}
      <div className="max-w-7xl mx-auto px-6 lg:px-12 py-8">
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
          <div className="lg:col-span-2">
            <QuestionsList status={status} sort={sort} tags={tags} />
          </div>
          <div className="lg:col-span-1">
            <QuestionsSidebar />
          </div>
        </div>
      </div>

      {/* Mobile CTA */}
      <div className="md:hidden fixed bottom-6 left-6 right-6">
        <button onClick={handleAskQuestion} className="w-full font-mono text-xs tracking-wider bg-foreground text-background px-6 py-4 hover:bg-foreground/90 transition-colors">
          ASK QUESTION
        </button>
      </div>
    </div>
  );
}
