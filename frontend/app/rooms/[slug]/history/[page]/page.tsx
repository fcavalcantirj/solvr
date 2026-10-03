import { cache } from "react";
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import Link from "next/link";
import { Header } from "@/components/header";
import { MarkdownContent } from "@/components/shared/markdown-content";
import { NOINDEX } from "@/lib/seo/route-policy";
import { fetchSEO } from "@/lib/seo/fetch-seo";
import type { APIRoomDetailResponse, APIRoomHistoryPage, APIRoomMessage, APIRoomSEO } from "@/lib/api-types";

// One segment of a public room's transcript (task idx 81, SPEC.md Part 27): a fixed
// sequence range the API decides, server-rendered with ordinary links, so a crawler
// without JavaScript reads every message the live view loads on demand.

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || "https://api.solvr.dev";

// A room can turn private or be deleted at any moment; no stored copy may outlive that.
export const dynamic = "force-dynamic";

// Only the canonical spelling of a page number is a page: /history/01 is not /history/1.
const PAGE_NUMBER = /^[1-9][0-9]{0,8}$/;

type Params = Promise<{ slug: string; page: string }>;

async function apiGet<T>(path: string): Promise<{ status: number; data: T | null }> {
  const res = await fetch(`${API_BASE_URL}${path}`, { cache: "no-store" });
  if (!res.ok) return { status: res.status, data: null };
  return { status: res.status, data: (await res.json()) as T };
}

const getRoom = cache((slug: string) =>
  apiGet<APIRoomDetailResponse>(`/v1/rooms/${encodeURIComponent(slug)}`)
);
// The room page's search verdict (task idx 80); a transcript page follows it.
const getRoomSEO = cache((slug: string) =>
  fetchSEO<APIRoomSEO>(`/v1/rooms/${encodeURIComponent(slug)}/seo`)
);
const getHistory = cache((slug: string, page: string) =>
  apiGet<{ data: APIRoomHistoryPage }>(`/v1/rooms/${encodeURIComponent(slug)}/history/${page}`)
);

// load answers the page's data, a real 404 for a room or range the public cannot
// read, and throws (a retryable 5xx) when the API itself fails.
async function load(slug: string, page: string) {
  if (!PAGE_NUMBER.test(page)) notFound();
  const room = await getRoom(slug);
  if (room.status === 404 || room.status === 401 || room.status === 403) notFound();
  if (!room.data?.data?.room) throw new Error(`room ${slug}: API answered ${room.status}`);
  const history = await getHistory(slug, page);
  if (history.status === 404) notFound();
  if (!history.data?.data) throw new Error(`room ${slug} history ${page}: API answered ${history.status}`);
  return { room: room.data.data, history: history.data.data };
}

function rangeLabel(h: APIRoomHistoryPage): string {
  return `messages ${h.from_sequence}–${h.to_sequence}`;
}

export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
  const { slug, page } = await params;
  let data;
  try {
    data = await load(slug, page);
  } catch {
    return { robots: NOINDEX };
  }
  const { room, history } = data;
  const seo = await getRoomSEO(slug);
  const name = seo?.title ?? room.room.display_name;
  const indexable = seo?.indexable === true && history.messages.length > 0;
  return {
    title: `${name}: transcript, ${rangeLabel(history)}`,
    description: `Page ${history.page} of ${history.total_pages} of the ${name} room transcript on Solvr, ${rangeLabel(history)}.`,
    alternates: { canonical: `/rooms/${slug}/history/${history.page}` },
    robots: indexable ? undefined : NOINDEX,
  };
}

function formatTime(iso: string): string {
  return `${new Date(iso).toISOString().slice(0, 16).replace("T", " ")} UTC`;
}

function Author({ message }: { message: APIRoomMessage }) {
  const label = <span className="font-mono text-xs text-foreground">{message.agent_name}</span>;
  if (message.author_type === "agent" && message.author_id) {
    return (
      <Link href={`/agents/${encodeURIComponent(message.author_id)}`} className="hover:underline">
        {label}
      </Link>
    );
  }
  return label;
}

export default async function RoomHistoryPage({ params }: { params: Params }) {
  const { slug, page } = await params;
  const { room, history } = await load(slug, page);
  const name = room.room.display_name;
  const roomHref = `/rooms/${slug}`;
  const pageHref = (n: number) => `${roomHref}/history/${n}`;
  const link = "font-mono text-xs underline underline-offset-4 hover:text-foreground";

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="pt-20">
        <article className="max-w-3xl mx-auto px-6 py-12 space-y-8">
          <nav aria-label="Breadcrumb" className="font-mono text-xs text-muted-foreground space-x-2">
            <Link href="/rooms" className={link}>Rooms</Link>
            <span aria-hidden="true">/</span>
            <Link href={roomHref} className={link}>{name}</Link>
            <span aria-hidden="true">/</span>
            <span>{`Transcript page ${history.page}`}</span>
          </nav>

          <header className="space-y-2">
            <h1 className="text-2xl sm:text-3xl font-light tracking-tight">{`${name}: transcript`}</h1>
            <p className="font-mono text-xs text-muted-foreground">
              {`Page ${history.page} of ${history.total_pages}, ${rangeLabel(history)}.`}{" "}
              <Link href={roomHref} className={link}>Back to the live room</Link>
            </p>
          </header>

          {history.messages.length === 0 ? (
            <p className="font-mono text-sm text-muted-foreground">No messages remain in this range.</p>
          ) : (
            <ol className="space-y-4">
              {history.messages.map((m) => (
                <li
                  key={m.id}
                  id={m.sequence_num !== undefined ? `message-${m.sequence_num}` : undefined}
                  className="border border-border p-4 space-y-2"
                >
                  <div className="flex flex-wrap items-center gap-3">
                    <Author message={m} />
                    <time dateTime={m.created_at} className="font-mono text-xs text-muted-foreground">
                      {formatTime(m.created_at)}
                    </time>
                  </div>
                  <MarkdownContent content={m.content} variant="compact" />
                </li>
              ))}
            </ol>
          )}

          <nav aria-label="Transcript pages" className="flex flex-wrap gap-4 border-t border-border pt-6">
            {history.page > 1 && <Link href={pageHref(1)} className={link}>First page</Link>}
            {history.prev_page !== null && (
              <Link href={pageHref(history.prev_page)} rel="prev" className={link}>Earlier messages</Link>
            )}
            {history.next_page !== null && (
              <Link href={pageHref(history.next_page)} rel="next" className={link}>Later messages</Link>
            )}
            {history.page < history.total_pages && (
              <Link href={pageHref(history.total_pages)} className={link}>Last page</Link>
            )}
            <Link href={roomHref} className={link}>Live room</Link>
          </nav>
        </article>
      </main>
    </div>
  );
}
