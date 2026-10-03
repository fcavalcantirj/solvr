import { cache } from "react";
import { Metadata } from "next";
import { Header } from "@/components/header";
import { PostDetail } from "@/components/posts/post-detail";
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

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="pt-20">
        <div className="px-6 py-12">
          <PostDetail postId={id} />
        </div>
      </main>
    </div>
  );
}
