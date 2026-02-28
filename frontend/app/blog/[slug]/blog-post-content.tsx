"use client";

import Link from "next/link";
import Markdown from "react-markdown";
import {
  ArrowLeft,
  Calendar,
  Clock,
  User,
  Bot,
} from "lucide-react";
import { cn } from "@/lib/utils";
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
        <div className="max-w-3xl mx-auto">
          {/* Back link */}
          <Link
            href="/blog"
            className="inline-flex items-center gap-2 font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors mb-8"
          >
            <ArrowLeft size={14} />
            BACK TO BLOG
          </Link>

          <article>
            {/* Tags */}
            <div className="flex flex-wrap gap-2 mb-4">
              {post.tags.map((tag) => (
                <Link
                  key={tag}
                  href={`/blog?tag=${tag}`}
                  className="font-mono text-[10px] tracking-wider px-2 py-1 border border-border hover:border-foreground transition-colors"
                >
                  {tag.toUpperCase()}
                </Link>
              ))}
            </div>

            {/* Title */}
            <h1 className="text-2xl sm:text-3xl lg:text-4xl font-light leading-tight tracking-tight mb-6">
              {post.title}
            </h1>

            {/* Author info and meta */}
            <div className="flex items-center gap-4 mb-8 pb-6 border-b border-border">
              <div className="flex items-center gap-3">
                <div
                  className={cn(
                    "w-8 h-8 flex items-center justify-center",
                    post.author.type === "ai"
                      ? "bg-gradient-to-br from-cyan-400 to-blue-500 text-white"
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
                    <span className="font-mono text-sm">
                      {post.author.name}
                    </span>
                    {post.author.type === "ai" && (
                      <span className="font-mono text-[9px] tracking-wider px-1.5 py-0.5 bg-gradient-to-r from-cyan-400/20 to-blue-500/20 text-cyan-400 border border-cyan-400/30">
                        AI
                      </span>
                    )}
                  </div>
                </div>
              </div>
              <span className="text-muted-foreground">·</span>
              <div className="flex items-center gap-1.5 text-muted-foreground">
                <Calendar size={12} />
                <span className="font-mono text-xs">{post.publishedAt}</span>
              </div>
              <div className="flex items-center gap-1.5 text-muted-foreground">
                <Clock size={12} />
                <span className="font-mono text-xs">{post.readTime}</span>
              </div>
            </div>

            {/* Cover image */}
            {post.coverImageUrl && (
              <div className="aspect-[16/9] mb-8 bg-secondary overflow-hidden">
                <img
                  src={post.coverImageUrl}
                  alt={post.title}
                  className="w-full h-full object-cover"
                />
              </div>
            )}

            {/* Body */}
            <div className="prose prose-invert prose-sm sm:prose-base max-w-none mb-8 [&_h1]:text-2xl [&_h1]:font-light [&_h1]:tracking-tight [&_h2]:text-xl [&_h2]:font-light [&_h3]:text-lg [&_h3]:font-light [&_p]:text-muted-foreground [&_p]:leading-relaxed [&_a]:text-foreground [&_a]:underline [&_a]:underline-offset-4 [&_code]:font-mono [&_code]:text-sm [&_code]:bg-secondary [&_code]:px-1.5 [&_code]:py-0.5 [&_pre]:bg-secondary [&_pre]:border [&_pre]:border-border [&_blockquote]:border-l-2 [&_blockquote]:border-foreground [&_blockquote]:pl-4 [&_blockquote]:italic [&_ul]:list-disc [&_ol]:list-decimal [&_li]:text-muted-foreground">
              <Markdown>{post.body}</Markdown>
            </div>

            {/* Interactive elements */}
            <div className="pt-6 border-t border-border">
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
