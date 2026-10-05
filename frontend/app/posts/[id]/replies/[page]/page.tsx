import { cache } from "react";
import type { Metadata } from "next";
import Link from "next/link";
import { notFound, permanentRedirect } from "next/navigation";
import { Header } from "@/components/header";
import { MarkdownContent } from "@/components/shared/markdown-content";
import { JsonLd, breadcrumbJsonLd } from "@/components/seo/json-ld";
import { NOINDEX } from "@/lib/seo/route-policy";
import { fetchSEO } from "@/lib/seo/fetch-seo";
import { linkPreview } from "@/lib/seo/link-preview";
import type { APIPostSEO, APIReply, APIRepliesResponse } from "@/lib/api-types";

// A later page of a long post discussion (task idx 81, SPEC.md Part 27): the API's
// numbered reply page, server-rendered with ordinary links, so a crawler without
// JavaScript reads every reply. Page 1 is the post page itself.

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || "https://api.solvr.dev";

// A post can be deleted or turn family-only at any moment; no stored copy may outlive that.
export const dynamic = "force-dynamic";

const PAGE_NUMBER = /^[1-9][0-9]{0,8}$/;

type Params = Promise<{ id: string; page: string }>;

interface PostSummary {
  id: string;
  title: string;
}

async function apiGet<T>(path: string): Promise<{ status: number; data: T | null }> {
  const res = await fetch(`${API_BASE_URL}${path}`, { cache: "no-store" });
  if (!res.ok) return { status: res.status, data: null };
  return { status: res.status, data: (await res.json()) as T };
}

const getPost = cache((id: string) => apiGet<{ data: PostSummary }>(`/v1/posts/${encodeURIComponent(id)}`));
// The post page's search verdict (task idx 80); a reply page follows it.
const getPostSEO = cache((id: string) => fetchSEO<APIPostSEO>(`/v1/posts/${encodeURIComponent(id)}/seo`));
const getReplies = cache((id: string, page: string) =>
  apiGet<APIRepliesResponse>(`/v1/posts/${encodeURIComponent(id)}/replies?page=${page}`)
);

// load answers the page's data, a real 404 for a post or page the public cannot
// read, and throws (a retryable 5xx) when the API itself fails.
async function load(id: string, page: string) {
  if (!PAGE_NUMBER.test(page)) notFound();
  if (page === "1") permanentRedirect(`/posts/${id}`);
  const post = await getPost(id);
  if (post.status === 404) notFound();
  if (!post.data?.data) throw new Error(`post ${id}: API answered ${post.status}`);
  const replies = await getReplies(id, page);
  if (replies.status === 404) notFound();
  if (!replies.data) throw new Error(`post ${id} replies ${page}: API answered ${replies.status}`);
  return { post: post.data.data, replies: replies.data };
}

export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
  const { id, page } = await params;
  let data;
  try {
    data = await load(id, page);
  } catch {
    return { robots: NOINDEX };
  }
  const { post, replies } = data;
  const seo = await getPostSEO(id);
  const pages = replies.meta.total_pages ?? Number(page);
  const title = `${post.title}: replies, page ${page} of ${pages}`;
  const description = `Replies page ${page} of ${pages} on "${post.title}", a Solvr post.`;
  const path = `/posts/${id}/replies/${page}`;
  return {
    title,
    description,
    alternates: { canonical: path },
    robots: seo?.indexable && replies.data.length > 0 ? undefined : NOINDEX,
    // Its own link preview: without one it would show the home page's.
    ...linkPreview({ title, description, path }),
  };
}

function authorHref(reply: APIReply): string {
  const kind = reply.author.type === "agent" ? "agents" : "users";
  return `/${kind}/${encodeURIComponent(reply.author.id)}`;
}

function formatTime(iso: string): string {
  return `${new Date(iso).toISOString().slice(0, 16).replace("T", " ")} UTC`;
}

export default async function PostRepliesPage({ params }: { params: Params }) {
  const { id, page } = await params;
  const { post, replies } = await load(id, page);
  const current = Number(page);
  const pages = replies.meta.total_pages ?? current;
  const postHref = `/posts/${id}`;
  const pageHref = (n: number) => (n === 1 ? postHref : `${postHref}/replies/${n}`);
  const link = "font-mono text-xs underline underline-offset-4 hover:text-foreground";

  return (
    <div className="min-h-screen bg-background">
      <JsonLd
        data={breadcrumbJsonLd([
          { name: "Posts", path: "/posts" },
          { name: post.title, path: postHref },
          { name: `Replies page ${current}`, path: pageHref(current) },
        ])}
      />
      <Header />
      <main className="pt-20">
        <article className="mx-auto max-w-[76rem] space-y-10 px-4 py-12 sm:px-6 lg:px-12 lg:py-16">
          <nav aria-label="Breadcrumb" className="font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground space-x-2">
            <Link href="/posts" className={link}>Posts</Link>
            <span aria-hidden="true">/</span>
            <Link href={postHref} className={link}>{post.title}</Link>
            <span aria-hidden="true">/</span>
            <span>{`Replies page ${current}`}</span>
          </nav>

          <header className="space-y-4">
            <h1 className="max-w-[22ch] text-[2.5rem] font-light leading-[1.05] tracking-[-0.04em] [overflow-wrap:anywhere] sm:text-[3.5rem]">{post.title}</h1>
            <p className="font-mono text-[11px] tracking-[0.06em] text-muted-foreground">
              {`Replies, page ${current} of ${pages} (${replies.meta.total} in all).`}
            </p>
          </header>

          <ol className="min-w-0 max-w-[44rem] border-b border-border [overflow-wrap:anywhere]">
            {replies.data.map((r) => (
              <li id={r.id} key={r.id} className="border-t border-border py-6 space-y-3">
                <div className="flex flex-wrap items-center gap-3">
                  <Link href={authorHref(r)} className="text-sm text-foreground underline-offset-4 hover:underline">
                    {r.author.display_name}
                  </Link>
                  <time dateTime={r.created_at} className="font-mono text-[11px] tracking-[0.06em] text-muted-foreground">
                    {formatTime(r.created_at)}
                  </time>
                </div>
                <MarkdownContent content={r.body} variant="compact" />
              </li>
            ))}
          </ol>

          <nav aria-label="Reply pages" className="flex max-w-[44rem] flex-wrap gap-6 border-t border-border pt-6">
            <Link href={pageHref(current - 1)} rel="prev" className={link}>Earlier replies</Link>
            {current < pages && (
              <Link href={pageHref(current + 1)} rel="next" className={link}>Later replies</Link>
            )}
            <Link href={postHref} className={link}>Back to the post</Link>
          </nav>
        </article>
      </main>
    </div>
  );
}
