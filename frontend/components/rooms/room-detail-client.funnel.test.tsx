import { render, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { RoomDetailClient } from "./room-detail-client";
import type { APIRoom } from "@/lib/api-types";

// Opening a room page reports one BROWSER funnel step, room_viewed. It carries no
// room identity, so it is safe for public and private rooms alike.

vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

const postFunnelEventMock = vi.fn();
vi.mock("@/lib/api", () => ({
  api: {
    fetchRoomMessages: vi.fn(),
    fetchRoomMessage: vi.fn(),
    postRoomMessage: vi.fn(),
    getRoomConnect: vi.fn(),
    postFunnelEvent: (...args: unknown[]) => postFunnelEventMock(...args),
  },
}));

vi.mock("@/hooks/use-auth", () => ({
  useAuth: () => ({ user: null, isAuthenticated: false, loading: false }),
}));

vi.mock("@/components/shared/markdown-content", () => ({
  MarkdownContent: ({ content }: { content: string }) => <div>{content}</div>,
}));

vi.mock("@/hooks/use-room-sse", () => ({
  useRoomSse: () => ({
    status: "connected",
    newMessages: [],
    presenceJoins: [],
    presenceLeaves: [],
    clearNewMessages: vi.fn(),
    clearPresenceEvents: vi.fn(),
  }),
}));

const room: APIRoom = {
  id: "room-1",
  slug: "help-with-hermes-agent",
  display_name: "Help with Hermes Agent",
  description: "desc",
  category: "agents",
  tags: ["hermes"],
  is_private: false,
  owner_id: "owner-uuid",
  message_count: 0,
  created_at: new Date("2026-04-15T20:00:00Z").toISOString(),
  updated_at: new Date("2026-04-15T20:00:00Z").toISOString(),
  last_active_at: new Date("2026-04-15T20:00:00Z").toISOString(),
};

describe("RoomDetailClient connection funnel", () => {
  beforeEach(() => {
    postFunnelEventMock.mockReset();
  });

  it("reports room_viewed when the room page mounts", async () => {
    render(<RoomDetailClient room={room} initialMessages={[]} initialAgents={[]} />);

    await waitFor(() => {
      expect(postFunnelEventMock).toHaveBeenCalledWith(
        expect.objectContaining({ event: "room_viewed" }),
      );
    });
  });
});
