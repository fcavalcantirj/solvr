import Link from 'next/link';
import type { APIOverviewPosts } from '@/lib/api-types';
import { SectionHeading } from './metric';

// Reusable Posts: the knowledge that survives a room. The API chooses which
// posts qualify, states the rule it used, routes each one to its own page and
// words both the contribution count and the age.

export function ReusablePostsSection({ data }: { data: APIOverviewPosts }) {
  return (
    <section
      data-testid="overview-section-posts"
      className="px-4 sm:px-6 lg:px-12 py-24 lg:py-32 border-t border-border"
    >
      <div className="max-w-7xl mx-auto">
        <SectionHeading eyebrow="POSTS" heading={data.heading} intro={data.intro} />

        <p className="mt-4 font-mono text-[10px] tracking-wider text-muted-foreground max-w-3xl">
          {data.definition}
        </p>

        {data.items.length === 0 ? (
          <p className="mt-12 text-sm text-muted-foreground">{data.empty_note}</p>
        ) : (
          <ul className="mt-12 border border-border divide-y divide-border">
            {data.items.map((item) => (
              <li key={item.id} className="p-5 sm:p-6">
                <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
                  <p className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground">
                    {item.type} · {item.status}
                  </p>
                  <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
                    {item.last_activity_label}
                  </span>
                </div>

                <Link
                  href={item.url}
                  className="block mt-2 text-lg font-light tracking-tight underline underline-offset-4 hover:no-underline"
                >
                  {item.title}
                </Link>

                <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-2">
                  <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
                    {item.contribution_label}
                  </span>
                  {item.tags.map((tag) => (
                    <span
                      key={tag}
                      className="font-mono text-[10px] tracking-wider border border-border px-2 py-0.5 text-muted-foreground"
                    >
                      {tag}
                    </span>
                  ))}
                </div>
              </li>
            ))}
          </ul>
        )}

        <Link
          href={data.browse_url}
          className="inline-block mt-10 font-mono text-xs uppercase tracking-wider border border-foreground px-8 py-4 hover:bg-foreground hover:text-background transition-colors"
        >
          {data.browse_label}
        </Link>
      </div>
    </section>
  );
}
