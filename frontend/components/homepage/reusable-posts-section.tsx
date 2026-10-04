import Link from 'next/link';
import type { APIOverviewPosts } from '@/lib/api-types';
import { SectionHeading } from './metric';
import mosaic from '@/components/posts/posts-mosaic.module.css';

// Reusable Posts: the knowledge that survives a room. The API chooses which
// posts qualify, states the rule it used, routes each one to its own page and
// words both the contribution count and the age. Below three qualifying posts the
// API publishes no section at all, so this always has posts to show.

export function ReusablePostsSection({ data }: { data: APIOverviewPosts }) {
  return (
    <section
      data-testid="overview-section-posts"
      className="px-4 sm:px-6 lg:px-12 py-12 lg:py-16 border-t border-border"
    >
      <div className="mx-auto max-w-[78rem]">
        <SectionHeading heading={data.heading} intro={data.intro} />

        <p className="mt-4 max-w-[68ch] text-[0.8125rem] leading-relaxed text-muted-foreground">
          {data.definition}
        </p>

        {/* The same mosaic as /posts: tile sizes follow position, never the post. */}
        <ul className={`mt-12 ${mosaic.mosaic}`}>
          {data.items.map((item) => (
            <li key={item.id} className={mosaic.tile}>
              <div className="mb-5 flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 font-mono text-[11px] tracking-[0.06em] opacity-70">
                <span className="uppercase tracking-[0.18em]">{item.type} · {item.status}</span>
                <span>{item.last_activity_label}</span>
              </div>

              <h3 className={mosaic.title}>
                <Link href={item.url} className={mosaic.titleLink}>
                  {item.title}
                </Link>
              </h3>

              <div className={mosaic.footer}>
                <div className={mosaic.counts}>
                  <span>{item.contribution_label}</span>
                </div>
                {item.tags.length > 0 ? (
                  <div className={mosaic.tags}>
                    {item.tags.map((tag) => (
                      <span key={tag}>{tag}</span>
                    ))}
                  </div>
                ) : null}
              </div>
            </li>
          ))}
        </ul>

        <Link
          href={data.browse_url}
          className="group mt-10 inline-flex items-center gap-2 font-mono text-[11px] uppercase tracking-[0.18em] text-foreground transition-colors hover:text-muted-foreground"
        >
          {data.browse_label}
        </Link>
      </div>
    </section>
  );
}
