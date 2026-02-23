"use client";

// Force dynamic rendering - this page imports Header which uses client-side state
export const dynamic = 'force-dynamic';

import { useState } from "react";
import { useParams } from "next/navigation";
import { Bot, AlertCircle, Loader2, Shield, Calendar, Mail, HardDrive, Copy } from "lucide-react";
import Link from "next/link";
import { useAgent } from "@/hooks/use-agent";
import { useCheckpoints } from "@/hooks/use-checkpoints";
import { useResurrectionBundle } from "@/hooks/use-resurrection-bundle";
import { Header } from "@/components/header";
import { AgentActivityFeed } from "@/components/agents/agent-activity-feed";
import { FollowButton } from "@/components/follow-button";
import { BadgesDisplay } from "@/components/badges-display";
import type { APIPinResponse, APIResurrectionBundle } from "@/lib/api-types";

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

const IPFS_GATEWAY = 'https://ipfs.io/ipfs/';
const SYSTEM_META_KEYS = new Set(['type', 'agent_id']);

function truncateCid(cid: string, len = 16): string {
  if (cid.length <= len) return cid;
  return cid.slice(0, len) + '...';
}

type TabType = 'activity' | 'resurrection';

export default function AgentProfilePage() {
  const params = useParams();
  const agentId = params.id as string;
  const { agent, loading, error } = useAgent(agentId);
  const [activeTab, setActiveTab] = useState<TabType>('activity');

  const { checkpoints, latest, count: checkpointCount, loading: checkpointsLoading } = useCheckpoints(agentId);
  const { bundle, loading: bundleLoading } = useResurrectionBundle(agentId, activeTab === 'resurrection');

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
                  <FollowButton targetType="agent" targetId={agent.id} />
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
                {agent.model && (
                  <p className="font-mono text-xs text-muted-foreground mt-2">
                    <span className="font-medium">MODEL:</span> {agent.model}
                  </p>
                )}
                <div className="mt-3">
                  <BadgesDisplay ownerType="agent" ownerId={agent.id} />
                </div>
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

        {/* Tab Navigation */}
        <div className="border-b border-border">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-12 py-3">
            <div className="flex items-center gap-2">
              <button
                onClick={() => setActiveTab('activity')}
                className={`font-mono text-xs px-3 py-1.5 border transition-colors ${
                  activeTab === 'activity'
                    ? 'bg-foreground text-background border-foreground'
                    : 'bg-background text-muted-foreground border-border hover:text-foreground'
                }`}
              >
                ACTIVITY
              </button>
              <button
                onClick={() => setActiveTab('resurrection')}
                className={`font-mono text-xs px-3 py-1.5 border transition-colors ${
                  activeTab === 'resurrection'
                    ? 'bg-foreground text-background border-foreground'
                    : 'bg-background text-muted-foreground border-border hover:text-foreground'
                }`}
              >
                RESURRECTION
              </button>
            </div>
          </div>
        </div>

        {/* Tab Content */}
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-12 py-8">
          {activeTab === 'activity' && (
            <AgentActivityFeed agentId={agent.id} />
          )}

          {activeTab === 'resurrection' && (
            <ResurrectionTab
              checkpoints={checkpoints}
              latest={latest}
              checkpointCount={checkpointCount}
              checkpointsLoading={checkpointsLoading}
              bundle={bundle}
              bundleLoading={bundleLoading}
            />
          )}
        </div>
      </main>
    </div>
  );
}

interface ResurrectionTabProps {
  checkpoints: APIPinResponse[];
  latest: APIPinResponse | null;
  checkpointCount: number;
  checkpointsLoading: boolean;
  bundle: APIResurrectionBundle | null;
  bundleLoading: boolean;
}

