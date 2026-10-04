"use client";

import Link from "next/link";
import {
  ArrowLeft,
  Calendar,
  Clock,
  User,
  Bot,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { MarkdownContent } from "@/components/shared/markdown-content";
import { BlogPostClient } from "./blog-post-client";
import { Header } from "@/components/header";
import { Footer } from "@/components/footer";

export interface BlogPostData {
  slug: string;
  title: string;
  excerpt: string;
  body: string;
  tags: string[];
  coverImageUrl?: string;
  author: {
    name: string;
    type: "human" | "ai";
    avatar?: string;
  };
  readTime: string;
  publishedAt: string;
  voteScore: number;
  viewCount: number;
  userVote?: "up" | "down" | null;
}

export function BlogPostContent({ post }: { post: BlogPostData }) {
  return (
    <div className="min-h-screen bg-background text-foreground">
      <Header />

      <main className="pt-28 sm:pt-32 pb-16 px-4 sm:px-6 lg:px-12">
        <div className="mx-auto max-w-[76rem]">
          {/* Back link */}
          <Link
            href="/blog"
            className="mb-10 inline-flex items-center gap-2 font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground transition-colors hover:text-foreground"
          >
            <ArrowLeft size={14} />
            BACK TO BLOG
          </Link>

          <article>
            {/* Tags */}
            <div className="mb-6 flex flex-wrap gap-x-5 gap-y-2">
              {post.tags.map((tag) => (
                <Link
                  key={tag}
                  href={`/blog?tag=${tag}`}
                  className="font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground underline-offset-4 transition-colors hover:text-foreground hover:underline"
                >
                  {tag.toUpperCase()}
                </Link>
              ))}
            </div>

            {/* Title */}
            <h1 className="mb-10 max-w-[22ch] text-[2.5rem] font-light leading-[1.05] tracking-[-0.04em] sm:text-[3.5rem] lg:text-[4.5rem]">
              {post.title}
            </h1>

            {/* Author info and meta */}
            <div className="mb-12 flex flex-wrap items-center gap-x-5 gap-y-3 border-y border-border py-4">
              <div className="flex items-center gap-3">
                <div
                  className={cn(
                    "w-8 h-8 flex items-center justify-center",
                    post.author.type === "ai"
                      ? "border border-foreground text-foreground"
                      : "bg-foreground text-background"
                  )}
                >
                  {post.author.avatar ? (
                    <img
                      src={post.author.avatar}
                      alt=""
                      className="w-full h-full object-cover"
                    />
                  ) : post.author.type === "ai" ? (
                    <Bot size={16} />
                  ) : (
                    <User size={16} />
                  )}
                </div>
                <div>
                  <div className="flex items-center gap-2">
                    <span className="text-base">
                      {post.author.name}
                    </span>
                    {post.author.type === "ai" && (
                      <span className="border border-foreground px-1.5 py-0.5 font-mono text-[11px] uppercase tracking-[0.18em]">
                        AI
                      </span>
                    )}
                  </div>
                </div>
              </div>
              <span className="text-muted-foreground">·</span>
              <div className="flex items-center gap-1.5 text-muted-foreground">
                <Calendar size={12} />
                <span className="font-mono text-[11px] tracking-[0.06em]">{post.publishedAt}</span>
              </div>
              <div className="flex items-center gap-1.5 text-muted-foreground">
                <Clock size={12} />
                <span className="font-mono text-[11px] tracking-[0.06em]">{post.readTime}</span>
              </div>
            </div>

            {/* Cover image */}
            {post.coverImageUrl && (
              <div className="mb-12 aspect-[16/9] overflow-hidden bg-secondary">
                <img
                  src={post.coverImageUrl}
                  alt={post.title}
                  className="w-full h-full object-cover"
                />
              </div>
            )}

            {/* Body */}
            <div className="max-w-[44rem]">
              <MarkdownContent content={post.body} className="mb-10 text-[1.0625rem] leading-relaxed" />
            </div>

            {/* Interactive elements */}
            <div className="max-w-[44rem] border-t border-border pt-6">
              <BlogPostClient
                slug={post.slug}
                initialVoteScore={post.voteScore}
                initialUserVote={post.userVote || null}
                viewCount={post.viewCount}
              />
            </div>
          </article>
        </div>
      </main>

      <Footer />
    </div>
  );
}
