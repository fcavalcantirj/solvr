"use client";

import { Users } from "lucide-react";
import type { APIRoomMember, APIRoom } from "@/lib/api-types";

interface ParticipantsPanelProps {
  room: APIRoom;
  members: APIRoomMember[];
}

export function ParticipantsPanel({ room, members }: ParticipantsPanelProps) {
  const memberCount = members.length;
  const capacityMax = room.capacity_max;

  return (
    <div className="border border-border bg-card">
      <div className="flex items-center gap-2 p-4 border-b border-border">
        <Users size={14} className="text-foreground" />
        <h3 className="font-mono text-xs tracking-[0.2em]">PARTICIPANTS</h3>
        <span className="ml-auto font-mono text-[10px] text-muted-foreground">
          {capacityMax ? `${memberCount} / ${capacityMax}` : `${memberCount} members`}
        </span>
      </div>
      <div className="divide-y divide-border">
        {members.length > 0 ? (
          members.map((member) => (
            <div
              key={member.agent_id}
              className="flex items-center justify-between p-4 hover:bg-secondary/50 transition-colors"
            >
              <div className="min-w-0 flex-1">
                <p className="font-mono text-xs tracking-wider truncate">
                  {member.agent_id}
                </p>
                <p className="font-mono text-[10px] text-muted-foreground capitalize">
                  {member.role}
                </p>
              </div>
            </div>
          ))
        ) : (
          <div className="p-4">
            <p className="font-mono text-[10px] text-muted-foreground leading-relaxed">
              No participants yet.
            </p>
          </div>
        )}
      </div>

      {/* Step 5: Connect an agent action clear on desktop and mobile */}
      <a
        href={`/connect?preset=plan-and-build&room=${room.slug}`}
        className="block w-full font-mono text-xs tracking-wider text-center py-2.5 border-t border-border hover:bg-secondary/50 transition-colors"
      >
        ADD ANOTHER AGENT
      </a>
    </div>
  );
}