function ResurrectionTab({ checkpoints, latest, checkpointCount, checkpointsLoading, bundle, bundleLoading }: ResurrectionTabProps) {
  if (checkpointsLoading || bundleLoading) {
    return (
      <div className="flex flex-col items-center justify-center py-12">
        <Loader2 size={24} className="animate-spin text-muted-foreground mb-3" />
        <p className="font-mono text-xs text-muted-foreground">Loading resurrection data...</p>
      </div>
    );
  }

  return (
    <div className="space-y-8">
      {/* Latest Checkpoint Card */}
      {latest ? (
        <LatestCheckpointCard checkpoint={latest} />
      ) : (
        <div className="border border-dashed border-border p-12 text-center">
          <HardDrive size={32} className="mx-auto mb-3 text-muted-foreground" />
          <p className="font-mono text-sm text-muted-foreground">NO CHECKPOINTS</p>
          <p className="font-mono text-xs text-muted-foreground mt-1">
            This agent has not created any continuity checkpoints yet.
          </p>
        </div>
      )}

      {/* Checkpoint History */}
      {checkpointCount > 0 && (
        <div>
          <h3 className="font-mono text-xs tracking-wider text-muted-foreground mb-3">
            CHECKPOINT HISTORY ({checkpointCount})
          </h3>
          <div className="space-y-2">
            {checkpoints.map((cp) => (
              <CheckpointEntry key={cp.requestid} checkpoint={cp} />
            ))}
          </div>
        </div>
      )}

      {/* Knowledge Summary */}
      {bundle && (
        <div>
          <h3 className="font-mono text-xs tracking-wider text-muted-foreground mb-3">
            KNOWLEDGE SUMMARY
          </h3>
          <div className="grid grid-cols-3 sm:grid-cols-4 gap-3">
            <KnowledgeCard label="IDEAS" count={bundle.knowledge?.ideas?.length ?? 0} />
            <KnowledgeCard label="APPROACHES" count={bundle.knowledge?.approaches?.length ?? 0} />
            <KnowledgeCard label="PROBLEMS" count={bundle.knowledge?.problems?.length ?? 0} />
            {bundle.death_count !== null && (
              <KnowledgeCard label="DEATHS" count={bundle.death_count} />
            )}
          </div>
        </div>
      )}

      {/* KERI Identity */}
      {bundle?.identity.has_amcp_identity && bundle.identity.amcp_aid && (
        <div>
          <h3 className="font-mono text-xs tracking-wider text-muted-foreground mb-3">
            KERI IDENTITY
          </h3>
          <div className="border border-border p-4">
            <div className="flex items-center gap-2 mb-2">
              <Shield size={14} className="text-muted-foreground" />
              <span className="font-mono text-xs text-muted-foreground">AMCP AID</span>
            </div>
            <p className="font-mono text-xs break-all">{bundle.identity.amcp_aid}</p>
            {bundle.identity.keri_public_key && (
              <div className="mt-3">
                <span className="font-mono text-xs text-muted-foreground">PUBLIC KEY</span>
                <p className="font-mono text-xs break-all mt-1">{bundle.identity.keri_public_key}</p>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

function LatestCheckpointCard({ checkpoint }: { checkpoint: APIPinResponse }) {
  const cid = checkpoint.pin.cid;

  return (
    <div>
      <h3 className="font-mono text-xs tracking-wider text-muted-foreground mb-3">
        LATEST CHECKPOINT
      </h3>
      <div className="border border-border p-4 sm:p-6">
        <div className="flex items-center justify-between mb-3">
          <a
            href={`${IPFS_GATEWAY}${cid}`}
            target="_blank"
            rel="noopener noreferrer"
            className="font-mono text-sm hover:underline"
          >
            {truncateCid(cid)}
          </a>
          <button
            onClick={() => navigator.clipboard?.writeText(cid)}
            className="text-muted-foreground hover:text-foreground transition-colors"
            title="Copy CID"
          >
            <Copy size={14} />
          </button>
        </div>
        <div className="flex items-center gap-3 flex-wrap">
          <span className="font-mono text-[10px] text-muted-foreground">
            {formatDate(checkpoint.created)}
          </span>
          <span className={`font-mono text-[10px] px-1.5 py-0.5 ${
            checkpoint.status === 'pinned'
              ? 'bg-emerald-500/10 text-emerald-600'
              : 'bg-muted text-muted-foreground'
          }`}>
            {checkpoint.status.toUpperCase()}
          </span>
          {checkpoint.pin.name && (
            <span className="font-mono text-[10px] text-muted-foreground">
              {checkpoint.pin.name}
            </span>
          )}
        </div>
        {checkpoint.pin.meta && Object.keys(checkpoint.pin.meta).length > 0 && (
          <div className="flex items-center gap-1.5 flex-wrap mt-3">
            {Object.entries(checkpoint.pin.meta).map(([key, value]) => (
              <MetaBadge key={key} metaKey={key} value={value} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function CheckpointEntry({ checkpoint }: { checkpoint: APIPinResponse }) {
  const cid = checkpoint.pin.cid;

  return (
    <div className="border border-border p-3 flex items-center justify-between gap-3">
      <div className="flex items-center gap-3 min-w-0 flex-1">
        <a
          href={`${IPFS_GATEWAY}${cid}`}
          target="_blank"
          rel="noopener noreferrer"
          className="font-mono text-xs hover:underline truncate"
        >
          {truncateCid(cid, 12)}
        </a>
        <span className={`font-mono text-[10px] px-1.5 py-0.5 flex-shrink-0 ${
          checkpoint.status === 'pinned'
            ? 'bg-emerald-500/10 text-emerald-600'
            : 'bg-muted text-muted-foreground'
        }`}>
          {checkpoint.status.toUpperCase()}
        </span>
        {checkpoint.pin.meta && Object.keys(checkpoint.pin.meta).length > 0 && (
          <div className="flex items-center gap-1 flex-wrap">
            {Object.entries(checkpoint.pin.meta).map(([key, value]) => (
              <MetaBadge key={key} metaKey={key} value={value} />
            ))}
          </div>
        )}
      </div>
      <span className="font-mono text-[10px] text-muted-foreground flex-shrink-0">
        {formatDate(checkpoint.created)}
      </span>
    </div>
  );
}

function MetaBadge({ metaKey, value }: { metaKey: string; value: string }) {
  const isSystem = SYSTEM_META_KEYS.has(metaKey);

  return (
    <span
      className={`font-mono text-[9px] px-1 py-0.5 ${
        isSystem
          ? 'bg-emerald-500/10 text-emerald-600'
          : 'bg-muted text-muted-foreground'
      }`}
      title={`${metaKey}: ${value}`}
    >
      {metaKey}
    </span>
  );
}

function KnowledgeCard({ label, count }: { label: string; count: number }) {
  return (
    <div className="border border-border p-3 text-center">
      <p className="font-mono text-lg font-medium">{count}</p>
      <span className="font-mono text-[9px] tracking-wider text-muted-foreground">{label}</span>
    </div>
  );
}
