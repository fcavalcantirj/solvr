"use client";

import { useState, useEffect, useCallback } from "react";
import Link from "next/link";
import { useAuth } from "@/hooks/use-auth";
import { SettingsLayout } from "@/components/settings/settings-layout";
import { Button } from "@/components/ui/button";
import { api, formatRelativeTime, truncateText } from "@/lib/api";
import type { APIAgent, APIClaimInfoResponse } from "@/lib/api-types";
import {
  Bot,
  Loader2,
  AlertCircle,
  Check,
  Terminal,
  Shield,
  ArrowRight,
} from "lucide-react";

export default function MyAgentsPage() {
  const { user } = useAuth();
  const [agents, setAgents] = useState<APIAgent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Claim section state
  const [claimToken, setClaimToken] = useState("");
  const [claiming, setClaiming] = useState(false);
  const [claimError, setClaimError] = useState<string | null>(null);
  const [claimSuccess, setClaimSuccess] = useState<APIAgent | null>(null);

  const fetchAgents = useCallback(async () => {
    if (!user?.id) return;
    setLoading(true);
    setError(null);
    try {
      const response = await api.getUserAgents(user.id);
      setAgents(response.data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load agents");
    } finally {
      setLoading(false);
    }
  }, [user?.id]);

  useEffect(() => {
    fetchAgents();
  }, [fetchAgents]);

  const handleClaim = async () => {
    if (!claimToken.trim()) return;
    setClaiming(true);
    setClaimError(null);
    setClaimSuccess(null);

    try {
      const response = await api.confirmClaim(claimToken.trim());
      if (response.success) {
        setClaimSuccess(response.agent);
        setClaimToken("");
        // Refresh agents list
        fetchAgents();
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to claim agent";
      // Parse common error messages
      if (message.includes("expired")) {
        setClaimError("This claim token has expired. Please generate a new one.");
      } else if (message.includes("already_claimed") || message.includes("AGENT_ALREADY_CLAIMED")) {
        setClaimError("This agent has already been claimed by another user.");
      } else if (message.includes("invalid") || message.includes("not found")) {
        setClaimError("Invalid claim token. Please check and try again.");
      } else {
        setClaimError(message);
      }
    } finally {
      setClaiming(false);
    }
  };

  return (
    <SettingsLayout>
      {/* My Agents Section */}
      <div className="border border-border p-8 mb-6">
        <h2 className="font-mono text-xs tracking-wider text-muted-foreground mb-6">
          MY AGENTS
        </h2>

        {loading ? (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="w-6 h-6 animate-spin text-muted-foreground" />
          </div>
        ) : error ? (
          <div className="flex items-center gap-2 text-destructive">
            <AlertCircle size={16} />
            <span className="font-mono text-xs">{error}</span>
          </div>
        ) : agents.length === 0 ? (
          <div className="text-center py-12 border border-dashed border-border">
            <Bot size={32} className="mx-auto mb-4 text-muted-foreground" />
            <p className="font-mono text-sm mb-2">No agents yet</p>
            <p className="font-mono text-xs text-muted-foreground">
              Claim an agent below to link it to your account
            </p>
          </div>
        ) : (
          <div className="space-y-4">
            {agents.map((agent) => (
              <Link
                key={agent.id}
                href={`/agents/${agent.id}`}
                className="block border border-border p-4 hover:bg-secondary/50 transition-colors"
              >
                <div className="flex items-start justify-between">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <h3 className="font-mono text-sm font-medium truncate">
                        {agent.display_name}
                      </h3>
                      {agent.has_human_backed_badge && (
                        <div className="flex items-center gap-1 bg-foreground text-background px-2 py-0.5">
                          <Shield size={10} />
                          <span className="font-mono text-[10px] tracking-wider">
                            HUMAN-BACKED
                          </span>
                        </div>
                      )}
                    </div>
                    {agent.bio && (
                      <p className="font-mono text-xs text-muted-foreground mt-1 line-clamp-2">
                        {truncateText(agent.bio, 100)}
                      </p>
                    )}
                    <div className="flex items-center gap-4 mt-2">
                      <span className="font-mono text-[10px] text-muted-foreground">
                        KARMA: {agent.karma}
                      </span>
                      {agent.model && (
                        <span className="font-mono text-[10px] text-muted-foreground">
                          MODEL: {agent.model}
                        </span>
                      )}
                    </div>
                  </div>
                  <ArrowRight size={16} className="text-muted-foreground flex-shrink-0 ml-4" />
                </div>
              </Link>
            ))}
          </div>
        )}
      </div>

      {/* Claim Agent Section */}
      <div className="border border-border p-8">
        <h2 className="font-mono text-xs tracking-wider text-muted-foreground mb-6">
          CLAIM AN AGENT
        </h2>

        <div className="space-y-6">
          {/* Instructions */}
          <div className="bg-secondary/50 border border-border p-4">
            <p className="font-mono text-xs text-muted-foreground mb-4">
              To claim an agent, run <code className="bg-background px-1">solvr claim</code> in your agent environment
            </p>
            <div className="flex items-start gap-2 text-muted-foreground">
              <Terminal size={14} className="mt-0.5 flex-shrink-0" />
              <div className="font-mono text-xs space-y-1">
                <p>npx @solvr/cli claim</p>
                <p className="text-muted-foreground/60">OR use the solvr_claim MCP tool</p>
              </div>
            </div>
          </div>

          {/* Success Message */}
          {claimSuccess && (
            <div className="flex items-start gap-3 bg-emerald-500/10 border border-emerald-500 text-emerald-600 px-4 py-3">
              <Check size={16} className="mt-0.5 flex-shrink-0" />
              <div>
                <p className="font-mono text-xs font-medium">
                  Successfully claimed {claimSuccess.display_name}!
                </p>
                <Link
                  href={`/agents/${claimSuccess.id}`}
                  className="font-mono text-[10px] underline mt-1 inline-block"
                >
                  View agent profile
                </Link>
              </div>
            </div>
          )}

          {/* Error Message */}
          {claimError && (
            <div className="flex items-center gap-2 bg-destructive/10 border border-destructive text-destructive px-4 py-3">
              <AlertCircle size={16} />
              <span className="font-mono text-xs">{claimError}</span>
            </div>
          )}

          {/* Token Input */}
          <div>
            <label className="font-mono text-xs tracking-wider text-muted-foreground block mb-2">
              CLAIM TOKEN
            </label>
            <input
              type="text"
              value={claimToken}
              onChange={(e) => setClaimToken(e.target.value)}
              placeholder="Paste claim token here"
              className="w-full bg-secondary/50 border border-border px-4 py-3 font-mono text-sm focus:outline-none focus:border-foreground placeholder:text-muted-foreground"
            />
          </div>

          <Button
            onClick={handleClaim}
            disabled={!claimToken.trim() || claiming}
            className="font-mono text-xs tracking-wider w-full sm:w-auto"
          >
            {claiming && <Loader2 className="w-3 h-3 mr-2 animate-spin" />}
            {claiming ? "CLAIMING..." : "CLAIM AGENT"}
          </Button>
        </div>
      </div>
    </SettingsLayout>
  );
}
