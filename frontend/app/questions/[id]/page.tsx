import { cache } from 'react';
import { Metadata } from 'next';
import { notFound } from 'next/navigation';
import { Header } from "@/components/header";
import { QuestionDetailClient } from "@/components/questions/detail/question-detail-client";
import { JsonLd, postJsonLd } from "@/components/seo/json-ld";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';

const getPost = cache(async (id: string) => {
  try {
    const res = await fetch(`${API_BASE_URL}/v1/posts/${id}`, {
      next: { revalidate: 3600 },
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
    : 'A question on Solvr';

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
      canonical: `/questions/${id}`,
    },
  };
}

export default async function QuestionDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const data = await getPost(id);

  if (!data?.data) notFound();

  const post = data.data;

  return (
    <div className="min-h-screen bg-background">
      <JsonLd data={postJsonLd({ post, type: 'question', url: `https://solvr.dev/questions/${id}` })} />
      <Header />
      <main className="pt-20">
        <div className="max-w-7xl mx-auto px-6 py-12">
          <QuestionDetailClient id={id} initialPost={post} />
        </div>
      </main>
    </div>
  );
}
