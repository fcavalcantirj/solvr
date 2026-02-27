import { cache } from 'react';
import { Metadata } from 'next';
import { notFound } from 'next/navigation';
import { Header } from "@/components/header";
import { ProblemDetailClient } from "@/components/problems/detail/problem-detail-client";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';

// Deduplicated server-side fetch — shared between generateMetadata and page component
// React cache() ensures this runs only ONCE per request even if called twice
const getPost = cache(async (id: string) => {
  try {
    const res = await fetch(`${API_BASE_URL}/v1/posts/${id}`, {
      next: { revalidate: 3600 }, // ISR: cache for 1 hour
    });
    if (!res.ok) return null;
    return res.json();
  } catch {
    return null;
  }
});

export async function generateMetadata({
  params,
}: {
  params: Promise<{ id: string }>;
}): Promise<Metadata> {
  const { id } = await params;
  const data = await getPost(id);
  if (!data?.data) return {};

  const post = data.data;
  const description = post.description
    ? post.description.replace(/[#*`\[\]]/g, '').slice(0, 160)
    : 'A problem on Solvr';

  return {
    title: post.title,
    description,
    openGraph: {
      title: post.title,
      description,
      type: 'article',
      publishedTime: post.created_at,
      modifiedTime: post.updated_at,
      tags: post.tags,
    },
    twitter: {
      card: 'summary',
      title: post.title,
      description,
    },
    alternates: {
      canonical: `/problems/${id}`,
    },
  };
}

export default async function ProblemDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const data = await getPost(id);

  // Proper 404 — Googlebot gets a real 404 status, not 200 with a spinner
  if (!data?.data) notFound();

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="pt-20">
        <div className="max-w-7xl mx-auto px-6 lg:px-12 py-8">
          <ProblemDetailClient id={id} initialPost={data.data} />
        </div>
      </main>
    </div>
  );
}
