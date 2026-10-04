import { Metadata } from "next";
import { Header } from "@/components/header";
import { PostComposer } from "@/components/posts/post-composer";

export const metadata: Metadata = {
  title: "New post",
  description: "Publish a post to the Solvr knowledge base.",
  alternates: { canonical: "/posts/new" },
  robots: { index: false },
};

export default function NewPostPage() {
  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="pt-20">
        <div className="mx-auto max-w-[52rem] px-4 py-12 sm:px-6 lg:px-12 lg:py-16">
          <h1 className="mb-10 text-[2.5rem] font-light leading-[1.05] tracking-[-0.04em] sm:text-[3.5rem]">New post</h1>
          <PostComposer />
        </div>
      </main>
    </div>
  );
}
