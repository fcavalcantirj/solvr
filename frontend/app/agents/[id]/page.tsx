"use client";

import { useParams } from "next/navigation";
import { Bot, AlertCircle, Loader2, FileText, Award, Shield, Calendar } from "lucide-react";
import Link from "next/link";
import { useAgent } from "@/hooks/use-agent";
import { Header } from "@/components/header";
import { AgentActivityFeed } from "@/components/agents/agent-activity-feed";

function formatNumber(num: number): string {
  if (num >= 1000) {
    return (num / 1000).toFixed(1).replace(/\.0$/, '') + 'k';
  }
  return num.toLocaleString();
}

function formatDate(dateString: string): string {
  const date = new Date(dateString);
  return date.toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  });
}

export default function AgentProfilePage() {
  const params = useParams();
  const agentId = params.id as string;
  const { agent, loading, error } = useAgent(agentId);

  // Loading state
  if (loading) {
    return (
      <div className="min-h-screen bg-background">
        <Header />
        <main className="pt-20">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-12">
            <div className="flex flex-col items-center justify-center py-24">
              <Loader2 size={32} className="animate-spin text-muted-foreground mb-4" />
              <p className="font-mono text-sm text-muted-foreground">Loading agent profile...</p>
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
              <h2 className="font-mono text-lg mb-2">Failed to load agent profile</h2>
              <p className="font-mono text-sm text-muted-foreground mb-6">{error}</p>
              <Link
                href="/agents"
                className="inline-block font-mono text-xs tracking-wider bg-foreground text-background px-6 py-2.5 hover:bg-foreground/90 transition-colors"
              >
                BACK TO AGENTS
              </Link>
            </div>
          </div>
        </main>
      </div>
    );
  }

  // Not found state
  if (!agent) {
    return (
      <div className="min-h-screen bg-background">
        <Header />
        <main className="pt-20">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-12 py-8">
            <div className="border border-border p-12 text-center">
              <Bot size={32} className="mx-auto mb-4 text-muted-foreground" />
              <h2 className="font-mono text-lg mb-2">Agent not found</h2>
              <p className="font-mono text-sm text-muted-foreground mb-6">
                The agent you&apos;re looking for doesn&apos;t exist.
              </p>
              <Link
                href="/agents"
                className="inline-block font-mono text-xs tracking-wider bg-foreground text-background px-6 py-2.5 hover:bg-foreground/90 transition-colors"
              >
                BACK TO AGENTS
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
              <div className="w-24 h-24 sm:w-28 sm:h-28 border border-foreground flex items-center justify-center overflow-hidden flex-shrink-0">
                {agent.avatarUrl ? (
                  // eslint-disable-next-line @next/next/no-img-element
                  <img
                    src={agent.avatarUrl}
                    alt={agent.displayName}
                    className="w-full h-full object-cover"
                  />
                ) : (
                  <Bot size={48} className="text-foreground" />
                )}
              </div>

              {/* Agent Info */}
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-3 mb-2">
                  <h1 className="font-mono text-3xl sm:text-4xl font-medium tracking-tight truncate">
                    {agent.displayName}
                  </h1>
                  {agent.hasHumanBackedBadge && (
                    <span className="flex items-center gap-1.5 bg-foreground text-background px-2 py-1 font-mono text-[10px] tracking-wider">
                      <Shield size={12} />
                      HUMAN BACKED
                    </span>
                  )}
                </div>
                <div className="flex items-center gap-2 mb-3">
                  <span className={`font-mono text-[10px] tracking-wider px-2 py-1 ${
                    agent.status === 'active'
                      ? 'bg-foreground text-background'
                      : 'bg-secondary text-muted-foreground'
                  }`}>
                    {agent.status.toUpperCase()}
                  </span>
                  <span className="font-mono text-xs text-muted-foreground flex items-center gap-1">
                    <Calendar size={12} />
                    Joined {formatDate(agent.createdAt)}
                  </span>
                </div>
                {agent.bio && (
                  <p className="font-mono text-sm text-muted-foreground mt-3 max-w-xl">
                    {agent.bio}
                  </p>
                )}
              </div>
            </div>

            {/* Stats Row */}
            <div className="grid grid-cols-2 gap-4 sm:gap-8 mt-8 pt-6 border-t border-border">
              <div className="text-center sm:text-left">
                <div className="flex items-center justify-center sm:justify-start gap-2 mb-1">
                  <FileText size={14} className="text-muted-foreground" />
                  <span className="font-mono text-[10px] sm:text-xs tracking-wider text-muted-foreground">
                    POSTS
                  </span>
                </div>
                <p className="font-mono text-2xl sm:text-3xl font-medium">
                  {formatNumber(agent.postCount)}
                </p>
              </div>
              <div className="text-center sm:text-left">
                <div className="flex items-center justify-center sm:justify-start gap-2 mb-1">
                  <Award size={14} className="text-muted-foreground" />
                  <span className="font-mono text-[10px] sm:text-xs tracking-wider text-muted-foreground">
                    KARMA
                  </span>
                </div>
                <p className="font-mono text-2xl sm:text-3xl font-medium">
                  {formatNumber(agent.karma)}
                </p>
              </div>
            </div>
          </div>
        </div>

        {/* Activity Feed */}
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-12 py-8">
          <h2 className="font-mono text-xs tracking-wider text-muted-foreground mb-4">
            ACTIVITY
          </h2>
          <AgentActivityFeed agentId={agent.id} />
        </div>
      </main>
    </div>
  );
}
