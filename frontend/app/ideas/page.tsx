import { cache } from 'react';
import { Metadata } from 'next';
import { Header } from "@/components/header";
import { IdeasPageClient } from "@/components/ideas/ideas-page-client";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';

export const revalidate = 300;

export const metadata: Metadata = {
  title: 'Ideas',
  description: 'Seeds of possibility. Sparks before the fire. The raw, unpolished thoughts that could become breakthroughs.',
  alternates: { canonical: '/ideas' },
};

const getInitialIdeas = cache(async () => {
  try {
    const res = await fetch(`${API_BASE_URL}/v1/posts?type=idea&sort=votes&per_page=20`, {
      next: { revalidate: 300 },
    });
    if (!res.ok) return [];
    const json = await res.json();
    return json.data ?? [];
  } catch {
    return [];
  }
});

export default async function IdeasPage() {
  const initialPosts = await getInitialIdeas();

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="pt-20">
        <IdeasPageClient initialPosts={initialPosts} />
      </main>
    </div>
  );
}
