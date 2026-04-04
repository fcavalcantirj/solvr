"use client";

import { Bot } from "lucide-react";
import type { APIAgentPresenceRecord } from "@/lib/api-types";

interface PresenceSidebarProps {
  agents: APIAgentPresenceRecord[];
  layout?: "mobile" | "desktop";
}

export function PresenceSidebar({
  agents,
  layout = "desktop",
}: PresenceSidebarProps) {
  if (layout === "mobile") {
    return (
      <div className="flex items-center gap-4 overflow-x-auto py-3 border-b border-border mb-4">
        <span className="font-mono text-[10px] tracking-[0.3em] text-muted-foreground shrink-0">
          ACTIVE
        </span>
        {agents.map((agent) => (
          <div key={agent.id} className="flex items-center gap-1.5 shrink-0">
            <div className="relative">
              <div className="w-6 h-6 bg-secondary rounded-full flex items-center justify-center">
                <Bot className="w-3 h-3 text-muted-foreground" />
              </div>
              <div className="absolute -bottom-0.5 -right-0.5 w-2 h-2 bg-green-500 rounded-full animate-pulse" />
            </div>
            <span className="font-mono text-xs truncate max-w-[80px]">
              {agent.agent_name}
            </span>
          </div>
        ))}
        {agents.length === 0 && (
          <span className="text-xs text-muted-foreground">No agents active</span>
        )}
      </div>
    );
  }

  // Desktop layout (default)
  return (
    <div className="sticky top-24 space-y-1">
      <h3 className="font-mono text-[10px] tracking-[0.3em] text-muted-foreground mb-3">
        ACTIVE AGENTS
      </h3>
      {agents.length === 0 ? (
        <p className="text-xs text-muted-foreground">No agents currently active</p>
      ) : (
        agents.map((agent) => (
          <div key={agent.id} className="flex items-center gap-2 py-2">
            <div className="relative">
              <div className="w-8 h-8 bg-secondary rounded-full flex items-center justify-center">
                <Bot className="w-4 h-4 text-muted-foreground" />
              </div>
              <div className="absolute -bottom-0.5 -right-0.5 w-2.5 h-2.5 bg-green-500 rounded-full animate-pulse" />
            </div>
            <div className="min-w-0">
              <p className="font-mono text-xs tracking-wider truncate">
                {agent.agent_name}
              </p>
            </div>
          </div>
        ))
      )}
    </div>
  );
}
