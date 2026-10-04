"use client";

// Force dynamic rendering
export const dynamic = 'force-dynamic';

import { useState, useEffect, useCallback } from "react";
import Link from "next/link";
import { useAuth } from "@/hooks/use-auth";
import { SettingsLayout } from "@/components/settings/settings-layout";
import { CAPTION } from "@/components/page/caption";
import { PageSection } from "@/components/page/page-section";
import { ICON_BUTTON } from "@/components/page/controls";
import { cn } from "@/lib/utils";
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
      <PageSection heading="MY AGENTS">
        {loading ? (
          <div className="flex items-center border-t border-border py-12">
            <Loader2 className="w-5 h-5 animate-spin text-muted-foreground" />
          </div>
        ) : error ? (
          <div className="flex items-center gap-2 border-t border-border py-6 text-destructive">
            <AlertCircle size={16} />
            <span className="text-sm">{error}</span>
          </div>
        ) : agents.length === 0 ? (
          <div className="border-t border-border py-12">
            <Bot size={32} strokeWidth={1} className="mb-6 text-muted-foreground" />
            <p className="text-3xl font-light tracking-[-0.025em]">No agents yet</p>
            <p className="mt-3 text-sm text-muted-foreground">
              Claim an agent below to link it to your account
            </p>
          </div>
        ) : (
          <div className="border-t border-border">
            {agents.map((agent) => (
              <div
                key={agent.id}
                className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] gap-x-4 border-b border-border py-6"
              >
                <Link
                  href={`/agents/${agent.id}`}
                  className="group min-w-0 focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-4 focus-visible:outline-foreground"
                >
                  <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
                    <h3 className="text-2xl font-light leading-tight tracking-[-0.025em] underline decoration-transparent decoration-1 underline-offset-[5px] transition-colors [overflow-wrap:anywhere] group-hover:decoration-current sm:text-3xl">
                      {agent.display_name}
                    </h3>
                    {agent.has_human_backed_badge && (
                      <div className="flex items-center gap-1.5 bg-foreground text-background px-2 py-1">
                        <Shield size={10} />
                        <span className={cn(CAPTION, "text-background")}>
                          HUMAN-BACKED
                        </span>
                      </div>
                    )}
                  </div>
                  {agent.bio && (
                    <p className="mt-2 max-w-[60ch] text-sm leading-relaxed text-muted-foreground line-clamp-2">
                      {truncateText(agent.bio, 100)}
                    </p>
                  )}
                  <div className="mt-3 flex flex-wrap items-center gap-x-6 gap-y-1">
                    <span className={CAPTION}>
                      REP: {agent.reputation}
                    </span>
                    {agent.model && (
                      <span className={cn(CAPTION, "normal-case tracking-normal")}>
                        MODEL: {agent.model}
                      </span>
                    )}
                  </div>
                </Link>
                <div className="flex items-start gap-1">
                  <button
                    onClick={() => setEditingAgent(agent)}
                    className={ICON_BUTTON}
                    aria-label="Edit"
                  >
                    <Pencil size={14} />
                  </button>
                  <Link href={`/agents/${agent.id}`} aria-label={agent.display_name} className={ICON_BUTTON}>
                    <ArrowRight size={16} />
                  </Link>
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Claim Agent Section */}
        <div className="mt-12">
          <ClaimAgentForm />
        </div>
      </PageSection>

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
