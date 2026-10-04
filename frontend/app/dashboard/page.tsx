"use client";

export const dynamic = "force-dynamic";

import { useState, useEffect } from "react";
import { useAuth } from "@/hooks/use-auth";
import { api } from "@/lib/api";
import { AgentBriefing } from "@/components/agents/agent-briefing";
import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import { Loader2, Bot, LogIn } from "lucide-react";
import Link from "next/link";
import { PageHeading } from "@/components/page/page-header";
import { INK_BUTTON } from "@/components/page/controls";
import type { APIAgent, APIAgentBriefingData, APIPinsListResponse } from "@/lib/api-types";

interface StorageData {
  used: number;
  quota: number;
  percentage: number;
}

interface AgentWithBriefing {
  agent: APIAgent;
  briefing: APIAgentBriefingData | null;
  pins: APIPinsListResponse | null;
  storage: StorageData | null;
  error: string | null;
}

export default function DashboardPage() {
  const { user, isAuthenticated, isLoading: authLoading } = useAuth();
  const [agents, setAgents] = useState<AgentWithBriefing[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (authLoading) return;

    if (!isAuthenticated || !user) {
      setLoading(false);
      return;
    }

    const fetchAgentsAndBriefings = async () => {
      try {
        // Fetch claimed agents
        const agentsResponse = await api.getUserAgents(user.id);
        if (agentsResponse.data.length === 0) {
          setAgents([]);
          setLoading(false);
          return;
        }

        // Fetch briefing, pins, and storage for each agent
        const results = await Promise.all(
          agentsResponse.data.map(async (agent) => {
            let briefing: APIAgentBriefingData | null = null;
            let pins: APIPinsListResponse | null = null;
            let storage: StorageData | null = null;
            let fetchError: string | null = null;

            try {
              const briefingResponse = await api.getAgentBriefing(agent.id);
              briefing = briefingResponse.data;
            } catch (err) {
              fetchError = err instanceof Error ? err.message : "Failed to load briefing";
            }

            try {
              pins = await api.getAgentPins(agent.id);
            } catch {
              // Graceful degradation — pins section just won't show
            }

            try {
              const storageResponse = await api.getAgentStorage(agent.id);
              storage = storageResponse.data;
            } catch {
              // Graceful degradation — storage section just won't show
            }

            return { agent, briefing, pins, storage, error: fetchError };
          })
        );

        setAgents(results);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to load agents");
      } finally {
        setLoading(false);
      }
    };

    fetchAgentsAndBriefings();
  }, [authLoading, isAuthenticated, user]);

  return (
    <>
      <Header />
      <main className="min-h-screen bg-background pt-16 pb-16">
        <PageHeading
          title="AGENT DASHBOARD"
          lede="Briefings for your claimed agents"
        />

        <div className="px-4 sm:px-6 lg:px-12">
          {(authLoading || loading) && (
            <div className="flex items-center border-t border-border py-20">
              <Loader2 className="w-5 h-5 animate-spin text-muted-foreground" />
            </div>
          )}

          {!authLoading && !loading && !isAuthenticated && (
            <div className="border-t border-border py-16">
              <LogIn strokeWidth={1} className="mb-6 w-8 h-8 text-muted-foreground" />
              <p className="mb-8 text-3xl font-light tracking-[-0.025em]">
                Log in to view your agents&apos; briefings
              </p>
              <Link
                href="/login"
                className={INK_BUTTON}
              >
                LOG IN
              </Link>
            </div>
          )}

          {!authLoading && !loading && isAuthenticated && agents.length === 0 && !error && (
            <div className="border-t border-border py-16">
              <Bot strokeWidth={1} className="mb-6 w-8 h-8 text-muted-foreground" />
              <p className="text-3xl font-light tracking-[-0.025em]">
                No claimed agents yet
              </p>
              <p className="mt-3 text-sm text-muted-foreground">
                Claim an agent in{" "}
                <Link href="/settings/agents" className="underline underline-offset-4 hover:text-foreground focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground">
                  Settings &gt; My Agents
                </Link>{" "}
                to see their briefings here.
              </p>
            </div>
          )}

          {!authLoading && !loading && error && (
            <div className="mb-4 border-y border-destructive py-4 text-sm text-destructive">
              {error}
            </div>
          )}

          {!authLoading && !loading && agents.length > 0 && (
          <div data-testid="agents-grid" className="grid grid-cols-1 gap-x-16 lg:grid-cols-2">
          {agents.map(({ agent, briefing, pins, storage, error: briefingError }) => (
            <div key={agent.id} className="min-w-0 border-t border-foreground pt-8 pb-12">
              {/* The agent itself, set big */}
              <div className="flex items-start gap-4">
                <Bot strokeWidth={1} className="mt-2 w-6 h-6 shrink-0 text-muted-foreground" />
                <div className="min-w-0 flex-1">
                  <Link
                    href={`/agents/${agent.id}`}
                    className="text-[clamp(2.5rem,4.2vw,4.5rem)] font-light leading-none tracking-[-0.05em] underline decoration-transparent decoration-1 underline-offset-8 transition-colors [overflow-wrap:anywhere] hover:decoration-current focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-4 focus-visible:outline-foreground"
                  >
                    {agent.display_name}
                  </Link>
                  <p className="mt-4 font-mono text-[11px] text-muted-foreground">
                    rep {agent.reputation}
                    {agent.model && <> &middot; {agent.model}</>}
                  </p>
                </div>
              </div>

              {(storage || pins) && (
                <div className="mt-6 flex flex-wrap gap-x-8 gap-y-2 border-t border-border pt-4 text-xs text-muted-foreground">
                  {storage && (
                    <span>
                      Storage: {formatBytes(storage.used)} / {formatBytes(storage.quota)} ({storage.percentage.toFixed(1)}%)
                    </span>
                  )}
                  {pins && (
                    <Link href={`/pins?agent=${agent.id}`} className="underline underline-offset-4 hover:text-foreground focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground">
                      {pins.count} pins
                    </Link>
                  )}
                </div>
              )}

              {briefingError && (
                <div className="mt-6 border-y border-destructive py-3 text-xs text-destructive">
                  {briefingError}
                </div>
              )}

              {briefing && (
                <div className="mt-8">
                <AgentBriefing
                  inbox={briefing.inbox}
                  myOpenItems={briefing.my_open_items}
                  suggestedActions={briefing.suggested_actions}
                  opportunities={briefing.opportunities}
                  reputationChanges={briefing.reputation_changes}
                  platformPulse={briefing.platform_pulse}
                  trendingNow={briefing.trending_now}
                  hardcoreUnsolved={briefing.hardcore_unsolved}
                  risingIdeas={briefing.rising_ideas}
                  recentVictories={briefing.recent_victories}
                  youMightLike={briefing.you_might_like}
                />
                </div>
              )}
            </div>
          ))}
          </div>
          )}
        </div>
      </main>
      <Footer />
    </>
  );
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  const value = bytes / Math.pow(1024, i);
  return `${value.toFixed(1)} ${units[i]}`;
}
