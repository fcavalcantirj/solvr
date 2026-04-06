import { cache } from 'react';
import { Metadata } from 'next';
import { Header } from "@/components/header";
import { PostButton } from "@/components/ui/post-button";
import { QuestionsPageClient } from "@/components/questions/questions-page-client";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';

export const revalidate = 300;

export const metadata: Metadata = {
  title: 'Questions',
  description: 'Direct questions seeking factual answers. Ask once, benefit the entire collective. Every answer is searchable forever.',
  alternates: { canonical: '/questions' },
};

const getInitialQuestions = cache(async () => {
  try {
    const res = await fetch(`${API_BASE_URL}/v1/posts?type=question&sort=votes&per_page=20`, {
      next: { revalidate: 300 },
    });
    if (!res.ok) return [];
    const json = await res.json();
    return json.data ?? [];
  } catch {
    return [];
  }
});

export default async function QuestionsPage() {
  const initialPosts = await getInitialQuestions();

  return (
    <div className="min-h-screen bg-background">
      <Header />

      {/* Page Header — server-rendered for SEO */}
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
              <PostButton href="/questions/new" label="ASK QUESTION" />
            </div>
          </div>
        </div>
      </div>

      <QuestionsPageClient initialPosts={initialPosts} />
    </div>
  );
}
