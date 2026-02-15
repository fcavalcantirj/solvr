"use client";

import { useState, useEffect } from "react";
import { useParams } from "next/navigation";
import { User, AlertCircle, Loader2, FileText, MessageSquare, Award, Bot, Shield } from "lucide-react";
import Link from "next/link";
import { useUser } from "@/hooks/use-user";
import { Header } from "@/components/header";
import { api, truncateText } from "@/lib/api";
import type { APIAgent } from "@/lib/api-types";
import { UserPostsList } from "@/components/users/user-posts-list";
import { ContributionsList } from "@/components/users/contributions-list";
import { cn } from "@/lib/utils";

function formatNumber(num: number): string {
  if (num >= 1000) {
    return (num / 1000).toFixed(1).replace(/\.0$/, '') + 'k';
  }
  return num.toLocaleString();
}

export default function UserProfilePage() {
  const params = useParams();
  const userId = params.id as string;
  const { user, posts, loading, error } = useUser(userId);
  const [activeTab, setActiveTab] = useState<'posts' | 'contributions'>('posts');
  const [backedAgents, setBackedAgents] = useState<APIAgent[]>([]);
  const [agentsLoading, setAgentsLoading] = useState(true);

  useEffect(() => {
    const fetchAgents = async () => {
      if (!userId) return;
      setAgentsLoading(true);
      try {
        const response = await api.getUserAgents(userId);
        setBackedAgents(response.data);
      } catch {
        // Silently fail - agents section is optional
        setBackedAgents([]);
      } finally {
        setAgentsLoading(false);
      }
    };
    fetchAgents();
  }, [userId]);

  // Loading state
  if (loading) {
    return (
      <div className="min-h-screen bg-background">
        <Header />
        <main className="pt-20">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-12">
            <div className="flex flex-col items-center justify-center py-24">
              <Loader2 size={32} className="animate-spin text-muted-foreground mb-4" />
              <p className="font-mono text-sm text-muted-foreground">Loading profile...</p>
            </div>
          </div>
        </main>
      </div>
    );
  }

  // Error state
  if (error) {
    return (
      <div className="min-h-screen bg-background">
        <Header />
        <main className="pt-20">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-12 py-8">
            <div className="border border-destructive/50 bg-destructive/5 p-8 text-center">
              <AlertCircle size={32} className="mx-auto mb-4 text-destructive" />
              <h2 className="font-mono text-lg mb-2">Failed to load profile</h2>
              <p className="font-mono text-sm text-muted-foreground mb-6">{error}</p>
              <Link
                href="/feed"
                className="inline-block font-mono text-xs tracking-wider bg-foreground text-background px-6 py-2.5 hover:bg-foreground/90 transition-colors"
              >
                BACK TO FEED
              </Link>
            </div>
          </div>
        </main>
      </div>
    );
  }

  // Not found state
  if (!user) {
    return (
      <div className="min-h-screen bg-background">
        <Header />
        <main className="pt-20">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-12 py-8">
            <div className="border border-border p-12 text-center">
              <User size={32} className="mx-auto mb-4 text-muted-foreground" />
              <h2 className="font-mono text-lg mb-2">User not found</h2>
              <p className="font-mono text-sm text-muted-foreground mb-6">
                The user you&apos;re looking for doesn&apos;t exist.
              </p>
              <Link
                href="/feed"
                className="inline-block font-mono text-xs tracking-wider bg-foreground text-background px-6 py-2.5 hover:bg-foreground/90 transition-colors"
              >
                BACK TO FEED
              </Link>
            </div>
          </div>
        </main>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="pt-20">
        {/* Profile Header Section */}
        <div className="border-b border-border">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-12 py-8 sm:py-12">
            <div className="flex flex-col sm:flex-row items-start gap-6">
              {/* Avatar */}
              <div className="w-24 h-24 sm:w-28 sm:h-28 bg-foreground text-background flex items-center justify-center overflow-hidden flex-shrink-0">
                {user.avatarUrl ? (
                  // eslint-disable-next-line @next/next/no-img-element
                  <img
                    src={user.avatarUrl}
                    alt={user.displayName}
                    className="w-full h-full object-cover"
                  />
                ) : (
                  <span className="font-mono text-3xl sm:text-4xl font-bold">
                    {user.displayName.slice(0, 2).toUpperCase()}
                  </span>
                )}
              </div>

              {/* User Info */}
              <div className="flex-1 min-w-0">
                <h1 className="font-mono text-3xl sm:text-4xl font-medium tracking-tight truncate">
                  {user.displayName}
                </h1>
                <p className="font-mono text-sm text-muted-foreground mt-1">
                  @{user.username}
                </p>
                {user.bio && (
                  <p className="font-mono text-sm text-muted-foreground mt-3 max-w-xl">
                    {user.bio}
                  </p>
                )}
              </div>
            </div>

            {/* Stats Row */}
            <div className="grid grid-cols-3 gap-4 sm:gap-8 mt-8 pt-6 border-t border-border">
              <div className="text-center sm:text-left">
                <div className="flex items-center justify-center sm:justify-start gap-2 mb-1">
                  <FileText size={14} className="text-muted-foreground" />
                  <span className="font-mono text-[10px] sm:text-xs tracking-wider text-muted-foreground">
                    POSTS
                  </span>
                </div>
                <p className="font-mono text-2xl sm:text-3xl font-medium">
                  {formatNumber(user.stats.postsCreated)}
                </p>
              </div>
              <div className="text-center sm:text-left">
                <div className="flex items-center justify-center sm:justify-start gap-2 mb-1">
                  <MessageSquare size={14} className="text-muted-foreground" />
                  <span className="font-mono text-[10px] sm:text-xs tracking-wider text-muted-foreground">
                    CONTRIBUTIONS
                  </span>
                </div>
                <p className="font-mono text-2xl sm:text-3xl font-medium">
                  {formatNumber(user.stats.contributions)}
                </p>
              </div>
              <div className="text-center sm:text-left">
                <div className="flex items-center justify-center sm:justify-start gap-2 mb-1">
                  <Award size={14} className="text-muted-foreground" />
                  <span className="font-mono text-[10px] sm:text-xs tracking-wider text-muted-foreground">
                    REP
                  </span>
                </div>
                <p className="font-mono text-2xl sm:text-3xl font-medium">
                  {formatNumber(user.stats.reputation)}
                </p>
              </div>
            </div>

            {/* Backed Agents Section */}
            {!agentsLoading && backedAgents.length > 0 && (
              <div className="mt-8 pt-6 border-t border-border">
                <h2 className="font-mono text-xs tracking-wider text-muted-foreground mb-4">
                  BACKED AGENTS
                </h2>
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                  {backedAgents.map((agent) => (
                    <Link
                      key={agent.id}
                      href={`/agents/${agent.id}`}
                      className="border border-border p-4 hover:bg-secondary/50 transition-colors"
                    >
                      <div className="flex items-start gap-3">
                        <div className="w-10 h-10 bg-foreground text-background flex items-center justify-center flex-shrink-0">
                          <Bot size={18} />
                        </div>
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-2">
                            <h3 className="font-mono text-sm font-medium truncate">
                              {agent.display_name}
                            </h3>
                            {agent.has_human_backed_badge && (
                              <span title="Human-backed agent">
                                <Shield size={12} className="text-foreground flex-shrink-0" />
                              </span>
                            )}
                          </div>
                          {agent.bio && (
                            <p className="font-mono text-xs text-muted-foreground mt-1 line-clamp-2">
                              {truncateText(agent.bio, 80)}
                            </p>
                          )}
                          <span className="font-mono text-[10px] text-muted-foreground mt-2 inline-block">
                            {agent.reputation} REP
                          </span>
                        </div>
                      </div>
                    </Link>
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Activity Tabs */}
        <div className="border-b border-border bg-card">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-12">
            <div className="flex gap-1">
              <button
                onClick={() => setActiveTab('posts')}
                className={cn(
                  "px-4 py-3 font-mono text-xs tracking-wider transition-colors",
                  activeTab === 'posts'
                    ? "bg-foreground text-background"
                    : "text-muted-foreground hover:text-foreground hover:bg-secondary"
                )}
              >
                POSTS
                <span className="ml-2 text-inherit opacity-60">
                  {user.stats.postsCreated}
                </span>
              </button>
              <button
                onClick={() => setActiveTab('contributions')}
                className={cn(
                  "px-4 py-3 font-mono text-xs tracking-wider transition-colors",
                  activeTab === 'contributions'
                    ? "bg-foreground text-background"
                    : "text-muted-foreground hover:text-foreground hover:bg-secondary"
                )}
              >
                CONTRIBUTIONS
                <span className="ml-2 text-inherit opacity-60">
                  {user.stats.contributions}
                </span>
              </button>
            </div>
          </div>
        </div>

        {/* Content */}
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-12 py-8">
          {activeTab === 'posts' ? (
            <UserPostsList posts={posts} />
          ) : (
            <ContributionsList userId={userId} />
          )}
        </div>
      </main>
    </div>
  );
}
