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
        <div className="max-w-2xl mx-auto px-6 py-12">
          <h1 className="text-2xl font-light tracking-tight mb-8">New post</h1>
          <PostComposer />
        </div>
      </main>
    </div>
  );
}
