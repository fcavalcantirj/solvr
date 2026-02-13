import type { Metadata } from 'next';
import { Header } from "@/components/header";
import { IdeaDetailClient } from "@/components/ideas/detail/idea-detail-client";
import { api } from '@/lib/api';

function truncate(text: string, max: number = 160): string {
  if (text.length <= max) return text;
  return text.slice(0, max) + '...';
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ id: string }>;
}): Promise<Metadata> {
  try {
    const { id } = await params;
    const { data: post } = await api.getPost(id);
    const description = truncate(post.description);
    return {
      title: post.title,
      description,
      openGraph: {
        title: post.title,
        description,
        type: 'article',
        url: `/ideas/${id}`,
      },
      twitter: {
        title: post.title,
        description,
      },
    };
  } catch {
    return {
      title: 'Idea',
      description: 'An idea on Solvr',
    };
  }
}

export default async function IdeaDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="pt-20">
        <div className="max-w-7xl mx-auto px-6 py-12">
          <IdeaDetailClient id={id} />
        </div>
      </main>
    </div>
  );
}
