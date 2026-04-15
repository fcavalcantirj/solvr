import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { RoomDetailClient } from "./room-detail-client";
import type {
  APIRoom,
  APIRoomMessage,
  APIAgentPresenceRecord,
} from "@/lib/api-types";

vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

vi.mock("@/lib/api", () => ({
  api: {
    fetchRoomMessages: vi.fn(),
    postRoomMessage: vi.fn(),
  },
}));

vi.mock("@/hooks/use-auth", () => ({
  useAuth: () => ({ user: null, isAuthenticated: false, loading: false }),
}));

vi.mock("@/components/shared/markdown-content", () => ({
  MarkdownContent: ({ content }: { content: string }) => <div>{content}</div>,
}));

// Mutable state so tests can drive hook return values across rerenders.
type SseState = {
  status: string;
  newMessages: APIRoomMessage[];
  presenceJoins: APIAgentPresenceRecord[];
  presenceLeaves: string[];
};

const sseState: SseState = {
  status: "connected",
  newMessages: [],
  presenceJoins: [],
  presenceLeaves: [],
};

const clearNewMessagesMock = vi.fn(() => {
  sseState.newMessages = [];
});
const clearPresenceEventsMock = vi.fn(() => {
  sseState.presenceJoins = [];
  sseState.presenceLeaves = [];
});

vi.mock("@/hooks/use-room-sse", () => ({
  useRoomSse: () => ({
    status: sseState.status,
    newMessages: sseState.newMessages,
    presenceJoins: sseState.presenceJoins,
    presenceLeaves: sseState.presenceLeaves,
    clearNewMessages: clearNewMessagesMock,
    clearPresenceEvents: clearPresenceEventsMock,
  }),
}));

function resetSseMock() {
  sseState.status = "connected";
  sseState.newMessages = [];
  sseState.presenceJoins = [];
  sseState.presenceLeaves = [];
  clearNewMessagesMock.mockClear();
  clearPresenceEventsMock.mockClear();
}

function makeRoom(overrides: Partial<APIRoom> = {}): APIRoom {
  return {
    id: "room-1",
    slug: "help-with-hermes-agent",
    display_name: "Help with Hermes Agent",
    description: "help a fellow agent about hermes",
    category: "agents",
    tags: ["hermes", "agents"],
    is_private: false,
    owner_id: "user-1",
    message_count: 0,
    created_at: new Date("2026-04-15T20:00:00Z").toISOString(),
    updated_at: new Date("2026-04-15T20:00:00Z").toISOString(),
    last_active_at: new Date("2026-04-15T20:00:00Z").toISOString(),
    ...overrides,
  };
}

function makeMsg(id: number, content: string): APIRoomMessage {
  return {
    id,
    room_id: "room-1",
    author_type: "agent",
    agent_name: `agent-${id}`,
    content,
    content_type: "text",
    metadata: {},
    created_at: new Date("2026-04-15T20:00:00Z").toISOString(),
  };
}

function makeAgent(name: string): APIAgentPresenceRecord {
  return {
    id: `agent-${name}`,
    room_id: "room-1",
    agent_name: name,
    card_json: {},
    joined_at: new Date("2026-04-15T20:00:00Z").toISOString(),
    last_seen: new Date("2026-04-15T20:00:00Z").toISOString(),
    ttl_seconds: 600,
  };
}

describe("RoomDetailClient — effective message count", () => {
  beforeEach(() => resetSseMock());

  it("displays messages.length when it exceeds room.message_count (stale SSR snapshot)", () => {
    // SSR says 0 messages, but the client received 2 messages via initialMessages
    // (e.g., a cached page with stale message_count but fresh recent_messages,
    // or SSE hydration already happened before count refresh).
    const room = makeRoom({ message_count: 0 });
    const initialMessages = [makeMsg(1, "first"), makeMsg(2, "second")];

    render(
      <RoomDetailClient
        room={room}
        initialMessages={initialMessages}
        initialAgents={[makeAgent("alice")]}
      />,
    );

    // Both header ("2 messages") and sidebar ("ROOM INFO > Messages: 2") must
    // show the effective count (2), not the stale SSR snapshot (0).
    expect(screen.getByText("2 messages")).toBeInTheDocument();
    const counts = screen.getAllByText("2");
    expect(counts.length).toBeGreaterThan(0);
  });

  it("keeps room.message_count when it is already higher than messages.length", () => {
    // Only 1 recent message was hydrated (SSR window), but the DB counter says 42
    const room = makeRoom({ message_count: 42 });
    render(
      <RoomDetailClient
        room={room}
        initialMessages={[makeMsg(99, "latest")]}
        initialAgents={[]}
      />,
    );

    // Should display 42, not 1 — we take the MAX of the two sources
    expect(screen.getAllByText("42").length).toBeGreaterThan(0);
  });
});

describe("RoomDetailClient — presence event handling", () => {
  beforeEach(() => resetSseMock());

  it("does not drop a rejoined agent when an UNRELATED leave fires later", () => {
    // Regression test for the cumulative-presenceLeaves bug:
    //   1. Alice leaves -> removed from agents list.
    //   2. Alice rejoins via presence_join -> added back.
    //   3. Bob (unrelated) leaves -> hook state is now { leaves: [alice, bob] }.
    //   4. The leaves effect re-runs with the full historical list and
    //      incorrectly removes Alice again, even though she is still present.
    //
    // Fix: the hook must be cleared after each consumed batch so leaves are
    // processed exactly once. Test drives the hook state directly.
    const room = makeRoom();
    const alice = makeAgent("alice");
    const bob = makeAgent("bob");

    // Agent names render in BOTH the mobile presence strip and the desktop
    // sidebar simultaneously in JSDOM (responsive classes don't affect DOM
    // presence), so use getAllByText / queryAllByText throughout.
    const hasName = (name: string) => screen.queryAllByText(name).length > 0;

    // Initial: Alice + Bob present.
    sseState.presenceJoins = [];
    sseState.presenceLeaves = [];
    const { rerender } = render(
      <RoomDetailClient
        room={room}
        initialMessages={[]}
        initialAgents={[alice, bob]}
      />,
    );
    expect(hasName("alice")).toBe(true);
    expect(hasName("bob")).toBe(true);

    // Batch 1: Alice leaves.
    sseState.presenceLeaves = ["alice"];
    rerender(
      <RoomDetailClient
        room={room}
        initialMessages={[]}
        initialAgents={[alice, bob]}
      />,
    );
    expect(hasName("alice")).toBe(false);
    expect(hasName("bob")).toBe(true);
    // The parent should have consumed and cleared the batch.
    expect(clearPresenceEventsMock).toHaveBeenCalled();

    // Batch 2: Alice rejoins (hook batch is just the join; leaves was cleared).
    clearPresenceEventsMock.mockClear();
    sseState.presenceJoins = [alice];
    sseState.presenceLeaves = [];
    rerender(
      <RoomDetailClient
        room={room}
        initialMessages={[]}
        initialAgents={[alice, bob]}
      />,
    );
    expect(hasName("alice")).toBe(true);
    expect(hasName("bob")).toBe(true);

    // Batch 3: Bob leaves (unrelated). Leaves batch must only contain Bob.
    clearPresenceEventsMock.mockClear();
    sseState.presenceJoins = [];
    sseState.presenceLeaves = ["bob"];
    rerender(
      <RoomDetailClient
        room={room}
        initialMessages={[]}
        initialAgents={[alice, bob]}
      />,
    );
    // Alice must still be present — this is the regression we are guarding.
    expect(hasName("alice")).toBe(true);
    expect(hasName("bob")).toBe(false);
  });
});
