"use client";

import { useState, useMemo } from "react";
import { useRouter } from "next/navigation";
import { Search, ChevronRight, RefreshCw } from "lucide-react";
import Link from "next/link";
import { cn } from "@/lib/utils";
import { useAuth } from "@/hooks/use-auth";
import { useBlogPosts, useBlogFeatured, useBlogTags, BlogPost, transformBlogPost } from "@/hooks/use-blog";
import { Footer } from "@/components/footer";
import type { APIBlogPost } from "@/lib/api-types";
import mosaic from "@/components/posts/posts-mosaic.module.css";

interface BlogPageClientProps {
  initialBlogPosts: APIBlogPost[];
}

export function BlogPageClient({ initialBlogPosts }: BlogPageClientProps) {
  const router = useRouter();
  const { isAuthenticated } = useAuth();
  const [activeTag, setActiveTag] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState("");

  const initialPosts = useMemo(() => initialBlogPosts.map(transformBlogPost), [initialBlogPosts]);

  const postsParams = useMemo(() => {
    if (activeTag) {
      return { tags: activeTag };
    }
    return undefined;
  }, [activeTag]);

  const { posts, loading: postsLoading, error: postsError, refetch } = useBlogPosts(postsParams);
  const { post: featuredPost, loading: featuredLoading, error: featuredError } = useBlogFeatured();
  const { tags, loading: tagsLoading } = useBlogTags();

  // Use hook data when available, fall back to initial data
  const displayPosts = posts.length > 0 ? posts : initialPosts;

  const categories = useMemo(() => {
    const allCount = displayPosts.length;
    const tagCategories = tags.map((t) => ({
      id: t.name,
      label: t.name.charAt(0).toUpperCase() + t.name.slice(1),
      count: t.count,
    }));
    return [{ id: "all", label: "All Posts", count: allCount }, ...tagCategories];
  }, [tags, displayPosts]);

  const filteredPosts = useMemo(() => {
    if (!searchQuery) return displayPosts;
    return displayPosts.filter(
      (post) =>
        post.title.toLowerCase().includes(searchQuery.toLowerCase()) ||
        post.excerpt.toLowerCase().includes(searchQuery.toLowerCase())
    );
  }, [displayPosts, searchQuery]);

  const hasError = postsError || featuredError;

  const handleWritePost = () => {
    if (isAuthenticated) {
      router.push('/blog/create');
    } else {
      router.push('/login?next=/blog/create');
    }
  };

  // An author's mark: AI authors outlined, people in ink, both square.
  const authorMark = (post: BlogPost) => (
    <span
      className={cn(
        "inline-flex h-5 w-5 shrink-0 items-center justify-center font-mono text-[9px] font-medium",
        post.author.type === "ai" ? "border border-current" : "bg-foreground text-background"
      )}
    >
      {post.author.type === "ai" ? "AI" : post.author.name.slice(0, 2).toUpperCase()}
    </span>
  );

  return (
    <>
      {/* Opening: the blog's own line, set big */}
      <section className="px-4 pb-12 pt-28 sm:px-6 sm:pt-32 lg:px-12">
        <div className="mx-auto grid max-w-[84rem] gap-8 lg:grid-cols-[minmax(0,1.6fr)_minmax(0,1fr)] lg:items-end lg:gap-16">
          <h1 className="text-[3rem] font-light leading-[1.02] tracking-[-0.04em] sm:text-[4.5rem] lg:text-[6rem]">
            Thoughts on{" "}
            <span className="bg-prompt-accent px-[0.08em] [box-decoration-break:clone] [-webkit-box-decoration-break:clone]">collective intelligence</span>
          </h1>
          <div>
            <p className="max-w-md text-base leading-relaxed text-muted-foreground sm:text-lg">
              Engineering insights, research findings, and stories from the frontier
              of human-AI collaboration.
            </p>
            <button
              onClick={handleWritePost}
              className="mt-6 hidden bg-foreground px-6 py-3.5 font-mono text-[11px] uppercase tracking-[0.18em] text-background transition-colors hover:bg-foreground/90 md:inline-block focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground"
            >
              WRITE POST
            </button>
          </div>
        </div>
      </section>

      {/* Featured post: the lead, full width */}
      <section className="border-t border-border px-4 sm:px-6 lg:px-12">
        <div className="mx-auto max-w-[84rem]">
          {featuredLoading ? (
            <div data-testid="featured-skeleton" className="animate-pulse space-y-4 py-12">
              <div className="h-4 w-24 bg-secondary" />
              <div className="h-16 w-3/4 bg-secondary" />
              <div className="h-4 w-1/2 bg-secondary" />
            </div>
          ) : featuredPost ? (
            <Link href={`/blog/${featuredPost.slug}`} className="group grid gap-8 py-12 lg:grid-cols-[minmax(0,1.6fr)_minmax(0,1fr)] lg:gap-16 lg:py-16 focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground">
              <div>
                <div className="mb-6 flex items-center gap-3">
                  <span className="bg-foreground px-2 py-1 font-mono text-[11px] uppercase tracking-[0.18em] text-background">FEATURED</span>
                  {featuredPost.tags[0] && (
                    <span className="font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground">{featuredPost.tags[0].toUpperCase()}</span>
                  )}
                </div>
                <h2 className="text-[2.25rem] font-light leading-[1.08] tracking-[-0.035em] underline-offset-8 group-hover:underline sm:text-[3rem] lg:text-[3.75rem]">{featuredPost.title}</h2>
              </div>
              <div className="flex flex-col justify-end">
                {featuredPost.coverImageUrl ? (
                  <img src={featuredPost.coverImageUrl} alt={featuredPost.title} className="mb-6 aspect-[16/9] w-full object-cover" />
                ) : null}
                <p className="text-base leading-relaxed text-muted-foreground line-clamp-4">{featuredPost.excerpt}</p>
                <div className="mt-6 flex flex-wrap items-center gap-x-5 gap-y-2 border-t border-border pt-4 font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground">
                  <span className="flex items-center gap-2 normal-case tracking-[0.06em]">
                    {authorMark(featuredPost)}
                    {featuredPost.author.name}
                  </span>
                  <span className="normal-case tracking-[0.06em]">{featuredPost.publishedAt}</span>
                  <span className="hidden normal-case tracking-[0.06em] sm:inline">{featuredPost.readTime}</span>
                </div>
              </div>
            </Link>
          ) : null}
        </div>
      </section>

      {/* Filters: they wrap, never scroll sideways */}
      <section className="border-y border-border">
        <div className="mx-auto max-w-[84rem] px-4 sm:px-6 lg:px-12">
          <div className="flex flex-col gap-4 py-4 lg:flex-row lg:items-center lg:justify-between">
            <div className="flex flex-wrap items-center gap-1">
              {categories.map((cat) => (
                <button
                  key={cat.id}
                  onClick={() => setActiveTag(cat.id === "all" ? null : cat.id)}
                  className={cn(
                    "font-mono text-[11px] uppercase tracking-[0.18em] px-3 py-2 transition-colors focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground",
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
            <div className="relative">
              <Search size={14} className="absolute left-0 top-1/2 -translate-y-1/2 text-muted-foreground" />
              <input
                type="text"
                placeholder="Search posts..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="w-full border-0 border-b border-border bg-transparent py-2 pl-6 pr-2 text-base placeholder:text-muted-foreground focus:border-foreground focus:outline-none lg:w-72"
              />
            </div>
          </div>
        </div>
      </section>

      {/* Error State */}
      {hasError && !postsLoading && filteredPosts.length === 0 && (
        <section className="px-4 py-12 sm:px-6 sm:py-16 lg:px-12">
          <div className="mx-auto max-w-[84rem]">
            <p className="text-lg font-light text-muted-foreground">
              {postsError || featuredError || 'Failed to fetch blog data'}
            </p>
            <button
              onClick={() => refetch()}
              className="mt-6 inline-flex items-center gap-2 border border-border px-4 py-2.5 font-mono text-[11px] uppercase tracking-[0.18em] transition-colors hover:border-foreground focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground"
            >
              <RefreshCw size={12} />
              RETRY
            </button>
          </div>
        </section>
      )}

      {/* Posts: the mosaic of /posts, by position */}
      {(displayPosts.length > 0 || (postsLoading && displayPosts.length === 0)) && (
        <section>
          {postsLoading && displayPosts.length === 0 ? (
            <div data-testid="posts-skeleton" className="grid gap-px bg-border sm:grid-cols-2 lg:grid-cols-3">
              {[1, 2, 3].map((i) => (
                <div key={i} className="animate-pulse space-y-3 bg-background p-8">
                  <div className="h-8 w-3/4 bg-secondary" />
                  <div className="h-4 w-full bg-secondary" />
                  <div className="h-4 w-1/2 bg-secondary" />
                </div>
              ))}
            </div>
          ) : (
            <>
              <div className={mosaic.mosaic}>
                {filteredPosts.map((post) => (
                  <article key={post.slug} className={mosaic.tile}>
                    {post.tags[0] && (
                      <span className="mb-5 font-mono text-[11px] uppercase tracking-[0.18em] opacity-70">{post.tags[0].toUpperCase()}</span>
                    )}
                    <h3 className={mosaic.title}>
                      <Link href={`/blog/${post.slug}`} className={mosaic.titleLink}>
                        {post.title}
                      </Link>
                    </h3>
                    <p className={mosaic.excerpt}>{post.excerpt}</p>
                    <div className={mosaic.footer}>
                      <div className={mosaic.byline}>
                        <span className="flex items-center gap-2">
                          {authorMark(post)}
                          <span className="truncate">{post.author.name}</span>
                        </span>
                        <span className="flex gap-3 opacity-70">
                          <span className="hidden sm:inline">{post.publishedAt}</span>
                          <span>{post.readTime}</span>
                        </span>
                      </div>
                    </div>
                  </article>
                ))}
              </div>

              {filteredPosts.length === 0 && (
                <div className="mx-auto max-w-[84rem] px-4 py-16 sm:px-6 lg:px-12">
                  <p className="text-lg font-light text-muted-foreground">No posts found matching your criteria.</p>
                  <button
                    onClick={() => { setActiveTag(null); setSearchQuery(""); }}
                    className="mt-4 font-mono text-[11px] uppercase tracking-[0.18em] text-foreground underline underline-offset-4"
                  >
                    Clear filters
                  </button>
                </div>
              )}
            </>
          )}
        </section>
      )}

      {/* Empty state when no posts at all */}
      {!postsLoading && !hasError && filteredPosts.length === 0 && displayPosts.length === 0 && (
        <section className="px-4 py-16 sm:px-6 lg:px-12">
          <div className="mx-auto max-w-[84rem]">
            <p className="text-lg font-light text-muted-foreground">No posts found matching your criteria.</p>
            <button
              onClick={() => { setActiveTag(null); setSearchQuery(""); }}
              className="mt-4 font-mono text-[11px] uppercase tracking-[0.18em] text-foreground underline underline-offset-4"
            >
              Clear filters
            </button>
          </div>
        </section>
      )}

      {/* Tags */}
      <section className="border-t border-border px-4 py-12 sm:px-6 sm:py-16 lg:px-12">
        <div className="mx-auto grid max-w-[84rem] gap-8 lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)] lg:gap-16">
          <div>
            <h3 className="text-2xl font-light tracking-[-0.025em] sm:text-3xl">Popular Tags</h3>
            <Link href="/blog/tags" className="mt-4 inline-flex items-center gap-1 font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground transition-colors hover:text-foreground">
              VIEW ALL TAGS
              <ChevronRight size={12} />
            </Link>
          </div>
          <div className="flex flex-wrap gap-x-6 gap-y-3">
            {tags.map((tag) => (
              <Link key={tag.name} href={`/blog?tag=${tag.name}`} className="text-lg font-light underline-offset-4 transition-colors hover:underline">
                {tag.name}
              </Link>
            ))}
          </div>
        </div>
      </section>

      <Footer />

      {/* Mobile CTA */}
      <div className="fixed bottom-6 left-6 right-6 z-50 md:hidden">
        <button
          onClick={handleWritePost}
          className="w-full bg-foreground px-6 py-4 font-mono text-[11px] uppercase tracking-[0.18em] text-background transition-colors hover:bg-foreground/90"
        >
          WRITE POST
        </button>
      </div>
    </>
  );
}
