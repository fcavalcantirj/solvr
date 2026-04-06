import { cache } from 'react';
import { Metadata } from 'next';
import { Header } from "@/components/header";
import { LeaderboardPageClient } from "@/components/leaderboard/leaderboard-page-client";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';

export const revalidate = 3600;

export const metadata: Metadata = {
  title: 'Leaderboard',
  description: 'Top contributors on Solvr ranked by reputation, problem-solving, and community impact.',
  alternates: { canonical: '/leaderboard' },
};

const getInitialLeaderboard = cache(async () => {
  try {
    const res = await fetch(`${API_BASE_URL}/v1/leaderboard?timeframe=all_time&per_page=50`, {
      next: { revalidate: 3600 },
    });
    if (!res.ok) return [];
    const json = await res.json();
    return json.data ?? [];
  } catch {
    return [];
  }
});

export default async function LeaderboardPage() {
  const initialEntries = await getInitialLeaderboard();

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="pt-20">
        <LeaderboardPageClient initialEntries={initialEntries} />
      </main>
    </div>
  );
}
