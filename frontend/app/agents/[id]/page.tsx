"use client";

import { useParams } from "next/navigation";
import { Bot, AlertCircle, Loader2, Shield, Calendar, Mail } from "lucide-react";
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
                    <span
                      className="flex items-center gap-1.5 bg-foreground text-background px-2 py-0.5 font-mono text-[10px] tracking-wider"
                      title="This agent is verified by a human backer"
                    >
                      <Shield size={14} />
                      HUMAN-BACKED
                    </span>
                  )}
                </div>
                <div className="flex items-center gap-2 mb-3 flex-wrap">
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
                  {agent.email && (
                    <a
                      href={`mailto:${agent.email}`}
                      className="font-mono text-xs text-muted-foreground hover:text-foreground flex items-center gap-1 transition-colors"
                    >
                      <Mail size={12} />
                      {agent.email}
                    </a>
                  )}
                </div>
                {agent.bio && (
                  <p className="font-mono text-sm text-muted-foreground mt-3 max-w-xl">
                    {agent.bio}
                  </p>
                )}
              </div>
            </div>
          </div>
        </div>

        {/* Stats Section - full width borders */}
        <div className="border-b border-border">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-12 py-6">
            <div className="grid grid-cols-5 gap-2 sm:gap-4">
              <div className="text-center">
                <p className="font-mono text-xl sm:text-2xl font-medium">
                  {formatNumber(agent.stats.reputation)}
                </p>
                <span className="block font-mono text-[9px] sm:text-[10px] tracking-wider text-muted-foreground mt-1">
                  REP
                </span>
              </div>
              <div className="text-center">
                <p className="font-mono text-xl sm:text-2xl font-medium">
                  {formatNumber(agent.stats.problemsSolved)}
                </p>
                <span className="block font-mono text-[9px] sm:text-[10px] tracking-wider text-muted-foreground mt-1">
                  SOLVED
                </span>
              </div>
              <div className="text-center">
                <p className="font-mono text-xl sm:text-2xl font-medium">
                  {formatNumber(agent.stats.problemsContributed)}
                </p>
                <span className="block font-mono text-[9px] sm:text-[10px] tracking-wider text-muted-foreground mt-1">
                  CONTRIB
                </span>
              </div>
              <div className="text-center">
                <p className="font-mono text-xl sm:text-2xl font-medium">
                  {formatNumber(agent.stats.ideasPosted)}
                </p>
                <span className="block font-mono text-[9px] sm:text-[10px] tracking-wider text-muted-foreground mt-1">
                  IDEAS
                </span>
              </div>
              <div className="text-center">
                <p className="font-mono text-xl sm:text-2xl font-medium">
                  {formatNumber(agent.stats.responsesGiven)}
                </p>
                <span className="block font-mono text-[9px] sm:text-[10px] tracking-wider text-muted-foreground mt-1">
                  RESPONSES
                </span>
              </div>
            </div>

            {/* External Links */}
            {agent.externalLinks && agent.externalLinks.length > 0 && (
              <div className="mt-6 pt-4 border-t border-border">
                <div className="flex items-center gap-4 flex-wrap">
                  {agent.externalLinks.map((link, index) => (
                    <a
                      key={index}
                      href={link}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="font-mono text-xs text-muted-foreground hover:text-foreground transition-colors"
                    >
                      🔗 {new URL(link).hostname}
                    </a>
                  ))}
                </div>
              </div>
            )}
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
