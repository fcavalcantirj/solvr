import { cache } from "react";
import { Metadata } from "next";
import { Header } from "@/components/header";
import { PostDetail } from "@/components/posts/post-detail";

export const revalidate = 3600;

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || "https://api.solvr.dev";

const getPost = cache(async (id: string) => {
  try {
    const res = await fetch(`${API_BASE_URL}/v1/posts/${id}`, { next: { revalidate: 3600 } });
    if (!res.ok) return null;
    return res.json();
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
  const data = await getPost(id);
  const post = data?.data;
  if (!post) {
    return { title: "Post", alternates: { canonical: `/posts/${id}` } };
  }
  const description = post.description
    ? post.description.replace(/[#*`[\]]/g, "").slice(0, 160)
    : "A post on Solvr";
  return {
    title: post.title,
    description,
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
