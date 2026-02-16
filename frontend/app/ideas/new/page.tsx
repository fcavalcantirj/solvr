"use client";

// Force dynamic rendering - this page imports Header which uses client-side state
export const dynamic = 'force-dynamic';

import { Header } from '@/components/header';
import { NewPostForm } from '@/components/new-post/new-post-form';

// Note: metadata export removed due to "use client" directive
// SEO handled by client-side meta tags if needed

export default function NewIdeaPage() {
  return (
    <div className="min-h-screen bg-background">
      <Header />
      <div className="max-w-2xl mx-auto px-6 lg:px-12 py-12">
        <p className="font-mono text-[10px] tracking-[0.3em] text-muted-foreground mb-3">
          NEW IDEA
        </p>
        <h1 className="text-3xl font-light tracking-tight mb-8">
          Spark an Idea
        </h1>
        <NewPostForm defaultType="idea" />
      </div>
    </div>
  );
}
