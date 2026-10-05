import { Metadata } from "next";
import { Header } from "@/components/header";
import { PostEditor } from "@/components/posts/post-editor";
import { linkPreview } from "@/lib/seo/link-preview";

const TITLE = "Edit post";

export const metadata: Metadata = {
  title: TITLE,
  robots: { index: false },
  ...linkPreview({ title: TITLE }),
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
        <div className="mx-auto max-w-[52rem] px-4 py-12 sm:px-6 lg:px-12 lg:py-16">
          <h1 className="mb-10 text-[2.5rem] font-light leading-[1.05] tracking-[-0.04em] sm:text-[3.5rem]">Edit post</h1>
          <PostEditor postId={id} />
        </div>
      </main>
    </div>
  );
}
