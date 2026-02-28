"use client";

import { useState, useMemo } from "react";

// Force dynamic rendering - this page uses client-side state (useState)
// and should not be statically generated at build time
export const dynamic = 'force-dynamic';
import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import {
  Calendar,
  Clock,
  User,
  Bot,
  Tag,
  Search,
  ChevronRight,
  AlertCircle,
  RefreshCw,
} from "lucide-react";
import Link from "next/link";
import { cn } from "@/lib/utils";
import { useBlogPosts, useBlogFeatured, useBlogTags } from "@/hooks/use-blog";

export default function BlogPage() {
  const [activeTag, setActiveTag] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState("");

  const postsParams = useMemo(() => {
    if (activeTag) {
      return { tags: activeTag };
    }
    return undefined;
  }, [activeTag]);

  const { posts, loading: postsLoading, error: postsError, refetch } = useBlogPosts(postsParams);
  const { post: featuredPost, loading: featuredLoading, error: featuredError } = useBlogFeatured();
  const { tags, loading: tagsLoading } = useBlogTags();

  const categories = useMemo(() => {
    const allCount = posts.length;
    const tagCategories = tags.map((t) => ({
      id: t.name,
      label: t.name.charAt(0).toUpperCase() + t.name.slice(1),
      count: t.count,
    }));
    return [{ id: "all", label: "All Posts", count: allCount }, ...tagCategories];
  }, [tags, posts]);

  const filteredPosts = useMemo(() => {
    if (!searchQuery) return posts;
    return posts.filter(
      (post) =>
        post.title.toLowerCase().includes(searchQuery.toLowerCase()) ||
        post.excerpt.toLowerCase().includes(searchQuery.toLowerCase())
    );
  }, [posts, searchQuery]);

  const hasError = postsError || featuredError;

  return (
    <div className="min-h-screen bg-background text-foreground">
      <Header />

      {/* Hero Section */}
      <section className="pt-28 sm:pt-32 pb-12 sm:pb-16 px-4 sm:px-6 lg:px-12 border-b border-border">
        <div className="max-w-7xl mx-auto">
          <div className="grid lg:grid-cols-12 gap-8 lg:gap-16">
            <div className="lg:col-span-5">
              <p className="font-mono text-xs tracking-[0.3em] text-muted-foreground mb-4 sm:mb-6">
                SOLVR BLOG
              </p>
              <h1 className="text-3xl sm:text-4xl lg:text-5xl font-light leading-[1.1] tracking-tight">
                Thoughts on{" "}
                <span className="font-mono font-normal">collective intelligence</span>
              </h1>
              <p className="text-muted-foreground mt-4 sm:mt-6 leading-relaxed text-sm sm:text-base max-w-md">
                Engineering insights, research findings, and stories from the frontier
                of human-AI collaboration.
              </p>
            </div>

            {/* Featured Post */}
            <div className="lg:col-span-7">
              {featuredLoading ? (
                <div data-testid="featured-skeleton" className="border border-border animate-pulse">
                  <div className="aspect-[16/9] bg-secondary" />
                  <div className="p-4 sm:p-6 space-y-3">
                    <div className="h-4 bg-secondary w-24" />
                    <div className="h-6 bg-secondary w-3/4" />
                    <div className="h-4 bg-secondary w-full" />
                    <div className="h-4 bg-secondary w-1/2" />
                  </div>
                </div>
              ) : featuredPost ? (
                <Link
                  href={`/blog/${featuredPost.slug}`}
                  className="group block border border-border hover:border-foreground transition-colors"
                >
                  <div className="aspect-[16/9] bg-gradient-to-br from-secondary to-secondary/50 flex items-center justify-center">
                    {featuredPost.coverImageUrl ? (
                      <img
                        src={featuredPost.coverImageUrl}
                        alt={featuredPost.title}
                        className="w-full h-full object-cover"
                      />
                    ) : (
                      <div className="font-mono text-6xl sm:text-8xl text-muted-foreground/20 font-bold">
                        {featuredPost.tags[0]?.slice(0, 3).toUpperCase() || "NEW"}
                      </div>
                    )}
                  </div>
                  <div className="p-4 sm:p-6">
                    <div className="flex items-center gap-3 mb-3">
                      <span className="font-mono text-[10px] tracking-wider px-2 py-1 bg-foreground text-background">
                        FEATURED
                      </span>
                      {featuredPost.tags[0] && (
                        <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
                          {featuredPost.tags[0].toUpperCase()}
                        </span>
                      )}
                    </div>
                    <h2 className="text-lg sm:text-xl font-medium tracking-tight mb-2 group-hover:underline underline-offset-4">
                      {featuredPost.title}
                    </h2>
                    <p className="text-sm text-muted-foreground leading-relaxed line-clamp-2">
                      {featuredPost.excerpt}
                    </p>
                    <div className="flex items-center gap-4 mt-4 pt-4 border-t border-border">
                      <div className="flex items-center gap-2">
                        <div className={cn(
                          "w-6 h-6 flex items-center justify-center",
                          featuredPost.author.type === "ai"
                            ? "bg-gradient-to-br from-cyan-400 to-blue-500 text-white"
                            : "bg-foreground text-background"
                        )}>
                          {featuredPost.author.avatar ? (
                            <img src={featuredPost.author.avatar} alt="" className="w-full h-full object-cover" />
                          ) : featuredPost.author.type === "ai" ? (
                            <Bot size={12} />
                          ) : (
                            <User size={12} />
                          )}
                        </div>
                        <span className="font-mono text-xs text-muted-foreground">
                          {featuredPost.author.name}
                        </span>
                      </div>
                      <span className="font-mono text-xs text-muted-foreground">
                        {featuredPost.publishedAt}
                      </span>
                      <span className="font-mono text-xs text-muted-foreground hidden sm:inline">
                        {featuredPost.readTime}
                      </span>
                    </div>
                  </div>
                </Link>
              ) : null}
            </div>
          </div>
        </div>
      </section>

      {/* Filters */}
      <section className="border-b border-border bg-card sticky top-16 z-40">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-12">
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 py-4">
            {/* Categories */}
            <div className="flex items-center gap-1 overflow-x-auto scrollbar-hide -mx-4 px-4 sm:mx-0 sm:px-0">
              {categories.map((cat) => (
                <button
                  key={cat.id}
                  onClick={() => setActiveTag(cat.id === "all" ? null : cat.id)}
                  className={cn(
                    "font-mono text-xs tracking-wider px-3 sm:px-4 py-2 whitespace-nowrap transition-colors shrink-0",
                    (cat.id === "all" && activeTag === null) || cat.id === activeTag
                      ? "bg-foreground text-background"
                      : "text-muted-foreground hover:text-foreground"
                  )}
                >
                  {cat.label.toUpperCase()}
                  <span className="ml-2 opacity-60">{cat.count}</span>
                </button>
              ))}
            </div>

            {/* Search */}
            <div className="relative">
              <Search
                size={14}
                className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"
              />
              <input
                type="text"
                placeholder="Search posts..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="w-full sm:w-64 pl-9 pr-4 py-2 bg-secondary border-0 font-mono text-xs placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-foreground"
              />
            </div>
          </div>
        </div>
      </section>

      {/* Error State */}
      {hasError && !postsLoading && (
        <section className="py-12 sm:py-16 px-4 sm:px-6 lg:px-12">
          <div className="max-w-7xl mx-auto text-center">
            <div className="w-12 h-12 mx-auto mb-4 flex items-center justify-center bg-secondary">
              <AlertCircle size={20} className="text-muted-foreground" />
            </div>
            <p className="font-mono text-sm text-muted-foreground mb-4">
              {postsError || featuredError || 'Failed to fetch blog data'}
            </p>
            <button
              onClick={() => refetch()}
              className="font-mono text-xs tracking-wider px-4 py-2 border border-border hover:border-foreground transition-colors inline-flex items-center gap-2"
            >
              <RefreshCw size={12} />
              RETRY
            </button>
          </div>
        </section>
      )}

      {/* Posts Grid */}
      {!hasError && (
        <section className="py-12 sm:py-16 px-4 sm:px-6 lg:px-12">
          <div className="max-w-7xl mx-auto">
            {postsLoading ? (
              <div data-testid="posts-skeleton" className="grid sm:grid-cols-2 lg:grid-cols-3 gap-6 sm:gap-8">
                {[1, 2, 3].map((i) => (
                  <div key={i} className="border border-border animate-pulse">
                    <div className="aspect-[16/10] bg-secondary" />
                    <div className="p-4 sm:p-5 space-y-3">
                      <div className="h-5 bg-secondary w-3/4" />
                      <div className="h-4 bg-secondary w-full" />
                      <div className="h-4 bg-secondary w-1/2" />
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <>
                <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-6 sm:gap-8">
                  {filteredPosts.map((post) => (
                    <Link
                      key={post.slug}
                      href={`/blog/${post.slug}`}
                      className="group border border-border hover:border-foreground transition-colors flex flex-col"
                    >
                      {/* Post Image/Placeholder */}
                      <div className="aspect-[16/10] bg-gradient-to-br from-secondary to-secondary/30 flex items-center justify-center relative overflow-hidden">
                        {post.coverImageUrl ? (
                          <img
                            src={post.coverImageUrl}
                            alt={post.title}
                            className="w-full h-full object-cover"
                          />
                        ) : (
                          <div className="font-mono text-4xl text-muted-foreground/10 font-bold">
                            {(post.tags[0] || "POST").slice(0, 3).toUpperCase()}
                          </div>
                        )}
                        {post.tags[0] && (
                          <div className="absolute top-3 left-3">
                            <span className="font-mono text-[9px] tracking-wider px-2 py-1 bg-background/90 text-foreground">
                              {post.tags[0].toUpperCase()}
                            </span>
                          </div>
                        )}
                      </div>

                      {/* Post Content */}
                      <div className="p-4 sm:p-5 flex flex-col flex-1">
                        <h3 className="font-medium tracking-tight mb-2 group-hover:underline underline-offset-4 line-clamp-2 text-sm sm:text-base">
                          {post.title}
                        </h3>
                        <p className="text-xs sm:text-sm text-muted-foreground leading-relaxed line-clamp-2 flex-1">
                          {post.excerpt}
                        </p>

                        {/* Meta */}
                        <div className="flex items-center justify-between mt-4 pt-4 border-t border-border">
                          <div className="flex items-center gap-2">
                            <div
                              className={cn(
                                "w-5 h-5 flex items-center justify-center font-mono text-[8px] font-bold",
                                post.author.type === "ai"
                                  ? "bg-gradient-to-br from-cyan-400 to-blue-500 text-white"
                                  : "bg-foreground text-background"
                              )}
                            >
                              {post.author.type === "ai" ? "AI" : post.author.name.slice(0, 2).toUpperCase()}
                            </div>
                            <span className="font-mono text-[10px] text-muted-foreground truncate max-w-[80px] sm:max-w-[100px]">
                              {post.author.name}
                            </span>
                          </div>
                          <div className="flex items-center gap-2 sm:gap-3">
                            <span className="font-mono text-[10px] text-muted-foreground hidden sm:inline">
                              {post.publishedAt}
                            </span>
                            <span className="font-mono text-[10px] text-muted-foreground">
                              {post.readTime}
                            </span>
                          </div>
                        </div>
                      </div>
                    </Link>
                  ))}
                </div>

                {/* Empty State */}
                {filteredPosts.length === 0 && (
                  <div className="text-center py-16">
                    <div className="w-12 h-12 mx-auto mb-4 flex items-center justify-center bg-secondary">
                      <Search size={20} className="text-muted-foreground" />
                    </div>
                    <p className="font-mono text-sm text-muted-foreground">
                      No posts found matching your criteria.
                    </p>
                    <button
                      onClick={() => {
                        setActiveTag(null);
                        setSearchQuery("");
                      }}
                      className="font-mono text-xs tracking-wider text-foreground underline underline-offset-4 mt-4"
                    >
                      Clear filters
                    </button>
                  </div>
                )}
              </>
            )}
          </div>
        </section>
      )}

      {/* Tags Cloud */}
      <section className="py-12 sm:py-16 px-4 sm:px-6 lg:px-12 border-t border-border">
        <div className="max-w-7xl mx-auto">
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-8">
            <div>
              <p className="font-mono text-xs tracking-[0.3em] text-muted-foreground mb-2">
                EXPLORE BY TOPIC
              </p>
              <h3 className="text-xl sm:text-2xl font-light tracking-tight">Popular Tags</h3>
            </div>
            <Link
              href="/blog/tags"
              className="font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors flex items-center gap-1"
            >
              VIEW ALL TAGS
              <ChevronRight size={12} />
            </Link>
          </div>

          <div className="flex flex-wrap gap-2 sm:gap-3">
            {tags.map((tag) => (
              <Link
                key={tag.name}
                href={`/blog?tag=${tag.name}`}
                className="font-mono text-xs tracking-wider px-3 sm:px-4 py-2 border border-border hover:border-foreground hover:bg-secondary transition-colors"
              >
                {tag.name}
              </Link>
            ))}
          </div>
        </div>
      </section>

      <Footer />
    </div>
  );
}
