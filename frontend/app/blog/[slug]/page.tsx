import { cache } from 'react';
import { Metadata } from 'next';
import { notFound } from 'next/navigation';
import { JsonLd, blogPostJsonLd } from "@/components/seo/json-ld";
import { BlogPostContent } from "./blog-post-content";
import { formatRelativeTime } from "@/lib/api";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';

// Deduplicated server-side fetch — shared between generateMetadata and page component
// React cache() ensures this runs only ONCE per request even if called twice
const getBlogPost = cache(async (slug: string) => {
  try {
    const res = await fetch(`${API_BASE_URL}/v1/blog/${encodeURIComponent(slug)}`, {
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
  params: Promise<{ slug: string }>;
}): Promise<Metadata> {
  const { slug } = await params;
  const data = await getBlogPost(slug);
  if (!data?.data) return {};

  const post = data.data;
  const description = post.meta_description
    || post.excerpt
    || (post.body ? post.body.replace(/[#*`\[\]]/g, '').slice(0, 160) : 'A blog post on Solvr');

  return {
    title: post.title,
    description,
    openGraph: {
      title: post.title,
      description,
      type: 'article',
      publishedTime: post.published_at || post.created_at,
      modifiedTime: post.updated_at,
      tags: post.tags,
    },
    twitter: {
      card: 'summary',
      title: post.title,
      description,
    },
    alternates: {
      canonical: `/blog/${slug}`,
    },
  };
}

export default async function BlogPostPage({
  params,
}: {
  params: Promise<{ slug: string }>;
}) {
  const { slug } = await params;
  const data = await getBlogPost(slug);

  // Proper 404 — Googlebot gets a real 404 status, not 200 with a spinner
  if (!data?.data) notFound();

  const raw = data.data;

  // Transform API response to client format (same logic as use-blog hook)
  const post = {
    slug: raw.slug,
    title: raw.title,
    excerpt: raw.excerpt || '',
    body: raw.body,
    tags: raw.tags || [],
    coverImageUrl: raw.cover_image_url || undefined,
    author: {
      name: raw.author.display_name,
      type: (raw.author.type === 'agent' ? 'ai' : 'human') as 'human' | 'ai',
      avatar: raw.author.avatar_url || undefined,
    },
    readTime: `${raw.read_time_minutes} min read`,
    publishedAt: raw.published_at ? formatRelativeTime(raw.published_at) : formatRelativeTime(raw.created_at),
    voteScore: raw.vote_score,
    viewCount: raw.view_count,
    userVote: raw.user_vote,
  };

  return (
    <>
      <JsonLd data={blogPostJsonLd({
        post: {
          title: raw.title,
          body: raw.body,
          excerpt: raw.excerpt,
          created_at: raw.created_at,
          updated_at: raw.updated_at,
          published_at: raw.published_at,
          tags: raw.tags,
          author: raw.author ? { display_name: raw.author.display_name } : undefined,
        },
        url: `https://solvr.dev/blog/${slug}`,
      })} />
      <BlogPostContent post={post} />
    </>
  );
}
