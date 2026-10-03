import Link from 'next/link';
import type { APIRoomHistoryInfo } from '@/lib/api-types';

// Server-rendered links from a room to its transcript archive and its outcome
// posts (task idx 81). Ordinary anchors, so a crawler without JavaScript reaches
// every earlier message; Load older in the live view only enhances them.

// A long archive lists its first and last pages; each page links its neighbours,
// so every page stays reachable without hundreds of links on the room.
const LIST_ALL_UP_TO = 20;
const ENDS = 3;

export function archivePageNumbers(totalPages: number): number[] {
  if (totalPages <= 0) return [];
  if (totalPages <= LIST_ALL_UP_TO) return Array.from({ length: totalPages }, (_, i) => i + 1);
  const head = Array.from({ length: ENDS }, (_, i) => i + 1);
  const tail = Array.from({ length: ENDS }, (_, i) => totalPages - ENDS + 1 + i);
  return [...head, ...tail];
}

interface RoomArchiveNavProps {
  slug: string;
  history?: APIRoomHistoryInfo;
  outcomes: { id: string; title: string }[];
}

export function RoomArchiveNav({ slug, history, outcomes }: RoomArchiveNavProps) {
  const pages = archivePageNumbers(history?.total_pages ?? 0);
  if (pages.length === 0 && outcomes.length === 0) return null;
  const linkClass = 'font-mono text-xs underline underline-offset-4 hover:text-foreground';
  return (
    <nav aria-label="Room archive" className="border border-border bg-card p-4 space-y-4">
      {pages.length > 0 && (
        <div className="space-y-2">
          <h2 className="font-mono text-[10px] tracking-wider text-muted-foreground">FULL TRANSCRIPT</h2>
          <ul className="flex flex-wrap gap-x-3 gap-y-1">
            {pages.map((page, i) => (
              <li key={page} className="text-muted-foreground">
                {i > 0 && pages[i - 1] !== page - 1 && <span aria-hidden="true">… </span>}
                <Link href={`/rooms/${slug}/history/${page}`} className={linkClass}>
                  {`Page ${page}`}
                </Link>
              </li>
            ))}
          </ul>
        </div>
      )}
      {outcomes.length > 0 && (
        <div className="space-y-2">
          <h2 className="font-mono text-[10px] tracking-wider text-muted-foreground">OUTCOMES</h2>
          <ul className="space-y-1">
            {outcomes.map((post) => (
              <li key={post.id}>
                <Link href={`/posts/${post.id}`} className={linkClass}>
                  {post.title}
                </Link>
              </li>
            ))}
          </ul>
        </div>
      )}
    </nav>
  );
}
