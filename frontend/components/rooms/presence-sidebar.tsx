"use client";

import { Bot, Radio, MessageSquare, Clock, Terminal } from "lucide-react";
import { formatDistanceToNow } from "date-fns";
import { useRoomMembers } from "@/hooks/use-room-members";
import { ParticipantsPanel } from "./participants-panel";
import type { APIAgentPresenceRecord } from "@/lib/api-types";
import type { APIRoom } from "@/lib/api-types";

interface PresenceSidebarProps {
  agents: APIAgentPresenceRecord[];
  room?: APIRoom;
  layout?: "mobile" | "desktop";
}

export function PresenceSidebar({
  agents,
  room,
  layout = "desktop",
}: PresenceSidebarProps) {
  const { members } = useRoomMembers(room?.slug || "");

  if (layout === "mobile") {
    return (
      <div className="flex flex-wrap items-center gap-4 py-3 border-b border-border mb-4">
        <span className="font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground shrink-0">
          ACTIVE
        </span>
        {agents.map((agent) => (
          <div key={agent.id} className="flex items-center gap-1.5 shrink-0">
            <div className="relative">
              <div className="w-6 h-6 bg-secondary rounded-full flex items-center justify-center">
                <Bot className="w-3 h-3 text-muted-foreground" />
              </div>
              <div className="absolute -bottom-0.5 -right-0.5 w-2 h-2 bg-green-700 dark:bg-green-400 rounded-full animate-pulse" />
            </div>
            <span className="font-mono text-[11px] truncate max-w-[80px]">
              {agent.agent_name}
            </span>
          </div>
        ))}
        {agents.length === 0 && (
          <span className="font-mono text-[11px] text-muted-foreground">
            Waiting for agents to join...
          </span>
        )}
      </div>
    );
  }

  // Desktop layout
  return (
    <div className="space-y-8">
      {/* Active Agents Card */}
      <div className="min-w-0 border-t border-border">
        <div className="flex flex-wrap items-center gap-2 py-4">
          {agents.length > 0 ? (
            <Radio size={14} className="text-green-700 dark:text-green-400" />
          ) : (
            <Radio size={14} className="text-muted-foreground" />
          )}
          <h3 className="font-mono text-[11px] uppercase tracking-[0.18em]">
            {agents.length > 0 ? "LIVE AGENTS" : "AGENTS"}
          </h3>
          {agents.length > 0 && (
            <span className="ml-auto font-mono text-[11px] text-green-700 dark:text-green-400">
              {agents.length} online
            </span>
          )}
        </div>
        <div className="divide-y divide-border">
          {agents.length > 0 ? (
            agents.map((agent) => (
              <div
                key={agent.id}
                className="flex items-center gap-3 py-3"
              >
                <div className="relative">
                  <div className="w-8 h-8 bg-secondary rounded-full flex items-center justify-center">
                    <Bot className="w-4 h-4 text-muted-foreground" />
                  </div>
                  <div className="absolute -bottom-0.5 -right-0.5 w-2.5 h-2.5 bg-green-700 dark:bg-green-400 rounded-full animate-pulse" />
                </div>
                <div className="min-w-0 flex-1">
                  <p className="font-mono text-[11px] uppercase tracking-[0.18em] truncate">
                    {agent.agent_name}
                  </p>
                </div>
              </div>
            ))
          ) : (
            <div className="pb-4 space-y-4">
              <div className="flex items-center gap-3">
                <div className="w-8 h-8 bg-secondary/50 rounded-full flex items-center justify-center">
                  <Bot className="w-4 h-4 text-muted-foreground/30" />
                </div>
                <div className="flex-1">
                  <div className="h-2.5 bg-secondary/50 rounded-none w-24 mb-1.5" />
                  <div className="h-2 bg-secondary/30 rounded-none w-16" />
                </div>
              </div>
              <p className="text-xs text-muted-foreground leading-relaxed">
                No agents currently active. Agents join via the A2A protocol and
                appear here in real time.
              </p>
            </div>
          )}
        </div>
      </div>

      {/* Room Stats Card */}
      {room && (
        <div className="min-w-0 border-t border-border">
          <div className="flex flex-wrap items-center gap-2 py-4">
            <MessageSquare size={14} className="text-foreground" />
            <h3 className="font-mono text-[11px] uppercase tracking-[0.18em]">ROOM INFO</h3>
          </div>
          <div className="pb-4 space-y-4">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <MessageSquare size={12} className="text-muted-foreground" />
                <span className="font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground">
                  Messages
                </span>
              </div>
              <span className="text-4xl font-light leading-none tracking-[-0.04em] tabular-nums">
                {room.message_count}
              </span>
            </div>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Clock size={12} className="text-muted-foreground" />
                <span className="font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground">
                  Created
                </span>
              </div>
              <span className="font-mono text-[11px] text-foreground">
                {formatDistanceToNow(new Date(room.created_at), {
                  addSuffix: true,
                })}
              </span>
            </div>
            {room.tags && room.tags.length > 0 && (
              <div className="pt-2 border-t border-border">
                <div className="flex flex-wrap gap-1.5">
                  {room.tags.map((tag) => (
                    <span
                      key={tag}
                      className="font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground bg-secondary px-1.5 py-0.5"
                    >
                      {tag}
                    </span>
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Participants Panel — Step 5: display member list with capacity — Step 6 */}
      {room && members.length > 0 && (
        <ParticipantsPanel room={room} members={members} />
      )}

      {/* Add Another Agent — role-specific prompt for N-agent collaboration */}
      {room && (
        <div className="min-w-0 border-t border-border">
          <div className="flex flex-wrap items-center gap-2 py-4">
            <Terminal size={14} className="text-foreground" />
            <h3 className="font-mono text-[11px] uppercase tracking-[0.18em]">
              ADD ANOTHER AGENT
            </h3>
          </div>
          <div className="pb-4 space-y-4">
            <p className="text-xs text-muted-foreground leading-relaxed">
              Use a role-specific prompt to connect additional agents (reviewer, researcher, executor, or custom roles) to this room. Each agent gets its own identity and per-agent token.
            </p>
            <a
              href={`/connect?preset=plan-and-build&room=${room.slug}`}
              className="block w-full font-mono text-[11px] uppercase tracking-[0.18em] text-center py-2.5 border border-border hover:border-foreground transition-colors"
            >
              GENERATE ROLE-SPECIFIC PROMPT
            </a>
          </div>
        </div>
      )}
    </div>
  );
}
