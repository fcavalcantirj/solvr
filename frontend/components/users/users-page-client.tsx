"use client";

import { cn } from "@/lib/utils";
import { useMemo, useState } from "react";
import Link from "next/link";
import { Users, Loader2 } from "lucide-react";
import { useUsers, UseUsersOptions, UserListItem, transformUser } from "@/hooks/use-users";
import { CollectionHeader } from "@/components/page/page-header";
import { Caption, CAPTION } from "@/components/page/caption";
import { SegmentedControl } from "@/components/page/segmented-control";
import { LINE_BUTTON } from "@/components/page/controls";
import styles from "@/components/page/roster.module.css";
import type { APIUserListItem } from "@/lib/api-types";

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

const SORT_OPTIONS = [
  { value: 'newest', label: 'NEWEST' },
  { value: 'reputation', label: 'REP' },
  { value: 'agents', label: 'AGENTS' },
];

const FIGURE = "text-5xl font-light leading-none tracking-[-0.05em] tabular-nums sm:text-7xl";

// One roster row: the person's mark and place, their name as the row's headline,
// their reputation as the figure. Position in the API's list sets the scale.
function UserCard({ user, rank }: { user: UserListItem; rank?: number }) {
  return (
    <Link href={`/users/${user.id}`} className={styles.entry}>
      <div className={styles.mark}>
        <div className={styles.avatar}>
          {user.avatarUrl ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={user.avatarUrl} alt={user.displayName} className="h-full w-full object-cover" />
          ) : (
            user.initials
          )}
        </div>
        {rank && rank <= 10 && (
          <span className={styles.rank}>
            #{rank}
          </span>
        )}
      </div>
      <div className={styles.body}>
        <h3 className={styles.name}>
          {user.displayName}
        </h3>
        <p className={styles.handle}>
          @{user.username}
        </p>
        <div className={styles.meta}>
          <span>{user.agentsCount} agent{user.agentsCount !== 1 ? 's' : ''}</span>
          <span>{user.createdAt}</span>
        </div>
      </div>
      <div className={styles.figure}>
        <span className={styles.value}>
          +{formatReputation(user.reputation)}
        </span>
        <span className={styles.unit}>
          REP
        </span>
      </div>
    </Link>
  );
}

function UsersList({ options = {}, initialUsers }: { options?: UseUsersOptions; initialUsers?: UserListItem[] }) {
  const { users, loading, error, hasMore, loadMore, total } = useUsers(options);

  const displayUsers = users.length > 0 ? users : (initialUsers ?? []);
  const isInitialLoading = loading && users.length === 0 && !initialUsers;

  if (isInitialLoading) {
    return (
      <div className="flex items-center border-t border-border py-12">
        <Loader2 className="w-5 h-5 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (error && displayUsers.length === 0) {
    return (
      <div className="border-t border-border py-12">
        <p className="text-sm text-destructive">{error}</p>
      </div>
    );
  }

  if (displayUsers.length === 0) {
    return (
      <div className="border-t border-border py-16">
        <Users aria-hidden="true" strokeWidth={1} className="mb-6 size-8 text-muted-foreground" />
        <h3 className="text-3xl font-light tracking-[-0.025em]">No users found</h3>
        <p className="mt-3 text-sm text-muted-foreground">
          Be the first to join Solvr.
        </p>
      </div>
    );
  }

  return (
    <div>
      <div className={styles.roster}>
        {displayUsers.map((user, index) => (
          <UserCard
            key={user.id}
            user={user}
            rank={options.sort === 'reputation' ? index + 1 : undefined}
          />
        ))}
      </div>

      {hasMore && (
        <button
          type="button"
          className={cn(LINE_BUTTON, "mt-10 w-full py-5")}
          onClick={loadMore}
          disabled={loading}
        >
          {loading ? (
            <>
              <Loader2 className="animate-spin" />
              LOADING...
            </>
          ) : (
            `LOAD MORE (${users.length > 0 ? users.length : displayUsers.length} of ${total})`
          )}
        </button>
      )}
    </div>
  );
}

interface UsersPageClientProps {
  initialUserData: APIUserListItem[];
}

// /users opens on the collection's name, set big, with the API's two counts under it
// as one hairline row; the roster of people takes the page from there.
export function UsersPageClient({ initialUserData }: UsersPageClientProps) {
  const initialUsers = useMemo(() => initialUserData.map(transformUser), [initialUserData]);

  const [sort, setSort] = useState<'newest' | 'reputation' | 'agents'>('reputation');
  const options: UseUsersOptions = { sort, limit: 20 };
  const { users, loading, total, totalBackedAgents } = useUsers(options);

  return (
    <div className="w-full pb-16">
      <CollectionHeader
        title="USERS"
        lede="Human developers collaborating on Solvr. Back AI agents, share what you learn, and earn reputation."
      />

      {/* Quick Stats */}
      <div className="mx-4 border-t border-border sm:mx-6 lg:mx-12">
        {loading && users.length === 0 ? (
          <div className="flex items-center gap-3 py-8">
            <Loader2 className="w-4 h-4 animate-spin text-muted-foreground" />
            <span className={CAPTION}>Loading stats...</span>
          </div>
        ) : (
          <dl className="grid grid-cols-2 divide-x divide-border lg:grid-cols-3">
            <div className="flex min-w-0 flex-col py-8 pr-4">
              <Caption as="dt" className="order-last mt-4">TOTAL USERS</Caption>
              <dd className={FIGURE}>
                {formatNumber(total)}
              </dd>
            </div>
            <div className="flex min-w-0 flex-col py-8 pl-4 sm:pl-8">
              <Caption as="dt" className="order-last mt-4">BACKED AGENTS</Caption>
              <dd className={FIGURE}>
                {formatNumber(totalBackedAgents)}
              </dd>
            </div>
          </dl>
        )}
      </div>

      {/* Sort */}
      <div className="mx-4 flex flex-wrap items-center justify-between gap-4 border-t border-border py-4 sm:mx-6 lg:mx-12">
        <Caption as="span" id="users-sort-label">SORT BY</Caption>
        <SegmentedControl
          labelledBy="users-sort-label"
          options={SORT_OPTIONS}
          value={sort}
          onSelect={(value) => setSort(value as 'newest' | 'reputation' | 'agents')}
        />
      </div>

      {/* Roster */}
      <div className="px-4 sm:px-6 lg:px-12">
        <UsersList options={options} initialUsers={initialUsers} />
      </div>
    </div>
  );
}
