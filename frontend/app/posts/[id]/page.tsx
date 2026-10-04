import { cache } from "react";
import { Metadata } from "next";
import { notFound } from "next/navigation";
import { Header } from "@/components/header";
import { readForPage } from "@/lib/seo/read-for-page";
import { PostDetail, type PostDetailInitial } from "@/components/posts/post-detail";
import { JsonLd, postPageJsonLd, breadcrumbJsonLd } from "@/components/seo/json-ld";
import type { APIPost, APIPostSourceRoom, APIReply, APIRoom } from "@/lib/api-types";
import { NOINDEX } from "@/lib/seo/route-policy";
import { fetchSEO } from "@/lib/seo/fetch-seo";
import type { APIPostSEO } from "@/lib/api-types";

// A post that is deleted or made family-only is refused by the API at once; the
// page asks the API on every request so its metadata never republishes it.
export const dynamic = "force-dynamic";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || "https://api.solvr.dev";

// A refusal answers null (the page 404s); an API failure throws, a retryable 5xx
// (task idx 83, lib/seo/read-for-page.ts).
const getPost = cache(async (id: string) => (await readForPage<{ data: APIPost }>(`/v1/posts/${id}`)).data);

// The page's search verdict (task idx 80): the API decides it at its own endpoint.
const getPostSEO = cache((id: string) => fetchSEO<APIPostSEO>(`/v1/posts/${id}/seo`));

// The first replies page and the post's rooms, read on the server so the page's
// HTML carries the discussion (task idx 81). A failure leaves them to the client.
const getFirstReplies = cache(async (id: string): Promise<{ replies: APIReply[]; pages: number } | null> => {
  try {
    const res = await fetch(`${API_BASE_URL}/v1/posts/${id}/replies?page=1`, { cache: "no-store" });
    if (!res.ok) return null;
    const json = await res.json();
    return { replies: json.data ?? [], pages: Math.max(1, json.meta?.total_pages ?? 1) };
  } catch {
    return null;
  }
});

// The rooms started from the post, and the public room it was saved from (task idx 82).
const getRooms = cache(async (id: string): Promise<{ rooms: APIRoom[]; sourceRoom: APIPostSourceRoom | null } | null> => {
  try {
    const res = await fetch(`${API_BASE_URL}/v1/posts/${id}/rooms`, { cache: "no-store" });
    if (!res.ok) return null;
    const json = await res.json();
    return { rooms: json.data ?? [], sourceRoom: json.source_room ?? null };
  } catch {
    return null;
  }
});

export async function generateMetadata({
  params,
}: {
  params: Promise<{ id: string }>;
}): Promise<Metadata> {
  const { id } = await params;
  const [data, seo] = await Promise.all([getPost(id), getPostSEO(id)]);
  const post = data?.data;
  if (!post) {
    return { title: "Post", robots: NOINDEX };
  }
  // The API decides whether the page may be indexed and what its description says
  // (task idx 80); the page only renders that.
  const description = seo?.description;
  // The API's title is unique among indexable posts (task idx 82).
  const title = seo?.title ?? post.title;
  return {
    title,
    description,
    robots: seo?.indexable ? undefined : NOINDEX,
    openGraph: {
      title,
      description,
      type: "article",
      publishedTime: post.created_at,
      modifiedTime: post.updated_at,
      tags: post.tags,
    },
    alternates: { canonical: `/posts/${id}` },
  };
}

export default async function PostDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const data = await getPost(id);
  const post = data?.data;
  // A post the API does not have is a real 404, not a 200 "Could not load" page.
  if (!post) notFound();
  const [first, rooms, seo] = await Promise.all([getFirstReplies(id), getRooms(id), getPostSEO(id)]);
  const initial: PostDetailInitial | undefined =
    post && first && rooms
      ? { post, replies: first.replies, rooms: rooms.rooms, replyPages: first.pages, sourceRoom: rooms.sourceRoom }
      : undefined;
  const url = `https://solvr.dev/posts/${id}`;
  const headline = seo?.title ?? post?.title;

  return (
    <div className="min-h-screen bg-background">
      {post && headline && (
        <>
          <JsonLd data={postPageJsonLd({ post, url, headline })} />
          <JsonLd data={breadcrumbJsonLd([{ name: "Posts", path: "/posts" }, { name: headline, path: `/posts/${id}` }])} />
        </>
      )}
      <Header />
      <main className="pt-20">
        <div className="px-4 py-12 sm:px-6 lg:px-12 lg:py-16">
          <PostDetail postId={id} initial={initial} />
        </div>
      </main>
    </div>
  );
}
