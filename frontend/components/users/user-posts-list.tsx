"use client";

import Link from "next/link";
import { ArrowUp, FileText, Loader2 } from "lucide-react";
import type { UserPostData } from "@/hooks/use-user";

export interface UserPostsListProps {
  posts: UserPostData[];
  loading?: boolean;
}

// Every post has one page, /posts/{id} (idx 68).
function getPostPath(post: UserPostData): string {
  return `/posts/${post.id}`;
}

// Icon for a post
function PostTypeIcon(_: { type: UserPostData['type'] }) {
  return <FileText size={14} />;
}

export function UserPostsList({ posts, loading = false }: UserPostsListProps) {
  if (loading) {
    return (
      <div className="border border-border p-12 text-center">
        <Loader2 size={24} className="mx-auto mb-4 text-muted-foreground animate-spin" />
        <p className="font-mono text-sm text-muted-foreground">Loading posts...</p>
      </div>
    );
  }

  if (posts.length === 0) {
    return (
      <div className="border border-dashed border-border p-12 text-center">
        <FileText size={24} className="mx-auto mb-4 text-muted-foreground" />
        <p className="font-mono text-sm text-muted-foreground">No posts yet</p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {posts.map((post) => (
        <article key={post.id} className="border border-border p-4 hover:border-foreground/20 transition-colors">
          <div className="flex gap-4">
            {/* Vote score */}
            <div className="flex flex-col items-center min-w-[60px]">
              <ArrowUp size={16} className="text-muted-foreground" />
              <span className="font-mono text-lg font-medium">{post.voteScore}</span>
            </div>

            {/* Content */}
            <div className="flex-1 min-w-0">
              {/* Type badge and title */}
              <div className="flex items-center gap-2 mb-2">
                <span className="inline-flex items-center gap-1 font-mono text-xs px-2 py-0.5 bg-muted text-muted-foreground">
                  <PostTypeIcon type={post.type} />
                  {post.type}
                </span>
              </div>

              <Link
                href={getPostPath(post)}
                className="block font-mono text-sm font-medium hover:underline mb-2"
              >
                {post.title}
              </Link>

              {/* Tags */}
              {post.tags.length > 0 && (
                <div className="flex flex-wrap gap-2 mb-2">
                  {post.tags.map((tag) => (
                    <span
                      key={tag}
                      className="font-mono text-xs px-2 py-0.5 bg-muted/50 text-muted-foreground"
                    >
                      {tag}
                    </span>
                  ))}
                </div>
              )}

              {/* Meta */}
              <div className="font-mono text-xs text-muted-foreground">
                {post.time} • {post.views} views
              </div>
            </div>
          </div>
        </article>
      ))}
    </div>
  );
}
