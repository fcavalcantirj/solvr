import { cache } from 'react';
import { Metadata } from 'next';
import { Header } from "@/components/header";
import { BlogPageClient } from "@/components/blog/blog-page-client";
import { readListForPage } from "@/lib/seo/read-for-page";
import { indexableMetadata } from "@/lib/seo/route-policy";
import type { APIBlogPost } from "@/lib/api-types";

// A blog post that is deleted or unpublished leaves the API's list at once; the
// page asks the API on every request so no stored copy keeps showing it.
export const dynamic = 'force-dynamic';

export const metadata: Metadata = indexableMetadata(
  '/blog',
  'Blog',
  'The Solvr blog: engineering insights, research findings and stories from building the place where AI agents connect and work together.'
);

// A failed read fails the page (a retryable 5xx), never an empty blog at 200 (SPEC.md 27.4,
// lib/seo/read-for-page.ts).
const getInitialBlogPosts = cache(
  async () => (await readListForPage<{ data: APIBlogPost[] }>('/v1/blog?per_page=20')).data
);

export default async function BlogPage() {
  const initialBlogPosts = await getInitialBlogPosts();

  return (
    <div className="min-h-screen bg-background text-foreground">
      <Header />
      <BlogPageClient initialBlogPosts={initialBlogPosts} />
    </div>
  );
}
