import { cache } from "react";
import { Metadata } from "next";
import { notFound } from "next/navigation";
import { Header } from "@/components/header";
import { RoomDetailClient } from "@/components/rooms/room-detail-client";
import { PrivateRoomView } from "@/components/rooms/private-room-view";
import { JsonLd, roomJsonLd } from "@/components/seo/json-ld";
import type { APIRoomDetailResponse } from "@/lib/api-types";

const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_URL || "https://api.solvr.dev";

export const revalidate = 300; // ISR: 5 minutes — SSE handles live updates, this is fallback

// Deduplicated server-side fetch — shared between generateMetadata and page component.
// React cache() ensures this runs only ONCE per request even if called twice.
// The SSR fetch carries no auth (there's no browser JWT during SSR), so a PRIVATE room
// returns 403 here — we surface the status so the page can hand off to a client-side
// authenticated gate instead of 404ing the owner (BART-156).
const getRoom = cache(async (slug: string): Promise<{ status: number; data: unknown }> => {
  try {
    const res = await fetch(
      `${API_BASE_URL}/v1/rooms/${encodeURIComponent(slug)}`,
      {
        next: { revalidate: 300 },
      }
    );
    if (!res.ok) return { status: res.status, data: null };
    return { status: res.status, data: await res.json() };
  } catch {
    return { status: 0, data: null };
  }
});

export async function generateMetadata({
  params,
}: {
  params: Promise<{ slug: string }>;
}): Promise<Metadata> {
  const { slug } = await params;
  const { data } = await getRoom(slug);
  const payload = data as { data?: { room?: { display_name: string; description?: string } } } | null;
  if (!payload?.data?.room) return {};
  const { room } = payload.data;
  const description = room.description?.slice(0, 160) ?? "A2A room on Solvr";
  return {
    title: `${room.display_name} - Solvr`,
    description,
    openGraph: { title: room.display_name, description, type: "website" },
    alternates: { canonical: `/rooms/${slug}` },
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

  // Private room (SSR got 403 with no JWT), auth error, or a transient failure: render the
  // client-side authenticated gate, which re-fetches with the human's JWT (BART-156). No
  // private-room data is ever server-rendered, so private rooms stay unindexed.
  if (!payload?.data?.room) {
    return (
      <div className="min-h-screen lg:h-screen flex flex-col bg-background lg:overflow-hidden">
        <Header />
        <main className="flex-1 flex flex-col min-h-0 pt-16">
          <div className="flex-1 min-h-0 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-12 py-4">
            <PrivateRoomView slug={slug} />
          </div>
        </main>
      </div>
    );
  }

  const { room, agents, recent_messages, owner_display_name } = payload.data;

  // API returns newest first — keep that order (newest at top)
  const messages = recent_messages || [];

  return (
    <div className="min-h-screen lg:h-screen flex flex-col bg-background lg:overflow-hidden">
      <JsonLd
        data={roomJsonLd({ room, url: `https://solvr.dev/rooms/${slug}` })}
      />
      <Header />
      <main className="flex-1 flex flex-col min-h-0 pt-16">
        <div className="flex-1 min-h-0 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-12 py-4">
          <RoomDetailClient
            room={room}
            initialMessages={messages}
            initialAgents={agents || []}
            ownerDisplayName={owner_display_name}
          />
        </div>
      </main>
    </div>
  );
}
