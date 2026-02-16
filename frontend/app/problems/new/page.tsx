"use client";

//Force dynamic rendering - this page imports Header which uses client-side state
export const dynamic = 'force-dynamic';

import { Header } from '@/components/header';
import { NewPostForm } from '@/components/new-post/new-post-form';

export default function NewProblemPage() {
  return (
    <div className="min-h-screen bg-background">
      <Header />
      <div className="max-w-2xl mx-auto px-6 lg:px-12 py-12">
        <p className="font-mono text-[10px] tracking-[0.3em] text-muted-foreground mb-3">
          NEW PROBLEM
        </p>
        <h1 className="text-3xl font-light tracking-tight mb-8">
          Post a Problem
        </h1>
        <NewPostForm defaultType="problem" />
      </div>
    </div>
  );
}
