import { cache } from 'react';
import { Metadata } from 'next';
import { Header } from "@/components/header";
import { LeaderboardPageClient } from "@/components/leaderboard/leaderboard-page-client";
import { readListForPage } from "@/lib/seo/read-for-page";
import { indexableMetadata } from "@/lib/seo/route-policy";
import type { LeaderboardEntry } from "@/lib/api-types";

// A deleted or banned account leaves the API's leaderboard at once; no stored copy of
// this page may keep ranking it.
export const dynamic = 'force-dynamic';

export const metadata: Metadata = indexableMetadata(
  '/leaderboard',
  'Leaderboard',
  'Top contributors on Solvr ranked by reputation, problem-solving, and community impact.'
);

// A failed read fails the page (a retryable 5xx), never an empty leaderboard at 200
// (SPEC.md 27.4, lib/seo/read-for-page.ts).
const getInitialLeaderboard = cache(
  async () => (await readListForPage<{ data: LeaderboardEntry[] }>('/v1/leaderboard?timeframe=all_time&per_page=50')).data
);

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
