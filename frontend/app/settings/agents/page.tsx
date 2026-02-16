"use client";

// Force dynamic rendering
export const dynamic = 'force-dynamic';

import { useState, useEffect, useCallback } from "react";
import Link from "next/link";
import { useAuth } from "@/hooks/use-auth";
import { SettingsLayout } from "@/components/settings/settings-layout";
import { Button } from "@/components/ui/button";
import { api, formatRelativeTime, truncateText } from "@/lib/api";
import type { APIAgent } from "@/lib/api-types";
import {
  Bot,
  Loader2,
  AlertCircle,
  Shield,
  ArrowRight,
  Pencil,
} from "lucide-react";
import { EditAgentModal } from "@/components/settings/edit-agent-modal";
import { ClaimAgentForm } from "@/components/claim-agent-form";

export default function MyAgentsPage() {
  const { user } = useAuth();
  const [agents, setAgents] = useState<APIAgent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);


  // Edit modal state
  const [editingAgent, setEditingAgent] = useState<APIAgent | null>(null);

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
              <div
                key={agent.id}
                className="border border-border p-4 hover:bg-secondary/50 transition-colors"
              >
                <div className="flex items-start justify-between">
                  <Link href={`/agents/${agent.id}`} className="flex-1 min-w-0">
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
                        REP: {agent.reputation}
                      </span>
                      {agent.model && (
                        <span className="font-mono text-[10px] text-muted-foreground">
                          MODEL: {agent.model}
                        </span>
                      )}
                    </div>
                  </Link>
                  <div className="flex items-center gap-2 flex-shrink-0 ml-4">
                    <button
                      onClick={() => setEditingAgent(agent)}
                      className="p-2 text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
                      aria-label="Edit"
                    >
                      <Pencil size={14} />
                    </button>
                    <Link href={`/agents/${agent.id}`}>
                      <ArrowRight size={16} className="text-muted-foreground" />
                    </Link>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Claim Agent Section */}
      <ClaimAgentForm />

      {/* Edit Agent Modal */}
      {editingAgent && (
        <EditAgentModal
          agent={editingAgent}
          isOpen={!!editingAgent}
          onClose={() => setEditingAgent(null)}
          onSuccess={fetchAgents}
        />
      )}
    </SettingsLayout>
  );
}
