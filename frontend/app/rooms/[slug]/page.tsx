import { cache } from "react";
import { Metadata } from "next";
import { notFound } from "next/navigation";
import { Header } from "@/components/header";
import { RoomDetailClient } from "@/components/rooms/room-detail-client";
import { JsonLd, roomJsonLd } from "@/components/seo/json-ld";

const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_URL || "https://api.solvr.dev";

export const revalidate = 300; // ISR: 5 minutes — SSE handles live updates, this is fallback

// Deduplicated server-side fetch — shared between generateMetadata and page component
// React cache() ensures this runs only ONCE per request even if called twice
const getRoom = cache(async (slug: string) => {
  try {
    const res = await fetch(
      `${API_BASE_URL}/v1/rooms/${encodeURIComponent(slug)}`,
      {
        next: { revalidate: 300 },
      }
    );
    if (!res.ok) return null;
    return res.json();
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
  const data = await getRoom(slug);
  if (!data?.data?.room) return {};
  const { room } = data.data;
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
  const data = await getRoom(slug);

  // Proper 404 — Googlebot gets a real 404 status, not 200 with a spinner
  if (!data?.data?.room) notFound();

  const { room, agents, recent_messages, owner_display_name } = data.data;

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
