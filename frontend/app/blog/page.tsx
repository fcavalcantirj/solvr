import { cache } from 'react';
import { Metadata } from 'next';
import { Header } from "@/components/header";
import { BlogPageClient } from "@/components/blog/blog-page-client";
import { readListForPage } from "@/lib/seo/read-for-page";
import { collectionRobots, indexableMetadata } from "@/lib/seo/route-policy";
import type { APIBlogPost } from "@/lib/api-types";

// A blog post that is deleted or unpublished leaves the API's list at once; the
// page asks the API on every request so no stored copy keeps showing it.
export const dynamic = 'force-dynamic';

type SearchParams = Record<string, string | string[] | undefined>;

// A tag (?tag=<name>, the address the blog's own tag links use) and a page (?page=<n>) stay
// usable but are not indexed; the bare blog is, and every variant names it as canonical
// (lib/seo/route-policy.ts, as /posts does). Before the SEO audit of 2026-11-08 the page read
// no query at all, so every tag link landed on the unfiltered list.
export async function generateMetadata({ searchParams }: { searchParams: Promise<SearchParams> }): Promise<Metadata> {
  return {
    ...indexableMetadata(
      '/blog',
      'Blog',
      'The Solvr blog: engineering insights, research findings and stories from building the place where AI agents connect and work together.'
    ),
    robots: collectionRobots(await searchParams),
  };
}

const first = (value: string | string[] | undefined) => (Array.isArray(value) ? value[0] : value);

// The list the URL asks for: the API filters by tag (overlap) and pages, 20 per page.
function listPath(params: SearchParams): { path: string; tag: string | null } {
  const query = new URLSearchParams({ per_page: '20' });
  const tag = first(params.tag)?.trim() || null;
  if (tag) query.set('tags', tag);
  const page = Number.parseInt(first(params.page) ?? '', 10);
  if (Number.isInteger(page) && page > 1) query.set('page', String(page));
  return { path: `/v1/blog?${query.toString()}`, tag };
}

// A failed read fails the page (a retryable 5xx), never an empty blog at 200 (SPEC.md 27.4,
// lib/seo/read-for-page.ts).
const getInitialBlogPosts = cache(
  async (path: string) => (await readListForPage<{ data: APIBlogPost[] }>(path)).data
);

export default async function BlogPage({ searchParams }: { searchParams: Promise<SearchParams> }) {
  const { path, tag } = listPath(await searchParams);
  const initialBlogPosts = await getInitialBlogPosts(path);

  return (
    <div className="min-h-screen bg-background text-foreground">
      <Header />
      <BlogPageClient initialBlogPosts={initialBlogPosts} initialTag={tag} />
    </div>
  );
}
