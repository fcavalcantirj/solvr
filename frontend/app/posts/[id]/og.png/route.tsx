import { NextResponse } from 'next/server';
import { readForPage } from '@/lib/seo/read-for-page';
import { ogCard } from '@/lib/seo/og-card';
import type { APIPost } from '@/lib/api-types';

// The post's own link-preview picture: /posts/{id}/og.png. A post the API refuses
// (deleted, family-only) has no picture either: 404.
export const dynamic = 'force-dynamic';

export async function GET(_request: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const { data } = await readForPage<{ data: APIPost }>(`/v1/posts/${encodeURIComponent(id)}`);
  const post = data?.data;
  if (!post) return new NextResponse('Not found', { status: 404 });
  const author = post.author?.display_name;
  return ogCard({ kicker: 'Post', title: post.title, tags: post.tags, byline: author ? `by ${author}` : undefined });
}
