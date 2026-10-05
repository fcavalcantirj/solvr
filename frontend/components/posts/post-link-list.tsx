import Link from 'next/link';
import { CAPTION } from '@/components/page/caption';
import { profileHref } from '@/lib/profile-href';
import { trackNav } from '@/lib/track-attrs';

// A list of posts as plain links (SPEC.md 27.2): the post archive (/posts/page/{n}) and the
// profile pages render it on the server, so every post the sitemap lists has an inbound
// link a crawler follows without running a script. No hook and no browser API: a server
// component renders it as it is.
//
// The links are not prefetched. Their targets (a post, a profile) are rendered on demand and
// have no loading state, so the router has nothing to prefetch for them, yet it would still
// ask the server once for every link that scrolls into view: a hundred requests for one
// archive page, for every reader and for a crawler that renders the page. They stay ordinary
// links, followed on click.

export interface LinkedPost {
  id: string;
  title: string;
  created_at: string;
  author: { id: string; type: string; display_name: string };
}

// The post's UTC calendar day, so the server and every browser print the same date.
function day(iso: string): string {
  const date = new Date(iso);
  return Number.isNaN(date.getTime()) ? '' : date.toISOString().slice(0, 10);
}

const TITLE =
  'text-xl font-light leading-snug tracking-[-0.02em] underline decoration-transparent decoration-1 underline-offset-[5px] transition-colors hover:decoration-current focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-4 focus-visible:outline-foreground [overflow-wrap:anywhere] sm:text-2xl';
const AUTHOR =
  'underline decoration-transparent underline-offset-4 transition-colors hover:text-foreground hover:decoration-current focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground';

export function PostLinkList({
  posts,
  showAuthor = false,
  testId,
}: {
  posts: LinkedPost[];
  // The archive names each post's author; a profile lists one author's posts and does not.
  showAuthor?: boolean;
  testId?: string;
}) {
  if (posts.length === 0) return null;
  return (
    <ol data-testid={testId} className="min-w-0 border-t border-border">
      {posts.map((post) => {
        // A post outlives its author's account (the API then sends no display name): such a
        // row names no author, rather than a raw id linking a profile that no longer exists.
        const name = showAuthor ? (post.author?.display_name ?? '') : '';
        const profile = name ? profileHref(post.author) : null;
        return (
          <li
            key={post.id}
            className="grid min-w-0 gap-x-10 gap-y-2 border-b border-border py-5 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-baseline"
          >
            {/* item says what was pressed, never which post (SPEC.md 27.7). */}
            <Link href={`/posts/${post.id}`} prefetch={false} {...trackNav('post', 'page')} className={TITLE}>
              {post.title}
            </Link>
            <p className={`${CAPTION} flex flex-wrap items-baseline gap-x-4 gap-y-1`}>
              <time dateTime={post.created_at}>{day(post.created_at)}</time>
              {name ? (
                profile ? (
                  <Link href={profile} prefetch={false} {...trackNav('post_author', 'page')} className={AUTHOR}>
                    {name}
                  </Link>
                ) : (
                  <span>{name}</span>
                )
              ) : null}
            </p>
          </li>
        );
      })}
    </ol>
  );
}
