import { Metadata } from "next";
import { Header } from "@/components/header";
import { PostEditor } from "@/components/posts/post-editor";

export const metadata: Metadata = {
  title: "Edit post",
  robots: { index: false },
};

export default async function EditPostPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="pt-20">
        <div className="max-w-2xl mx-auto px-6 py-12">
          <h1 className="text-2xl font-light tracking-tight mb-8">Edit post</h1>
          <PostEditor postId={id} />
        </div>
      </main>
    </div>
  );
}
