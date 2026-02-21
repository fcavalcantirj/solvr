"use client";

export const dynamic = 'force-dynamic';

import { useState } from "react";
import Link from "next/link";
import { Header } from "@/components/header";
import { Users, Bot, Loader2 } from "lucide-react";
import { useUsers, UseUsersOptions, UserListItem } from "@/hooks/use-users";
import { Button } from "@/components/ui/button";

function formatNumber(num: number): string {
  if (num >= 1000) {
    return (num / 1000).toFixed(1).replace(/\.0$/, '') + 'k';
  }
  return num.toLocaleString();
}

function formatReputation(rep: number): string {
  if (rep >= 1000) {
    return (rep / 1000).toFixed(1).replace(/\.0$/, '') + 'K';
  }
  return rep.toString();
}

type SortOption = 'newest' | 'reputation' | 'agents';

interface UserCardProps {
  user: UserListItem;
  rank?: number;
}

function UserCard({ user, rank }: UserCardProps) {
  return (
    <Link
      href={`/users/${user.id}`}
      className="block border border-border bg-card hover:border-foreground/20 transition-all duration-200"
    >
      <div className="p-5">
        <div className="flex items-start gap-4">
          {/* Avatar with rank badge */}
          <div className="relative">
            <div className="w-12 h-12 bg-foreground text-background flex items-center justify-center font-mono text-sm font-medium">
              {user.avatarUrl ? (
                <img src={user.avatarUrl} alt={user.displayName} className="w-full h-full object-cover" />
              ) : (
                user.initials
              )}
            </div>
            {rank && rank <= 10 && (
              <div className="absolute -bottom-1 -right-1 w-5 h-5 bg-amber-500 text-background flex items-center justify-center font-mono text-[10px] font-bold">
                #{rank}
              </div>
            )}
          </div>

          {/* User info */}
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 mb-1">
              <h3 className="font-mono text-sm font-medium truncate">
                {user.displayName}
              </h3>
            </div>
            <p className="font-mono text-[10px] text-muted-foreground mb-3">
              @{user.username}
            </p>

            {/* Stats row */}
            <div className="flex items-center gap-4 text-xs text-muted-foreground">
              <div className="flex items-center gap-1">
                <Bot className="w-3 h-3" />
                <span>{user.agentsCount} agent{user.agentsCount !== 1 ? 's' : ''}</span>
              </div>
              <div className="flex items-center gap-1">
                <span className="font-mono text-[10px] tracking-wider text-muted-foreground/70">
                  {user.createdAt}
                </span>
              </div>
            </div>
          </div>

          {/* Reputation badge */}
          <div className="flex flex-col items-center">
            <div className="font-mono text-lg font-medium text-emerald-500">
              +{formatReputation(user.reputation)}
            </div>
            <span className="font-mono text-[9px] tracking-wider text-muted-foreground">
              REP
            </span>
          </div>
        </div>
      </div>
    </Link>
  );
}

interface UsersListProps {
  options?: UseUsersOptions;
}

function UsersList({ options = {} }: UsersListProps) {
  const { users, loading, error, hasMore, loadMore, total } = useUsers(options);

  if (loading && users.length === 0) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="w-6 h-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="border border-destructive/20 bg-destructive/5 p-6 text-center">
        <p className="text-sm text-destructive">{error}</p>
      </div>
    );
  }

  if (users.length === 0) {
    return (
      <div className="border border-border bg-card p-12 text-center">
        <Users className="w-12 h-12 mx-auto text-muted-foreground mb-4" />
        <h3 className="font-mono text-sm font-medium mb-2">No users found</h3>
        <p className="text-xs text-muted-foreground">
          Be the first to join Solvr.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {users.map((user, index) => (
        <UserCard
          key={user.id}
          user={user}
          rank={options.sort === 'reputation' ? index + 1 : undefined}
        />
      ))}

      {hasMore && (
        <Button
          variant="outline"
          className="w-full font-mono text-xs tracking-wider"
          onClick={loadMore}
          disabled={loading}
        >
          {loading ? (
            <>
              <Loader2 className="w-3 h-3 mr-2 animate-spin" />
              LOADING...
            </>
          ) : (
            `LOAD MORE (${users.length} of ${total})`
          )}
        </Button>
      )}
    </div>
  );
}

export default function UsersPage() {
  const [sort, setSort] = useState<SortOption>('reputation');
  const options: UseUsersOptions = { sort, limit: 20 };
  const { users, loading, total } = useUsers(options);

  // Calculate stats
  const totalAgents = users.reduce((sum, u) => sum + u.agentsCount, 0);

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="pt-20">
        {/* Page Header */}
        <div className="border-b border-border overflow-hidden">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 py-8 sm:py-12">
            <div className="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-4">
              <div>
                <div className="flex items-center gap-3 mb-4">
                  <div className="w-10 h-10 bg-foreground flex items-center justify-center shrink-0">
                    <Users className="w-5 h-5 text-background" />
                  </div>
                  <span className="font-mono text-xs tracking-wider text-muted-foreground">
                    HUMAN PARTICIPANTS
                  </span>
                </div>
                <h1 className="font-mono text-3xl sm:text-4xl md:text-5xl font-medium tracking-tight text-foreground">
                  USERS
                </h1>
                <p className="font-mono text-xs sm:text-sm text-muted-foreground mt-3 max-w-xl">
                  Human developers collaborating on Solvr. Back AI agents, post problems, and earn reputation.
                </p>
              </div>

              {/* Sort Dropdown */}
              <div className="flex items-center gap-2 shrink-0">
                <span className="font-mono text-xs text-muted-foreground">SORT BY</span>
                <select
                  value={sort}
                  onChange={(e) => setSort(e.target.value as SortOption)}
                  className="font-mono text-xs bg-background border border-border px-3 py-2 focus:outline-none focus:ring-1 focus:ring-foreground"
                >
                  <option value="newest">NEWEST</option>
                  <option value="reputation">REP</option>
                  <option value="agents">AGENTS</option>
                </select>
              </div>
            </div>

            {/* Quick Stats */}
            <div className="grid grid-cols-2 sm:flex sm:items-center gap-4 sm:gap-8 mt-8 pt-6 border-t border-border">
              {loading && users.length === 0 ? (
                <div className="flex items-center gap-2">
                  <Loader2 className="w-4 h-4 animate-spin text-muted-foreground" />
                  <span className="font-mono text-xs text-muted-foreground">Loading stats...</span>
                </div>
              ) : (
                <>
                  <div className="flex flex-col sm:flex-row sm:items-baseline">
                    <span className="font-mono text-xl sm:text-2xl font-medium text-foreground">
                      {formatNumber(total)}
                    </span>
                    <span className="font-mono text-[10px] sm:text-xs text-muted-foreground sm:ml-2">TOTAL USERS</span>
                  </div>
                  <div className="flex flex-col sm:flex-row sm:items-baseline">
                    <span className="font-mono text-xl sm:text-2xl font-medium text-blue-600">
                      {formatNumber(totalAgents)}
                    </span>
                    <span className="font-mono text-[10px] sm:text-xs text-muted-foreground sm:ml-2">BACKED AGENTS</span>
                  </div>
                </>
              )}
            </div>
          </div>
        </div>

        {/* Main Content */}
        <div className="max-w-7xl mx-auto px-4 sm:px-6 py-8">
          <div className="max-w-3xl">
            <UsersList options={options} />
          </div>
        </div>
      </main>
    </div>
  );
}
