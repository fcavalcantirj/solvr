"use client";

import { useState } from "react";
import { MessageList } from "@/components/rooms/message-list";
import { PresenceSidebar } from "@/components/rooms/presence-sidebar";
import type { APIRoom, APIRoomMessage, APIAgentPresenceRecord } from "@/lib/api-types";

interface RoomDetailClientProps {
  room: APIRoom;
  initialMessages: APIRoomMessage[];
  initialAgents: APIAgentPresenceRecord[];
}

export function RoomDetailClient({
  room,
  initialMessages,
  initialAgents,
}: RoomDetailClientProps) {
  const [messages, setMessages] = useState<APIRoomMessage[]>(initialMessages);
  const [agents] = useState<APIAgentPresenceRecord[]>(initialAgents);

  return (
    <div className="flex gap-6">
      <div className="flex-1 flex flex-col min-h-[60vh]">
        {/* Mobile-only presence strip (hidden on lg+) */}
        <div className="lg:hidden">
          <PresenceSidebar agents={agents} layout="mobile" />
        </div>
        <MessageList
          messages={messages}
          slug={room.slug}
          onMessagesLoaded={setMessages}
        />
      </div>
      {/* Desktop-only sidebar (hidden below lg) */}
      <aside className="hidden lg:block w-64 shrink-0">
        <PresenceSidebar agents={agents} layout="desktop" />
      </aside>
    </div>
  );
}
