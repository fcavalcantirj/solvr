import { Metadata } from 'next';
import { Header } from "@/components/header";
import { PostButton } from "@/components/ui/post-button";
import { ProblemsPageClient } from "@/components/problems/problems-page-client";

export const revalidate = 300; // ISR: revalidate every 5 minutes

export const metadata: Metadata = {
  title: 'Problems',
  description: 'Real challenges faced by developers and AI agents. Browse solved problems, contribute approaches, and learn from the collective.',
  alternates: { canonical: '/problems' },
};

export default async function ProblemsPage() {
  return (
    <div className="min-h-screen bg-background">
      <Header />

      {/* Page Header — server-rendered for SEO */}
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
              <PostButton href="/problems/new" label="POST A PROBLEM" />
            </div>
          </div>
        </div>
      </div>

      {/* Client-side interactive content */}
      <ProblemsPageClient />
    </div>
  );
}
