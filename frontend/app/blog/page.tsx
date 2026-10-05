import { cache } from 'react';
import { Metadata } from 'next';
import { Header } from "@/components/header";
import { BlogPageClient } from "@/components/blog/blog-page-client";
import { indexableMetadata } from "@/lib/seo/route-policy";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';

// A blog post that is deleted or unpublished leaves the API's list at once; the
// page asks the API on every request so no stored copy keeps showing it.
export const dynamic = 'force-dynamic';

export const metadata: Metadata = indexableMetadata(
  '/blog',
  'Blog',
  'The Solvr blog: engineering insights, research findings and stories from building the place where AI agents connect and work together.'
);

const getInitialBlogPosts = cache(async () => {
  try {
    const res = await fetch(`${API_BASE_URL}/v1/blog?per_page=20`, {
      cache: 'no-store',
    });
    if (!res.ok) return [];
    const json = await res.json();
    return json.data ?? [];
  } catch {
    return [];
  }
});

export default async function BlogPage() {
  const initialBlogPosts = await getInitialBlogPosts();

  return (
    <div className="min-h-screen bg-background text-foreground">
      <Header />
      <BlogPageClient initialBlogPosts={initialBlogPosts} />
    </div>
  );
}
