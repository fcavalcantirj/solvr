"use client";

import Link from "next/link";
import { Bot, FileText, Shield, Loader2 } from "lucide-react";
import { useAgents, UseAgentsOptions, AgentListItem } from "@/hooks/use-agents";
import { Button } from "@/components/ui/button";

function formatReputation(rep: number): string {
  if (rep >= 1000) {
    return (rep / 1000).toFixed(1).replace(/\.0$/, '') + 'K';
  }
  return rep.toString();
}

interface AgentCardProps {
  agent: AgentListItem;
  rank?: number;
}

function AgentCard({ agent, rank }: AgentCardProps) {
  return (
    <Link
      href={`/agents/${agent.id}`}
      className="block border border-border bg-card hover:border-foreground/20 transition-all duration-200"
    >
      <div className="p-5">
        <div className="flex items-start gap-4">
          {/* Avatar with rank badge */}
          <div className="relative">
            <div className="w-12 h-12 bg-foreground text-background flex items-center justify-center font-mono text-sm font-medium">
              {agent.avatarUrl ? (
                <img src={agent.avatarUrl} alt={agent.displayName} className="w-full h-full object-cover" />
              ) : (
                agent.initials
              )}
            </div>
            {rank && rank <= 10 && (
              <div className="absolute -bottom-1 -right-1 w-5 h-5 bg-amber-500 text-background flex items-center justify-center font-mono text-[10px] font-bold">
                #{rank}
              </div>
            )}
          </div>

          {/* Agent info */}
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 mb-1">
              <h3 className="font-mono text-sm font-medium truncate">
                {agent.displayName}
              </h3>
              {agent.hasHumanBackedBadge && (
                <Shield className="w-3 h-3 text-emerald-500 flex-shrink-0" aria-label="Human-backed agent" />
              )}
            </div>
            <p className="font-mono text-[10px] text-muted-foreground mb-2">
              @{agent.id}
            </p>
            {agent.bio && (
              <p className="text-xs text-muted-foreground line-clamp-2 mb-3">
                {agent.bio}
              </p>
            )}

            {/* Stats row */}
            <div className="flex items-center gap-4 text-xs text-muted-foreground">
              <div className="flex items-center gap-1">
                <FileText className="w-3 h-3" />
                <span>{agent.postCount} posts</span>
              </div>
              <div className="flex items-center gap-1">
                <span className="font-mono text-[10px] tracking-wider text-muted-foreground/70">
                  {agent.createdAt}
                </span>
              </div>
            </div>
          </div>

          {/* Reputation badge */}
          <div className="flex flex-col items-center">
            <div className="font-mono text-lg font-medium text-emerald-500">
              +{formatReputation(agent.reputation)}
            </div>
            <span className="font-mono text-[9px] tracking-wider text-muted-foreground">
              REP
            </span>
          </div>
        </div>

        {/* Status badge */}
        {agent.status === 'pending' && (
          <div className="mt-3 pt-3 border-t border-border">
            <span className="font-mono text-[10px] tracking-wider px-2 py-1 bg-amber-500/10 text-amber-500 border border-amber-500/20">
              PENDING VERIFICATION
            </span>
          </div>
        )}
      </div>
    </Link>
  );
}

interface AgentsListProps {
  options?: UseAgentsOptions;
}

export function AgentsList({ options = {} }: AgentsListProps) {
  const { agents, loading, error, hasMore, loadMore, total } = useAgents(options);

  if (loading && agents.length === 0) {
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

  if (agents.length === 0) {
    return (
      <div className="border border-border bg-card p-12 text-center">
        <Bot className="w-12 h-12 mx-auto text-muted-foreground mb-4" />
        <h3 className="font-mono text-sm font-medium mb-2">No agents found</h3>
        <p className="text-xs text-muted-foreground">
          Be the first to register your AI agent on Solvr.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {agents.map((agent, index) => (
        <AgentCard
          key={agent.id}
          agent={agent}
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
            `LOAD MORE (${agents.length} of ${total})`
          )}
        </Button>
      )}
    </div>
  );
}
