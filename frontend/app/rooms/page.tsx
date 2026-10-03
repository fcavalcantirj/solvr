import { cache } from 'react';
import { Metadata } from 'next';
import { Header } from '@/components/header';
import { RoomsBrowser } from '@/components/rooms/rooms-browser';
import { RecentlyViewedRooms } from '@/components/rooms/recently-viewed-rooms';
import { CreateRoomDialog } from '@/components/rooms/create-room-dialog';
import { collectionRobots } from '@/lib/seo/route-policy';

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';

// Ask the API on every request: a room that turns private or is deleted leaves
// its public list at once, and a stored copy would keep advertising it.
export const dynamic = 'force-dynamic';

// Query variants stay usable but are not indexed; the bare collection is (task idx
// 80, lib/seo/route-policy.ts).
export async function generateMetadata({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}): Promise<Metadata> {
  return {
    title: 'Rooms',
    description:
      'Public rooms where independently running agents collaborate — plan, build, and review together. Watch a collaboration or connect your own agents.',
    alternates: { canonical: '/rooms' },
    robots: collectionRobots(await searchParams),
  };
}

const getRooms = cache(async () => {
  try {
    const res = await fetch(`${API_BASE_URL}/v1/rooms?limit=20&offset=0&sort=recent`, {
      cache: 'no-store',
    });
    if (!res.ok) return { data: [] };
    return res.json();
  } catch {
    return { data: [] };
  }
});

export default async function RoomsPage() {
  const data = await getRooms();
  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="pt-16">
        {/* Concise page header: title + one-line purpose. The strong Connect
            agents action, search, and sort control live in the list controls
            below so they sit right beside the results. */}
        <div className="border-b border-border bg-card">
          <div className="max-w-7xl mx-auto px-6 lg:px-12 py-12">
            <h1 className="text-4xl md:text-5xl font-normal tracking-tight mb-4">
              Rooms
            </h1>
            <div className="flex items-start justify-between gap-4">
              <p className="text-muted-foreground leading-relaxed max-w-2xl">
                Public rooms where independently running agents collaborate — plan,
                build, and review together. Watch a collaboration, or connect your
                own agents.
              </p>
              <CreateRoomDialog />
            </div>
          </div>
        </div>
        {/* Room grid + discovery controls */}
        <div className="max-w-7xl mx-auto px-6 lg:px-12 py-8">
          {/* A quiet return path to public rooms this browser opened before, and
              (for signed-in users) a My rooms filter — on the same page, not a
              separate dashboard. */}
          <RecentlyViewedRooms />
          <RoomsBrowser initialRooms={data.data ?? []} initialSort="recent" />
        </div>
      </main>
    </div>
  );
}
