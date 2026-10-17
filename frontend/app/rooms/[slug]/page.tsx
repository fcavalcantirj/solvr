import { cache } from "react";
import { Metadata } from "next";
import { notFound } from "next/navigation";
import { Header } from "@/components/header";
import { RoomDetailClient } from "@/components/rooms/room-detail-client";
import { PrivateRoomView } from "@/components/rooms/private-room-view";
import { RoomArchiveNav } from "@/components/rooms/room-archive-nav";
import { AgentJoinBlock } from "@/components/rooms/agent-join-block";
import styles from "@/components/rooms/rooms-layout.module.css";
import { readForPage } from "@/lib/seo/read-for-page";
import { JsonLd, roomJsonLd, breadcrumbJsonLd } from "@/components/seo/json-ld";
import type { APIRoomDetailResponse } from "@/lib/api-types";
import { NOINDEX } from "@/lib/seo/route-policy";
import { fetchSEO } from "@/lib/seo/fetch-seo";
import { linkPreview } from "@/lib/seo/link-preview";
import type { APIPrompt, APIRoomConnectResponse, APIRoomSEO } from "@/lib/api-types";
import { serverFetch } from "@/lib/server-fetch";

const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_URL || "https://api.solvr.dev";

// Rendered on every request, never from a stored copy: a room can turn private or be
// deleted at any moment and the API decides who may still read it, so a cached payload
// would keep showing the transcript to readers the API already refuses.
export const dynamic = "force-dynamic";

// Deduplicated server-side fetch — shared between generateMetadata and page component.
// React cache() ensures this runs only ONCE per request even if called twice.
// The SSR fetch carries no auth (there's no browser JWT during SSR), so a PRIVATE room
// returns 403 here — we surface the status so the page can hand off to a client-side
// authenticated gate instead of 404ing the owner (BART-156).
// An API failure (5xx) or an unreachable API throws, a retryable 5xx (task idx 83): it
// is neither a private room (the gate) nor a missing one (a real 404).
const getRoom = cache((slug: string): Promise<{ status: number; data: unknown }> =>
  readForPage<unknown>(`/v1/rooms/${encodeURIComponent(slug)}`)
);

// What the page of a room the server may not read is called. It says nothing of the room.
const PRIVATE_ROOM_TITLE = "Private room";

// The page's search verdict (task idx 80): the API decides it at its own endpoint. A
// refusal answers null (rendered as noindex); an API failure throws, a retryable 5xx
// like the room read above, never a false "not indexable" (lib/seo/fetch-seo.ts).
const getRoomSEO = cache((slug: string) =>
  fetchSEO<APIRoomSEO>(`/v1/rooms/${encodeURIComponent(slug)}/seo`)
);

// The room's published outcome posts, for the archive links. A failure leaves the
// links out; it never takes the live room down.
const getOutcomes = cache(async (slug: string): Promise<{ id: string; title: string }[]> => {
  try {
    const res = await serverFetch(`${API_BASE_URL}/v1/rooms/${encodeURIComponent(slug)}/posts`, {
      cache: "no-store",
    });
    if (!res.ok) return [];
    const json = await res.json();
    return (json.data ?? []).map((p: { id: string; title: string }) => ({ id: p.id, title: p.title }));
  } catch {
    return [];
  }
});

// The room's join sentence for a collaborator (SPEC.md 18.3), so an agent given only the
// room's URL finds in the HTML how to join it. A failure leaves the block out; it never takes
// the room down.
const getJoinPrompt = cache(async (slug: string): Promise<APIPrompt | null> => {
  try {
    const res = await serverFetch(`${API_BASE_URL}/v1/rooms/${encodeURIComponent(slug)}/connect?role=collaborator`, {
      cache: "no-store",
    });
    if (!res.ok) return null;
    const json = (await res.json()) as APIRoomConnectResponse | null;
    return json?.data?.prompt?.text ? json.data.prompt : null;
  } catch {
    return null;
  }
});

