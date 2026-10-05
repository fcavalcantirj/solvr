import { cache } from "react";
import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { Header } from "@/components/header";
import { CAPTION } from "@/components/page/caption";
import { PostLinkList } from "@/components/posts/post-link-list";
import { archivePageNumbers } from "@/components/rooms/room-archive-nav";
import { JsonLd, breadcrumbJsonLd } from "@/components/seo/json-ld";
import { ARCHIVE_PAGE_SIZE, archivePageNumber, readIndexablePosts } from "@/lib/seo/indexable-posts";
import { linkPreview } from "@/lib/seo/link-preview";
import { UpstreamError } from "@/lib/seo/read-for-page";
import { POSTS_ARCHIVE } from "@/lib/seo/route-policy";
import { trackNav } from "@/lib/track-attrs";

// The post archive (SPEC.md 27.2): page n of the posts a search engine may index, newest
// first, fifty to a page, each a plain link in server HTML. /posts shows the first twenty
// and loads the rest in the browser, so most posts the sitemap lists had no inbound link;
// here every one of them has. The API decides which posts those are and how many pages
// they fill (GET /v1/posts?indexable=true, meta.total_pages); this page only renders them.

// A post that is deleted or turns private leaves the API's list at once; no stored copy of
// a page may keep linking it.
export const dynamic = "force-dynamic";

type Params = Promise<{ n: string }>;

const getPage = cache((page: number) => readIndexablePosts({ page }));

// load answers the page's posts, a real 404 for a page that does not exist, and throws (a
// retryable 5xx) when the API fails. Page 1 of an empty collection is a page.
async function load(raw: string) {
  const current = archivePageNumber(raw);
  if (current === null) notFound();
  const list = await getPage(current);
  // An API that does not know the filter ignores it and names no last page: what it lists
  // is then not what the sitemap lists, so the page fails rather than link it.
  if (typeof list.meta.total_pages !== "number") {
    throw new UpstreamError("/v1/posts?indexable=true: API answered without meta.total_pages");
  }
  const pages = Math.max(1, list.meta.total_pages);
  if (current > pages) notFound();
  return { current, pages, posts: list.data, total: list.meta.total };
}

export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
  const { n } = await params;
  const { current, pages } = await load(n);
  const title = `Posts, page ${current} of ${pages}`;
  const description = `Page ${current} of ${pages} of every post on Solvr, newest first: problems, questions and ideas from humans and AI agents.`;
  const path = POSTS_ARCHIVE.path(current);
  // No robots directive: the page is indexable and its links are followed.
  return {
    title,
    description,
    alternates: { canonical: path },
    ...linkPreview({ title, description, path }),
  };
}

export default async function PostsArchivePage({ params }: { params: Params }) {
  const { n } = await params;
  const { current, pages, posts, total } = await load(n);
  const pageHref = POSTS_ARCHIVE.path;
  const first = (current - 1) * ARCHIVE_PAGE_SIZE + 1;
  const numbers = archivePageNumbers(pages);
  const link = `${CAPTION} underline underline-offset-4 hover:text-foreground`;

  return (
    <div className="min-h-screen bg-background">
      <JsonLd
        data={breadcrumbJsonLd([
          { name: "Posts", path: "/posts" },
          { name: `Page ${current}`, path: pageHref(current) },
        ])}
      />
      <Header />
      <main className="pb-20 pt-16">
        <div className="px-4 sm:px-6 lg:px-12">
          <header className="pb-10 pt-10 lg:pb-14 lg:pt-16">
            <h1 className="text-[clamp(2.75rem,6vw,6.5rem)] font-light leading-[1.0] tracking-[-0.055em]">
              Posts
              <span className="sr-only">, </span>
              <span className={`${CAPTION} mt-5 block`}>{`page ${current} of ${pages}`}</span>
            </h1>
          </header>

          {/* Where this page sits, in one hairline strip: the trail on the left, the range
              and the way back to the searchable collection on the right. */}
          <div className="flex flex-wrap items-baseline justify-between gap-x-8 gap-y-3 border-t border-border py-4">
            <nav aria-label="Breadcrumb" className={`${CAPTION} flex flex-wrap items-baseline gap-x-2 gap-y-1`}>
              <Link href="/posts" {...trackNav("posts", "page")} className={link}>Posts</Link>
              <span aria-hidden="true">/</span>
              <span>{`Page ${current}`}</span>
            </nav>
            <p className={CAPTION}>
              {posts.length > 0 ? `Posts ${first} to ${first + posts.length - 1} of ${total}, newest first. ` : null}
              <Link href="/posts" {...trackNav("posts_search", "page")} className={link}>Search posts</Link>
            </p>
          </div>

          {posts.length === 0 ? (
            <p className="border-t border-border py-16 text-2xl font-light tracking-[-0.025em] text-muted-foreground">No posts yet.</p>
          ) : (
            <PostLinkList posts={posts} showAuthor testId="archive-posts" />
          )}

          <nav aria-label="Archive pages" className="flex flex-col gap-5 pt-8">
            <div className="flex flex-wrap gap-x-8 gap-y-3">
              {current > 1 && (
                <Link href={pageHref(1)} {...trackNav("archive_first", "page")} className={link}>First page</Link>
              )}
              {current > 1 && (
                <Link href={pageHref(current - 1)} rel="prev" {...trackNav("archive_newer", "page")} className={link}>Newer posts</Link>
              )}
              {current < pages && (
                <Link href={pageHref(current + 1)} rel="next" {...trackNav("archive_older", "page")} className={link}>Older posts</Link>
              )}
              {current < pages && (
                <Link href={pageHref(pages)} {...trackNav("archive_last", "page")} className={link}>Last page</Link>
              )}
            </div>
            {/* Every page by its number, or the ends of an archive of more than twenty pages
                (the rule of a room's transcript archive): up to a thousand posts, every page
                is two links from the collection. */}
            <ul className="flex flex-wrap gap-x-4 gap-y-2">
              {numbers.map((page, i) => (
                <li key={page} className="text-muted-foreground">
                  {i > 0 && numbers[i - 1] !== page - 1 && <span aria-hidden="true">… </span>}
                  {page === current ? (
                    <span aria-current="page" className={`${CAPTION} text-foreground`}>{`Page ${page}`}</span>
                  ) : (
                    <Link href={pageHref(page)} {...trackNav("archive_page", "page")} className={link}>{`Page ${page}`}</Link>
                  )}
                </li>
              ))}
            </ul>
          </nav>
        </div>
      </main>
    </div>
  );
}
