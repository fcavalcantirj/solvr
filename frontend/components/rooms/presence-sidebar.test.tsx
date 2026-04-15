import { render, screen, fireEvent, waitFor } from "@testing-library/react";
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

describe("PresenceSidebar — CONNECT AGENT prompt", () => {
  beforeEach(() => {
    rotateRoomTokenMock.mockReset();
    writeTextMock.mockClear();
    useAuthMock.mockReset();
  });

  it("rotates and copies a REAL bearer token when the room owner clicks", async () => {
    useAuthMock.mockReturnValue({
      user: { id: "owner-uuid", type: "human", displayName: "Felipe" },
      isAuthenticated: true,
      isLoading: false,
    });
    rotateRoomTokenMock.mockResolvedValue({
      data: { token: "real-plain-token-xyz" },
    });

    const room = makeRoom({ owner_id: "owner-uuid" });
    render(<PresenceSidebar agents={agents()} room={room} layout="desktop" />);

    // Owner sees rotate-capable copy button
    const btn = screen.getByRole("button", { name: /COPY A2A PROMPT/i });
    fireEvent.click(btn);

    await waitFor(() => {
      expect(rotateRoomTokenMock).toHaveBeenCalledWith("help-with-hermes-agent");
    });

    await waitFor(() => {
      const copiedText = writeTextMock.mock.calls.at(-1)?.[0] as string;
      expect(copiedText).toContain("Bearer real-plain-token-xyz");
      expect(copiedText).not.toContain("YOUR_ROOM_TOKEN");
    });
  });

  it("copies the placeholder prompt for non-owners (they cannot rotate)", async () => {
    useAuthMock.mockReturnValue({
      user: { id: "other-user", type: "human", displayName: "Bob" },
      isAuthenticated: true,
      isLoading: false,
    });

    const room = makeRoom({ owner_id: "owner-uuid" });
    render(<PresenceSidebar agents={agents()} room={room} layout="desktop" />);

    const btn = screen.getByRole("button", { name: /COPY A2A PROMPT/i });
    fireEvent.click(btn);

    await waitFor(() => {
      expect(writeTextMock).toHaveBeenCalled();
    });

    expect(rotateRoomTokenMock).not.toHaveBeenCalled();
    const copiedText = writeTextMock.mock.calls.at(-1)?.[0] as string;
    expect(copiedText).toContain("YOUR_ROOM_TOKEN");
  });

  it("copies the placeholder prompt for anonymous visitors", async () => {
    useAuthMock.mockReturnValue({
      user: null,
      isAuthenticated: false,
      isLoading: false,
    });

    const room = makeRoom({ owner_id: "owner-uuid" });
    render(<PresenceSidebar agents={agents()} room={room} layout="desktop" />);

    fireEvent.click(screen.getByRole("button", { name: /COPY A2A PROMPT/i }));

    await waitFor(() => {
      expect(writeTextMock).toHaveBeenCalled();
    });

    expect(rotateRoomTokenMock).not.toHaveBeenCalled();
    const copiedText = writeTextMock.mock.calls.at(-1)?.[0] as string;
    expect(copiedText).toContain("YOUR_ROOM_TOKEN");
  });
});
