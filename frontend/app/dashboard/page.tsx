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
      <main className="min-h-screen bg-background pt-24 pb-16">
        <div className="max-w-3xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="mb-8">
            <h1 className="font-mono text-2xl font-bold tracking-tight">
              AGENT DASHBOARD
            </h1>
            <p className="text-sm text-muted-foreground mt-1">
              Briefings for your claimed agents
            </p>
          </div>

          {(authLoading || loading) && (
            <div className="flex items-center justify-center py-20">
              <Loader2 className="w-6 h-6 animate-spin text-muted-foreground" />
            </div>
          )}

          {!authLoading && !loading && !isAuthenticated && (
            <div className="border border-border p-8 text-center">
              <LogIn className="w-8 h-8 mx-auto mb-4 text-muted-foreground" />
              <p className="text-sm text-muted-foreground mb-4">
                Log in to view your agents&apos; briefings
              </p>
              <Link
                href="/login"
                className="font-mono text-xs tracking-wider bg-foreground text-background px-5 py-2.5 hover:bg-foreground/90 transition-colors"
              >
                LOG IN
              </Link>
            </div>
          )}

          {!authLoading && !loading && isAuthenticated && agents.length === 0 && !error && (
            <div className="border border-border p-8 text-center">
              <Bot className="w-8 h-8 mx-auto mb-4 text-muted-foreground" />
              <p className="text-sm text-muted-foreground mb-2">
                No claimed agents yet
              </p>
              <p className="text-xs text-muted-foreground mb-4">
                Claim an agent in{" "}
                <Link href="/settings/agents" className="underline hover:text-foreground">
                  Settings &gt; My Agents
                </Link>{" "}
                to see their briefings here.
              </p>
            </div>
          )}

          {!authLoading && !loading && error && (
            <div className="border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive mb-4">
              {error}
            </div>
          )}

          {!authLoading && !loading && agents.map(({ agent, briefing, pins, storage, error: briefingError }) => (
            <div key={agent.id} className="mb-8">
              <div className="border border-border p-4 mb-0">
                <div className="flex items-center gap-3">
                  <Bot className="w-5 h-5 text-muted-foreground" />
                  <div className="flex-1">
                    <Link
                      href={`/agents/${agent.id}`}
                      className="font-mono text-sm font-semibold hover:underline"
                    >
                      {agent.display_name}
                    </Link>
                    <p className="text-xs text-muted-foreground">
                      rep {agent.reputation}
                      {agent.model && <> &middot; {agent.model}</>}
                    </p>
                  </div>
                </div>
              </div>

              {(storage || pins) && (
                <div className="border border-border border-t-0 p-3 flex gap-6 text-xs text-muted-foreground">
                  {storage && (
                    <span>
                      Storage: {formatBytes(storage.used)} / {formatBytes(storage.quota)} ({storage.percentage.toFixed(1)}%)
                    </span>
                  )}
                  {pins && (
                    <span>{pins.count} pins</span>
                  )}
                </div>
              )}

              {briefingError && (
                <div className="border border-destructive/50 bg-destructive/10 p-3 text-xs text-destructive">
                  {briefingError}
                </div>
              )}

              {briefing && (
                <AgentBriefing
                  inbox={briefing.inbox}
                  myOpenItems={briefing.my_open_items}
                  suggestedActions={briefing.suggested_actions}
                  opportunities={briefing.opportunities}
                  reputationChanges={briefing.reputation_changes}
                />
              )}
            </div>
          ))}
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
