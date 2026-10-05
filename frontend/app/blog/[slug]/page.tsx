import { cache } from 'react';
import { Metadata } from 'next';
import { notFound } from 'next/navigation';
import { readForPage } from '@/lib/seo/read-for-page';
import { JsonLd, blogPostJsonLd } from "@/components/seo/json-ld";
import { linkPreview } from "@/lib/seo/link-preview";
import { BlogPostContent } from "./blog-post-content";
import { formatRelativeTime } from "@/lib/api";

// A blog post that is deleted or unpublished is refused by the API at once; the
// page asks the API on every request so its body is never served from a copy.
export const dynamic = 'force-dynamic';


// Deduplicated server-side fetch — shared between generateMetadata and page component
// React cache() ensures this runs only ONCE per request even if called twice
// A refusal answers null (the page 404s); an API failure throws, a retryable 5xx
// (task idx 83, lib/seo/read-for-page.ts).
const getBlogPost = cache(async (slug: string) => (await readForPage<any>(`/v1/blog/${encodeURIComponent(slug)}`)).data); // eslint-disable-line @typescript-eslint/no-explicit-any

export async function generateMetadata({
  params,
}: {
  params: Promise<{ slug: string }>;
}): Promise<Metadata> {
  const { slug } = await params;
  const data = await getBlogPost(slug);
  if (!data?.data) return {};

  const post = data.data;
  // The API serves every post's description (SPEC.md 27.1): the author's meta description,
  // or one it composed from the body, Markdown removed and cut at a word. The page derives none.
  const description = post.meta_description;

  const path = `/blog/${slug}`;
  return {
    title: post.title,
    description,
    alternates: { canonical: path },
    ...linkPreview({
      title: post.title,
      description,
      path,
      type: 'article',
      article: { publishedTime: post.published_at || post.created_at, modifiedTime: post.updated_at, tags: post.tags },
      // A post with a cover image previews with it; any other shows the card.
      image: post.cover_image_url,
    }),
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
          created_at: raw.created_at,
          updated_at: raw.updated_at,
          published_at: raw.published_at,
          tags: raw.tags,
          // The author as the API names them (SPEC.md 27.3).
          author: raw.author ? { id: raw.author.id, display_name: raw.author.display_name, type: raw.author.type } : undefined,
        },
        url: `https://solvr.dev/blog/${slug}`,
        description: raw.meta_description,
      })} />
      <BlogPostContent post={post} />
    </>
  );
}
