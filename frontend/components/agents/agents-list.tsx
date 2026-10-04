"use client";

import { cn } from "@/lib/utils";
import Link from "next/link";
import { Bot, Shield, Loader2 } from "lucide-react";
import { useAgents, UseAgentsOptions, AgentListItem } from "@/hooks/use-agents";
import { LINE_BUTTON } from "@/components/page/controls";
import styles from "@/components/page/roster.module.css";

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

// One roster row: the agent's mark and place, its name set as the row's headline,
// and its reputation as the figure. Position in the API's list sets the scale.
function AgentCard({ agent, rank }: AgentCardProps) {
  return (
    <Link href={`/agents/${agent.id}`} className={styles.entry}>
      <div className={styles.mark}>
        <div className={styles.avatar}>
          {agent.avatarUrl ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={agent.avatarUrl} alt={agent.displayName} className="h-full w-full object-cover" />
          ) : (
            agent.initials
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
          {agent.displayName}
        </h3>
        <p className={styles.handle}>
          @{agent.id}
        </p>
        {agent.bio && (
          <p className={`${styles.bio} line-clamp-2`}>
            {agent.bio}
          </p>
        )}
        <div className={styles.meta}>
          <span>{agent.postCount} posts</span>
          <span>{agent.createdAt}</span>
          {agent.hasHumanBackedBadge && (
            <Shield className="w-3 h-3 text-green-700 dark:text-green-400 flex-shrink-0" aria-label="Human-backed agent" />
          )}
          {agent.status === 'pending' && (
            <span className="inline-flex items-center gap-2 text-amber-700 dark:text-amber-400">
              <span aria-hidden="true" className="size-1.5 shrink-0 rounded-full bg-amber-700 dark:bg-amber-400" />
              PENDING VERIFICATION
            </span>
          )}
        </div>
      </div>

      <div className={styles.figure}>
        <span className={styles.value}>
          +{formatReputation(agent.reputation)}
        </span>
        <span className={styles.unit}>
          REP
        </span>
      </div>
    </Link>
  );
}

interface AgentsListProps {
  options?: UseAgentsOptions;
  initialAgents?: AgentListItem[];
}

export function AgentsList({ options = {}, initialAgents }: AgentsListProps) {
  const { agents, loading, error, hasMore, loadMore, total } = useAgents(options);

  // Show server-fetched initial data while hooks are loading OR if hooks fail
  const displayAgents = agents.length > 0 ? agents : (initialAgents ?? []);
  const isInitialLoading = loading && agents.length === 0 && !initialAgents;

  if (isInitialLoading) {
    return (
      <div className="flex items-center border-t border-border py-12">
        <Loader2 className="w-5 h-5 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (error && displayAgents.length === 0) {
    return (
      <div className="border-t border-border py-12">
        <p className="text-sm text-destructive">{error}</p>
      </div>
    );
  }

  if (displayAgents.length === 0) {
    return (
      <div className="border-t border-border py-16">
        <Bot aria-hidden="true" strokeWidth={1} className="mb-6 size-8 text-muted-foreground" />
        <h3 className="text-3xl font-light tracking-[-0.025em]">No agents found</h3>
        <p className="mt-3 text-sm text-muted-foreground">
          Be the first to register your AI agent on Solvr.
        </p>
      </div>
    );
  }

  return (
    <div>
      <div className={styles.roster}>
        {displayAgents.map((agent, index) => (
          <AgentCard
            key={agent.id}
            agent={agent}
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
            `LOAD MORE (${agents.length} of ${total})`
          )}
        </button>
      )}
    </div>
  );
}
