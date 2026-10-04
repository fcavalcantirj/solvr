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
    <div className="min-w-0 border-t border-border">
      <div className="flex flex-wrap items-center gap-2 py-4">
        <Users size={14} className="text-foreground" />
        <h3 className="font-mono text-[11px] uppercase tracking-[0.18em]">PARTICIPANTS</h3>
        <span className="ml-auto font-mono text-[11px] text-muted-foreground">
          {capacityMax ? `${memberCount} / ${capacityMax}` : `${memberCount} members`}
        </span>
      </div>
      <div className="divide-y divide-border">
        {members.length > 0 ? (
          members.map((member) => (
            <div
              key={member.agent_id}
              className="flex items-center justify-between py-3"
            >
              <div className="min-w-0 flex-1">
                <p className="font-mono text-[11px] uppercase tracking-[0.18em] truncate">
                  {member.agent_id}
                </p>
                <p className="font-mono text-[11px] text-muted-foreground capitalize">
                  {member.role}
                </p>
              </div>
            </div>
          ))
        ) : (
          <div className="py-4">
            <p className="font-mono text-[11px] text-muted-foreground leading-relaxed">
              No participants yet.
            </p>
          </div>
        )}
      </div>

      {/* Step 5: Connect an agent action clear on desktop and mobile */}
      <a
        href={`/connect?preset=plan-and-build&room=${room.slug}`}
        className="block w-full font-mono text-[11px] uppercase tracking-[0.18em] text-center py-2.5 border border-border hover:border-foreground transition-colors"
      >
        ADD ANOTHER AGENT
      </a>
    </div>
  );
}
