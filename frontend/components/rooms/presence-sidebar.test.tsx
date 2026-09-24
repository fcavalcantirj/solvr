import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { PresenceSidebar } from "./presence-sidebar";
import type { APIRoom, APIAgentPresenceRecord } from "@/lib/api-types";

const rotateRoomTokenMock = vi.fn();
const useAuthMock = vi.fn();

vi.mock("@/lib/api", () => ({
  api: {
    rotateRoomToken: (...args: unknown[]) => rotateRoomTokenMock(...args),
  },
}));

vi.mock("@/hooks/use-auth", () => ({
  useAuth: () => useAuthMock(),
}));

const writeTextMock = vi.fn().mockResolvedValue(undefined);
Object.assign(navigator, {
  clipboard: { writeText: writeTextMock },
});

function makeRoom(overrides: Partial<APIRoom> = {}): APIRoom {
  return {
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
    ...overrides,
  };
}

function agents(): APIAgentPresenceRecord[] {
  return [];
}

describe("PresenceSidebar — no shared room token", () => {
  beforeEach(() => {
    rotateRoomTokenMock.mockReset();
    writeTextMock.mockClear();
    useAuthMock.mockReset();
  });

  // Replaces the rotate-and-copy CONNECT AGENT card. Agents connect through the
  // API-owned join prompt (ConnectAgentPanel, rendered beside this sidebar), where each
  // agent takes its OWN room token by handshake; nothing here rotates or copies the
  // shared room token, for any viewer.
  it.each([
    ["the room owner", { id: "owner-uuid", type: "human", displayName: "Felipe" }],
    ["a non-owner", { id: "other-user", type: "human", displayName: "Bob" }],
    ["an anonymous visitor", null],
  ])("never rotates or copies a shared room token for %s", (_label, user) => {
    useAuthMock.mockReturnValue({
      user,
      isAuthenticated: user !== null,
      isLoading: false,
    });

    const room = makeRoom({ owner_id: "owner-uuid" });
    const { container } = render(
      <PresenceSidebar agents={agents()} room={room} layout="desktop" />,
    );

    expect(screen.queryByRole("button", { name: /COPY A2A PROMPT/i })).toBeNull();
    expect(screen.queryByText(/rotates the bearer token/i)).toBeNull();
    expect(container.textContent).not.toMatch(/YOUR_ROOM_TOKEN|bearer token from the room owner/i);
    expect(rotateRoomTokenMock).not.toHaveBeenCalled();
    expect(writeTextMock).not.toHaveBeenCalled();
  });
});
