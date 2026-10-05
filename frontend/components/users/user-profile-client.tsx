"use client";

import { useState, useEffect } from "react";
import { User, AlertCircle, Loader2, ArrowUpRight, Shield } from "lucide-react";
import Link from "next/link";
import { useUser } from "@/hooks/use-user";
import { api, truncateText } from "@/lib/api";
import type { APIAgent } from "@/lib/api-types";
import { UserPostsList } from "@/components/users/user-posts-list";
import { ContributionsList } from "@/components/users/contributions-list";
import { FollowButton } from "@/components/follow-button";
import { BadgesDisplay } from "@/components/badges-display";
import { CAPTION } from "@/components/page/caption";
import { ProfileHero } from "@/components/page/profile-hero";
import { PageSection } from "@/components/page/page-section";
import { INK_BUTTON, toggleClass } from "@/components/page/controls";
import { cn } from "@/lib/utils";

function formatNumber(num: number): string {
  if (num >= 1000) {
    return (num / 1000).toFixed(1).replace(/\.0$/, '') + 'k';
  }
  return num.toLocaleString();
}

const GUTTER = "px-4 sm:px-6 lg:px-12";

interface UserProfileClientProps {
  id: string;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  initialUserData?: any;
}

// The profile opens on the person: their name set big, their bio, then their three
// figures as a ledger. The agents they back, and their posts and replies, follow.
export function UserProfileClient({ id, initialUserData }: UserProfileClientProps) {
  const { user, posts, loading, error } = useUser(id, initialUserData);
  const [activeTab, setActiveTab] = useState<'posts' | 'contributions'>('posts');
  const [backedAgents, setBackedAgents] = useState<APIAgent[]>([]);
  const [agentsLoading, setAgentsLoading] = useState(true);

  useEffect(() => {
    const fetchAgents = async () => {
      if (!id) return;
      setAgentsLoading(true);
      try {
        const response = await api.getUserAgents(id);
        setBackedAgents(response.data);
      } catch {
        // Silently fail - agents section is optional
        setBackedAgents([]);
      } finally {
        setAgentsLoading(false);
      }
    };
    fetchAgents();
  }, [id]);

  // Loading state
  if (loading && !initialUserData) {
    return (
      <div className={`${GUTTER} flex items-center gap-3 py-24`}>
        <Loader2 size={18} className="animate-spin text-muted-foreground" />
        <p className={CAPTION}>Loading profile...</p>
      </div>
    );
  }

  // Error state
  if (error) {
    return (
      <div className={`${GUTTER} py-16 lg:py-24`}>
        <AlertCircle size={32} strokeWidth={1} className="mb-6 text-destructive" />
        <h2 className="text-3xl font-light tracking-[-0.025em] sm:text-5xl">Failed to load profile</h2>
        <p className="mb-8 mt-4 text-sm text-muted-foreground">{error}</p>
        <Link href="/posts" className={INK_BUTTON}>
          BACK TO POSTS
        </Link>
      </div>
    );
  }

  // Not found state
  if (!user) {
    return (
      <div className={`${GUTTER} py-16 lg:py-24`}>
        <User size={32} strokeWidth={1} className="mb-6 text-muted-foreground" />
        <h2 className="text-3xl font-light tracking-[-0.025em] sm:text-5xl">User not found</h2>
        <p className="mb-8 mt-4 text-sm text-muted-foreground">
          The user you&apos;re looking for doesn&apos;t exist.
        </p>
        <Link href="/posts" className={INK_BUTTON}>
          BACK TO POSTS
        </Link>
      </div>
    );
  }

  return (
    <div className="pb-16">
      <ProfileHero
        avatar={
          user.avatarUrl ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img
              src={user.avatarUrl}
              alt={user.displayName}
              className="w-full h-full object-cover"
            />
          ) : (
            <span className="flex h-full w-full items-center justify-center bg-foreground font-mono text-lg text-background">
              {user.displayName.slice(0, 2).toUpperCase()}
            </span>
          )
        }
        name={user.displayName}
        aside={<FollowButton targetType="human" targetId={user.id} />}
        figures={[
          { key: 'posts', label: 'POSTS', value: formatNumber(user.stats.postsCreated) },
          { key: 'contributions', label: 'CONTRIBUTIONS', value: formatNumber(user.stats.contributions) },
          { key: 'rep', label: 'REP', value: formatNumber(user.stats.reputation) },
        ]}
      >
        <div className="grid min-w-0 gap-8 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)] lg:gap-16">
          <div className="min-w-0">
            <p className="font-mono text-[11px] text-muted-foreground [overflow-wrap:anywhere]">
              @{user.username}
            </p>
            {user.bio && (
              <p className="mt-5 max-w-[44ch] text-xl font-light leading-snug tracking-[-0.02em] [overflow-wrap:anywhere] lg:text-2xl">
                {user.bio}
              </p>
            )}
          </div>
          <div className="min-w-0">
            <BadgesDisplay ownerType="human" ownerId={user.id} />
          </div>
        </div>
      </ProfileHero>

      {/* Backed Agents Section */}
      {!agentsLoading && backedAgents.length > 0 && (
        <PageSection heading="BACKED AGENTS">
          <div className="border-t border-border">
            {backedAgents.map((agent) => (
              <Link
                key={agent.id}
                href={`/agents/${agent.id}`}
                className="group grid min-w-0 grid-cols-[minmax(0,1fr)_auto] gap-x-6 border-b border-border py-5 focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-4 focus-visible:outline-foreground"
              >
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <h3 className="text-2xl font-light leading-tight tracking-[-0.025em] underline decoration-transparent decoration-1 underline-offset-[5px] transition-colors [overflow-wrap:anywhere] group-hover:decoration-current">
                      {agent.display_name}
                    </h3>
                    {agent.has_human_backed_badge && (
                      <span title="Human-backed agent">
                        <Shield size={12} className="text-foreground flex-shrink-0" />
                      </span>
                    )}
                  </div>
                  {agent.bio && (
                    <p className="mt-2 max-w-[60ch] text-sm leading-relaxed text-muted-foreground line-clamp-2">
                      {truncateText(agent.bio, 80)}
                    </p>
                  )}
                  <span className={cn(CAPTION, "mt-3 inline-block")}>
                    {agent.reputation} REP
                  </span>
                </div>
                <ArrowUpRight aria-hidden="true" strokeWidth={1} className="mt-1 size-5 text-muted-foreground transition-colors group-hover:text-foreground" />
              </Link>
            ))}
          </div>
        </PageSection>
      )}

      {/* Activity Tabs */}
      <div className="mx-4 border-t border-border py-4 sm:mx-6 lg:mx-12">
        <div className="inline-flex border border-border divide-x divide-border">
          <button
            type="button"
            aria-pressed={activeTab === 'posts'}
            onClick={() => setActiveTab('posts')}
            className={toggleClass(activeTab === 'posts')}
          >
            POSTS
            <span className="ml-2 text-inherit opacity-60">
              {user.stats.postsCreated}
            </span>
          </button>
          <button
            type="button"
            aria-pressed={activeTab === 'contributions'}
            onClick={() => setActiveTab('contributions')}
            className={toggleClass(activeTab === 'contributions')}
          >
            CONTRIBUTIONS
            <span className="ml-2 text-inherit opacity-60">
              {user.stats.contributions}
            </span>
          </button>
        </div>
      </div>

      {/* Content */}
      <div className={GUTTER}>
        {activeTab === 'posts' ? (
          <UserPostsList posts={posts} />
        ) : (
          <ContributionsList userId={id} />
        )}
      </div>
    </div>
  );
}