export async function generateMetadata({
  params,
}: {
  params: Promise<{ slug: string }>;
}): Promise<Metadata> {
  const { slug } = await params;
  const { status, data } = await getRoom(slug);
  const payload = data as APIRoomDetailResponse | null;
  if (!payload?.data?.room) {
    // A room the API does not have is a real 404; the not-found page names itself.
    if (status === 404) return { robots: NOINDEX };
    // A private room (403 to the server), or one the server could not read, is never
    // indexed; no private detail goes into its metadata. The gate names itself: with no
    // title it showed the home page's, in the tab and in a pasted link.
    return { title: PRIVATE_ROOM_TITLE, robots: NOINDEX, ...linkPreview({ title: PRIVATE_ROOM_TITLE }) };
  }
  const seo = await getRoomSEO(slug);
  // The API decides whether the page may be indexed, its title and its description
  // (task idx 80); the page only renders that. Its title is the whole <title> (SPEC.md
  // 27.3): at most 60 characters leading with the problem, with no site name after it.
  const title = seo ? { absolute: seo.title } : payload.data.room.display_name;
  const description = seo?.description;
  const path = `/rooms/${slug}`;
  return {
    title,
    description,
    alternates: { canonical: path },
    robots: seo?.indexable ? undefined : NOINDEX,
    // The link preview repeats that title and description (lib/seo/link-preview.ts).
    ...linkPreview({ title, description, path }),
  };
}

export default async function RoomDetailPage({
  params,
}: {
  params: Promise<{ slug: string }>;
}) {
  const { slug } = await params;
  const { status, data } = await getRoom(slug);
  const payload = data as APIRoomDetailResponse | null;

  // A genuinely missing room 404s (backend returns 404 only for an unknown slug);
  // Googlebot gets a real 404, not a 200 spinner.
  if (status === 404) notFound();

  // Private room (SSR got 403 with no JWT) or another refusal: render the client-side
  // authenticated gate, which re-fetches with the human's JWT (BART-156). No private-room
  // data is ever server-rendered, so private rooms stay unindexed. API failures threw above.
  if (!payload?.data?.room) {
    return (
      <div className="min-h-screen flex flex-col bg-background">
        <Header />
        <main className="flex-1 flex flex-col min-h-0 pt-16">
          <div className={`${styles.page} flex-1 w-full px-4 sm:px-6 lg:px-12 pb-12`}>
            <PrivateRoomView slug={slug} />
          </div>
        </main>
      </div>
    );
  }

  const { room, agents, recent_messages, owner_display_name, connection_status, initial_task, latest_pinned, try_workflow_url, history } = payload.data;
  // A finished room has nothing to join, and a private one is never read by the server.
  const joinable = !room.is_private && room.archived_at == null;
  const [outcomes, seo, joinPrompt] = await Promise.all([
    getOutcomes(slug),
    getRoomSEO(slug),
    joinable ? getJoinPrompt(slug) : Promise.resolve(null),
  ]);
  const url = `https://solvr.dev/rooms/${slug}`;

  // API returns the recent window newest-first; RoomDetailClient re-orders it
  // oldest -> newest for conventional top-to-bottom reading and de-duplicates.
  const messages = recent_messages || [];

  return (
    <div className="min-h-screen flex flex-col bg-background">
      {/* Structured data from what the page shows (task idx 82): the API's description,
          and the opening message decides whether a Person led the discussion. */}
      <JsonLd
        data={roomJsonLd({ room, url, description: seo?.description, firstMessage: initial_task })}
      />
      <JsonLd data={breadcrumbJsonLd([{ name: "Rooms", path: "/rooms" }, { name: room.display_name, path: `/rooms/${slug}` }])} />
      <Header />
      <main className="flex-1 flex flex-col min-h-0 pt-16">
        <div className="flex-1 min-w-0 w-full px-4 sm:px-6 lg:px-12 pb-12">
          <RoomDetailClient
            room={room}
            initialMessages={messages}
            initialAgents={agents || []}
            ownerDisplayName={owner_display_name}
            connectionStatus={connection_status}
            initialTask={initial_task}
            latestPinned={latest_pinned}
            tryWorkflowUrl={try_workflow_url}
            archive={<RoomArchiveNav slug={slug} history={history} outcomes={outcomes} />}
            agentJoin={joinPrompt ? <AgentJoinBlock prompt={joinPrompt} /> : undefined}
          />
        </div>
      </main>
    </div>
  );
}
