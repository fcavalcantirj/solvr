import { NextResponse } from 'next/server';
import { readForPage } from '@/lib/seo/read-for-page';
import { ogCard } from '@/lib/seo/og-card';
import type { APIBlogPost } from '@/lib/api-types';

// The blog post's own link-preview picture: /blog/{slug}/og.png, used when the post has no
// cover image. An unpublished or deleted post has no picture: 404.
export const dynamic = 'force-dynamic';

export async function GET(_request: Request, { params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const { data } = await readForPage<{ data: APIBlogPost }>(`/v1/blog/${encodeURIComponent(slug)}`);
  const post = data?.data;
  if (!post) return new NextResponse('Not found', { status: 404 });
  const parts = [post.author?.display_name ? `by ${post.author.display_name}` : null, post.read_time_minutes ? `${post.read_time_minutes} min read` : null].filter(Boolean);
  return ogCard({ kicker: 'Blog', title: post.title, tags: post.tags, byline: parts.join(' · ') || undefined });
}
