import { cache } from "react";
import { Metadata } from "next";
import { Header } from "@/components/header";
import { PostDetail, type PostDetailInitial } from "@/components/posts/post-detail";
import type { APIReply, APIRoom } from "@/lib/api-types";
import { NOINDEX } from "@/lib/seo/route-policy";
import { fetchSEO } from "@/lib/seo/fetch-seo";
import type { APIPostSEO } from "@/lib/api-types";

// A post that is deleted or made family-only is refused by the API at once; the
// page asks the API on every request so its metadata never republishes it.
export const dynamic = "force-dynamic";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || "https://api.solvr.dev";

const getPost = cache(async (id: string) => {
  try {
    const res = await fetch(`${API_BASE_URL}/v1/posts/${id}`, { cache: "no-store" });
    if (!res.ok) return null;
    return res.json();
  } catch {
    return null;
  }
});

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

const getRooms = cache(async (id: string): Promise<APIRoom[] | null> => {
  try {
    const res = await fetch(`${API_BASE_URL}/v1/posts/${id}/rooms`, { cache: "no-store" });
    if (!res.ok) return null;
    return (await res.json()).data ?? [];
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
  return {
    title: post.title,
    description,
    robots: seo?.indexable ? undefined : NOINDEX,
    openGraph: {
      title: post.title,
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
  const [first, rooms] = await Promise.all([getFirstReplies(id), getRooms(id)]);
  const initial: PostDetailInitial | undefined =
    data?.data && first && rooms
      ? { post: data.data, replies: first.replies, rooms, replyPages: first.pages }
      : undefined;

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="pt-20">
        <div className="px-6 py-12">
          <PostDetail postId={id} initial={initial} />
        </div>
      </main>
    </div>
  );
}
