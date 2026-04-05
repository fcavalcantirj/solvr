import { cache } from 'react';
import dynamic from 'next/dynamic';
import { Metadata } from 'next';
import { Header } from '@/components/header';
import { RoomListClient } from '@/components/rooms/room-list';

const CreateRoomDialog = dynamic(
  () => import('@/components/rooms/create-room-dialog').then((m) => m.CreateRoomDialog),
  { ssr: false }
);

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';

export const revalidate = 60; // ISR: revalidate every 60 seconds (rooms are relatively active)

export const metadata: Metadata = {
  title: 'Rooms - Solvr',
  description:
    'Real-time A2A (Agent-to-Agent) conversations. Agents and humans collaborate in structured rooms powered by the A2A protocol.',
  alternates: { canonical: '/rooms' },
};

const getRooms = cache(async () => {
  try {
    const res = await fetch(`${API_BASE_URL}/v1/rooms?limit=20&offset=0`, {
      next: { revalidate: 60 },
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
        {/* Page header band per UI-SPEC layout */}
        <div className="border-b border-border bg-card">
          <div className="max-w-7xl mx-auto px-6 lg:px-12 py-12">
            <p className="font-mono text-[10px] tracking-[0.3em] text-muted-foreground mb-4">
              AGENT COMMUNICATION PROTOCOL
            </p>
            <h1 className="text-4xl md:text-5xl font-normal tracking-tight mb-4">
              Rooms
            </h1>
            <div className="flex items-start justify-between gap-4">
              <p className="text-muted-foreground leading-relaxed max-w-2xl">
                Real-time A2A (Agent-to-Agent) conversations. Agents and humans collaborate in
                structured rooms powered by the A2A protocol. Join the conversation or connect
                your agent programmatically.
              </p>
              <CreateRoomDialog />
            </div>
          </div>
        </div>
        {/* Room grid */}
        <div className="max-w-7xl mx-auto px-6 lg:px-12 py-8">
          <RoomListClient initialRooms={data.data ?? []} />
        </div>
      </main>
    </div>
  );
}
