"use client";

import Link from "next/link";
import {
  TrendingUp,
  Flame,
  Loader2,
} from "lucide-react";
import { useTrending } from "@/hooks/use-stats";

export function FeedSidebar() {
  const { trending, loading, error } = useTrending();

  return (
    <div className="space-y-6">
      {/* Hot Discussions */}
      <div className="border border-border bg-card">
        <div className="flex items-center gap-2 p-4 border-b border-border">
          <Flame size={14} className="text-foreground" />
          <h3 className="font-mono text-xs tracking-[0.2em]">HOT RIGHT NOW</h3>
        </div>
        <div className="divide-y divide-border">
          {loading ? (
            <div className="p-4 flex items-center justify-center">
              <Loader2 size={16} className="animate-spin text-muted-foreground" />
            </div>
          ) : error ? (
            <div className="p-4 text-xs text-muted-foreground text-center">
              Failed to load
            </div>
          ) : trending?.posts && trending.posts.length > 0 ? (
            trending.posts.slice(0, 5).map((post, index) => (
              <Link
                key={post.id}
                href={`/${post.type}s/${post.id}`}
                className="block p-4 hover:bg-secondary/50 transition-colors group"
              >
                <div className="flex items-start gap-3">
                  <span className="font-mono text-[10px] text-muted-foreground w-4 mt-0.5">
                    {(index + 1).toString().padStart(2, "0")}
                  </span>
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-light leading-snug group-hover:text-foreground transition-colors line-clamp-2">
                      {post.title}
                    </p>
                    <div className="flex items-center gap-2 mt-2">
                      <span className="font-mono text-[9px] tracking-wider text-muted-foreground bg-secondary px-1.5 py-0.5">
                        {post.type.toUpperCase()}
                      </span>
                      <span className="font-mono text-[9px] text-muted-foreground">
                        {post.response_count} responses
                      </span>
                    </div>
                  </div>
                </div>
              </Link>
            ))
          ) : (
            <div className="p-4 text-xs text-muted-foreground text-center">
              No trending posts yet
            </div>
          )}
        </div>
      </div>

      {/* Trending Tags */}
      <div className="border border-border bg-card">
        <div className="flex items-center gap-2 p-4 border-b border-border">
          <TrendingUp size={14} className="text-foreground" />
          <h3 className="font-mono text-xs tracking-[0.2em]">TRENDING TAGS</h3>
        </div>
        <div className="p-4">
          {loading ? (
            <div className="flex items-center justify-center py-2">
              <Loader2 size={16} className="animate-spin text-muted-foreground" />
            </div>
          ) : error ? (
            <div className="text-xs text-muted-foreground text-center">
              Failed to load
            </div>
          ) : trending?.tags && trending.tags.length > 0 ? (
            <div className="space-y-3">
              {trending.tags.slice(0, 6).map((tag, index) => (
                <Link
                  key={tag.name}
                  href={`/search?tag=${tag.name}`}
                  className="flex items-center justify-between group"
                >
                  <div className="flex items-center gap-2">
                    <span className="font-mono text-[10px] text-muted-foreground w-4">
                      {(index + 1).toString().padStart(2, "0")}
                    </span>
                    <span className="font-mono text-xs group-hover:text-foreground transition-colors">
                      #{tag.name}
                    </span>
                  </div>
                  <div className="flex items-center gap-2">
                    {tag.growth > 0 && (
                      <span className="font-mono text-[9px] text-green-500">
                        +{tag.growth}%
                      </span>
                    )}
                    {tag.growth < 0 && (
                      <span className="font-mono text-[9px] text-red-500">
                        {tag.growth}%
                      </span>
                    )}
                    <span className="font-mono text-[10px] text-muted-foreground">
                      {tag.count}
                    </span>
                  </div>
                </Link>
              ))}
            </div>
          ) : (
            <div className="text-xs text-muted-foreground text-center">
              No trending tags yet
            </div>
          )}
        </div>
      </div>

      {/* Call to Action */}
      <div className="border border-border bg-card p-4">
        <p className="font-mono text-xs tracking-wider text-muted-foreground mb-3">
          READY TO CONTRIBUTE?
        </p>
        <div className="space-y-2">
          <Link
            href="/join"
            className="block w-full font-mono text-xs tracking-wider text-center py-2.5 bg-foreground text-background hover:bg-foreground/90 transition-colors"
          >
            JOIN SOLVR
          </Link>
          <Link
            href="/api-docs"
            className="block w-full font-mono text-xs tracking-wider text-center py-2.5 border border-border hover:border-foreground transition-colors"
          >
            API DOCS
          </Link>
        </div>
      </div>
    </div>
  );
}
