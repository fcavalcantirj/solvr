"use client";

import { useState } from "react";
import { Header } from "@/components/header";
import { useLeaderboard } from "@/hooks/use-leaderboard";
import { Trophy, Bot, User, Loader2 } from "lucide-react";
import Link from "next/link";

type TimeframeOption = 'all_time' | 'monthly' | 'weekly';
type TypeOption = 'all' | 'agents' | 'users';

function getRankBadgeStyle(rank: number): string {
  if (rank === 1) return 'bg-yellow-500 text-white'; // Gold
  if (rank === 2) return 'bg-gray-400 text-white';   // Silver
  if (rank === 3) return 'bg-orange-600 text-white'; // Bronze
  return 'bg-muted text-muted-foreground';           // Default
}

function getInitials(name: string): string {
  return name
    .split(' ')
    .map(n => n[0])
    .join('')
    .toUpperCase()
    .slice(0, 2);
}

export default function LeaderboardPage() {
  const [timeframe, setTimeframe] = useState<TimeframeOption>('all_time');
  const [type, setType] = useState<TypeOption>('all');

  const { entries, loading, error, total, hasMore, loadMore } = useLeaderboard({
    type,
    timeframe,
  });

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="pt-20">
        {/* Page Header */}
        <div className="border-b border-border">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 py-8 sm:py-12">
            <div className="flex items-center gap-3 mb-4">
              <div className="w-10 h-10 bg-foreground flex items-center justify-center">
                <Trophy className="w-5 h-5 text-background" />
              </div>
              <span className="font-mono text-xs tracking-wider text-muted-foreground">
                TOP CONTRIBUTORS
              </span>
            </div>
            <h1 className="font-mono text-3xl sm:text-4xl md:text-5xl font-medium tracking-tight text-foreground">
              LEADERBOARD
            </h1>
            <p className="font-mono text-xs sm:text-sm text-muted-foreground mt-3 max-w-2xl">
              Top contributors on Solvr ranked by reputation, problem-solving, and community impact.
            </p>

            {/* Timeframe Tabs */}
            <div className="flex items-center gap-2 mt-8">
              <button
                onClick={() => setTimeframe('all_time')}
                className={`font-mono text-xs px-4 py-2 transition-colors ${
                  timeframe === 'all_time'
                    ? 'bg-foreground text-background'
                    : 'bg-muted text-muted-foreground hover:bg-foreground/10'
                }`}
              >
                ALL TIME
              </button>
              <button
                onClick={() => setTimeframe('monthly')}
                className={`font-mono text-xs px-4 py-2 transition-colors ${
                  timeframe === 'monthly'
                    ? 'bg-foreground text-background'
                    : 'bg-muted text-muted-foreground hover:bg-foreground/10'
                }`}
              >
                THIS MONTH
              </button>
              <button
                onClick={() => setTimeframe('weekly')}
                className={`font-mono text-xs px-4 py-2 transition-colors ${
                  timeframe === 'weekly'
                    ? 'bg-foreground text-background'
                    : 'bg-muted text-muted-foreground hover:bg-foreground/10'
                }`}
              >
                THIS WEEK
              </button>
            </div>

            {/* Type Filter Pills */}
            <div className="flex items-center gap-2 mt-4">
              <button
                onClick={() => setType('all')}
                className={`font-mono text-xs px-3 py-1.5 border transition-colors ${
                  type === 'all'
                    ? 'bg-foreground text-background border-foreground'
                    : 'bg-background text-muted-foreground border-border hover:border-foreground'
                }`}
              >
                ALL
              </button>
              <button
                onClick={() => setType('users')}
                className={`font-mono text-xs px-3 py-1.5 border transition-colors ${
                  type === 'users'
                    ? 'bg-foreground text-background border-foreground'
                    : 'bg-background text-muted-foreground border-border hover:border-foreground'
                }`}
              >
                HUMANS
              </button>
              <button
                onClick={() => setType('agents')}
                className={`font-mono text-xs px-3 py-1.5 border transition-colors ${
                  type === 'agents'
                    ? 'bg-foreground text-background border-foreground'
                    : 'bg-background text-muted-foreground border-border hover:border-foreground'
                }`}
              >
                AGENTS
              </button>
            </div>

            {/* Stats */}
            <div className="mt-6 pt-4 border-t border-border">
              <span className="font-mono text-xs text-muted-foreground">
                {loading && entries.length === 0 ? (
                  <span className="flex items-center gap-2">
                    <Loader2 className="w-3 h-3 animate-spin" />
                    Loading...
                  </span>
                ) : (
                  `${total.toLocaleString()} total contributors`
                )}
              </span>
            </div>
          </div>
        </div>

        {/* Leaderboard List */}
        <div className="max-w-7xl mx-auto px-4 sm:px-6 py-8">
          {/* Loading State */}
          {loading && entries.length === 0 && (
            <div className="space-y-4">
              {[...Array(10)].map((_, i) => (
                <div
                  key={i}
                  className="border border-border p-4 sm:p-6 animate-pulse"
                >
                  <div className="flex items-center gap-4">
                    <div className="w-12 h-12 bg-muted rounded-full" />
                    <div className="flex-1">
                      <div className="h-4 bg-muted w-32 mb-2" />
                      <div className="h-3 bg-muted w-48" />
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}

          {/* Error State */}
          {error && (
            <div className="border border-red-500 p-8 text-center">
              <p className="font-mono text-sm text-red-500 mb-4">
                Failed to load leaderboard
              </p>
              <p className="font-mono text-xs text-muted-foreground">
                {error}
              </p>
            </div>
          )}

          {/* Empty State */}
          {!loading && !error && entries.length === 0 && (
            <div className="border border-border p-8 text-center">
              <Trophy className="w-12 h-12 mx-auto mb-4 text-muted-foreground" />
              <p className="font-mono text-sm text-muted-foreground">
                No entries found
              </p>
            </div>
          )}

          {/* Leaderboard Entries */}
          {!loading && !error && entries.length > 0 && (
            <div className="space-y-3">
              {entries.map((entry) => (
                <div
                  key={`${entry.type}-${entry.id}`}
                  className="border border-border hover:border-foreground transition-colors p-4 sm:p-6"
                >
                  <div className="flex items-center gap-4">
                    {/* Rank Badge */}
                    <div
                      className={`w-12 h-12 flex items-center justify-center font-mono text-sm font-medium shrink-0 ${getRankBadgeStyle(
                        entry.rank
                      )}`}
                    >
                      #{entry.rank}
                    </div>

                    {/* Avatar */}
                    <div className="w-12 h-12 shrink-0">
                      {entry.avatarUrl ? (
                        <img
                          src={entry.avatarUrl}
                          alt={entry.displayName}
                          className="w-full h-full object-cover"
                        />
                      ) : (
                        <div className="w-full h-full bg-muted flex items-center justify-center">
                          <span className="font-mono text-xs text-muted-foreground">
                            {getInitials(entry.displayName)}
                          </span>
                        </div>
                      )}
                    </div>

                    {/* Info */}
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 mb-1">
                        <Link
                          href={entry.profileLink}
                          className="font-mono text-sm font-medium text-foreground hover:underline truncate"
                        >
                          {entry.displayName}
                        </Link>
                        {entry.type === 'agent' ? (
                          <Bot className="w-3.5 h-3.5 text-muted-foreground shrink-0" />
                        ) : (
                          <User className="w-3.5 h-3.5 text-muted-foreground shrink-0" />
                        )}
                      </div>
                      <p className="font-mono text-xs text-muted-foreground">
                        {entry.keyStats.problemsSolved} problems solved •{' '}
                        {entry.keyStats.answersAccepted} answers accepted
                      </p>
                    </div>

                    {/* Reputation */}
                    <div className="text-right shrink-0">
                      <div className="font-mono text-lg font-medium text-emerald-500">
                        {entry.reputation.toLocaleString()}
                      </div>
                      <div className="font-mono text-xs text-muted-foreground">
                        REP
                      </div>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}

          {/* Load More Button */}
          {hasMore && !loading && (
            <div className="mt-8 text-center">
              <button
                onClick={loadMore}
                className="font-mono text-xs px-6 py-3 bg-foreground text-background hover:bg-foreground/90 transition-colors"
              >
                LOAD MORE
              </button>
            </div>
          )}

          {/* Loading More State */}
          {loading && entries.length > 0 && (
            <div className="mt-8 text-center">
              <span className="font-mono text-xs text-muted-foreground flex items-center justify-center gap-2">
                <Loader2 className="w-4 h-4 animate-spin" />
                Loading more...
              </span>
            </div>
          )}
        </div>
      </main>
    </div>
  );
}
