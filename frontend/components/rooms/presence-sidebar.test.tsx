import { render, screen, waitFor, act } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { PresenceSidebar } from "./presence-sidebar";
import type { APIRoom, APIAgentPresenceRecord, APIRoomMember } from "@/lib/api-types";

const rotateRoomTokenMock = vi.fn();
const getMembersMock = vi.fn();
const useAuthMock = vi.fn();

vi.mock("@/lib/api", () => ({
  api: {
    rotateRoomToken: (...args: unknown[]) => rotateRoomTokenMock(...args),
    getMembers: (...args: unknown[]) => getMembersMock(...args),
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

const members: APIRoomMember[] = [
  { room_id: "room-1", agent_id: "agent_planner", role: "owner", added_by: "system", created_at: "2026-04-15T20:00:00Z" },
  { room_id: "room-1", agent_id: "agent_executor", role: "member", added_by: "agent_planner", created_at: "2026-04-15T20:01:00Z" },
];

describe("PresenceSidebar — no shared room token", () => {
  beforeEach(() => {
    rotateRoomTokenMock.mockReset();
    getMembersMock.mockReset();
    getMembersMock.mockResolvedValue({ data: [] });
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
  ])("never rotates or copies a shared room token for %s", async (_label, user) => {
    useAuthMock.mockReturnValue({
      user,
      isAuthenticated: user !== null,
      isLoading: false,
    });

    const room = makeRoom({ owner_id: "owner-uuid" });
    const { container } = render(
      <PresenceSidebar agents={agents()} room={room} layout="desktop" />,
    );
    // Let the member-list read of a signed-in viewer settle.
    await act(async () => {});

    expect(screen.queryByRole("button", { name: /COPY A2A PROMPT/i })).toBeNull();
    expect(screen.queryByText(/rotates the bearer token/i)).toBeNull();
    expect(container.textContent).not.toMatch(/YOUR_ROOM_TOKEN|bearer token from the room owner/i);
    expect(rotateRoomTokenMock).not.toHaveBeenCalled();
    expect(writeTextMock).not.toHaveBeenCalled();
  });
});

// GET /v1/rooms/{slug}/members is an owner-only read: it answers 401 to an anonymous
// caller, and that 401 used to open the login dialog over every public room page.
// The sidebar asks for the list only on behalf of a signed-in viewer.
describe("PresenceSidebar — the member list is read only for a signed-in viewer", () => {
  beforeEach(() => {
    getMembersMock.mockReset();
    getMembersMock.mockResolvedValue({ data: members });
    useAuthMock.mockReset();
  });

  it.each([
    ["an anonymous visitor", { user: null, isAuthenticated: false, isLoading: false }],
    ["a visitor whose session is still being read", { user: null, isAuthenticated: false, isLoading: true }],
  ])("renders the room for %s without asking for the member list", async (_label, auth) => {
    useAuthMock.mockReturnValue(auth);

    render(<PresenceSidebar agents={agents()} room={makeRoom()} layout="desktop" />);

    // Let a stray request go out before asserting there was none.
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(getMembersMock).not.toHaveBeenCalled();
    expect(screen.getByText("ROOM INFO")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /generate role-specific prompt/i })).toBeInTheDocument();
    expect(screen.queryByText("PARTICIPANTS")).toBeNull();
  });

  it("asks for the member list once the viewer is signed in, and shows it", async () => {
    useAuthMock.mockReturnValue({
      user: { id: "owner-uuid", type: "human", displayName: "Felipe" },
      isAuthenticated: true,
      isLoading: false,
    });

    render(<PresenceSidebar agents={agents()} room={makeRoom()} layout="desktop" />);

    expect(await screen.findByText("PARTICIPANTS")).toBeInTheDocument();
    expect(getMembersMock).toHaveBeenCalledTimes(1);
    expect(getMembersMock).toHaveBeenCalledWith("help-with-hermes-agent");
    expect(screen.getByText("agent_executor")).toBeInTheDocument();
  });

  it("keeps the room readable when the API refuses the list to a signed-in non-owner", async () => {
    getMembersMock.mockRejectedValue(new Error("forbidden"));
    useAuthMock.mockReturnValue({
      user: { id: "other-user", type: "human", displayName: "Bob" },
      isAuthenticated: true,
      isLoading: false,
    });

    render(<PresenceSidebar agents={agents()} room={makeRoom()} layout="desktop" />);

    await waitFor(() => expect(getMembersMock).toHaveBeenCalledTimes(1));
    expect(screen.getByText("ROOM INFO")).toBeInTheDocument();
    expect(screen.queryByText("PARTICIPANTS")).toBeNull();
  });
});
