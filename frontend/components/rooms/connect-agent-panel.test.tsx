import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { ConnectAgentPanel } from "./connect-agent-panel";
import type { APIRoom, APIRoomConnectResponse } from "@/lib/api-types";

const getRoomConnectMock = vi.fn();

vi.mock("@/lib/api", () => ({
  api: {
    getRoomConnect: (...args: unknown[]) => getRoomConnectMock(...args),
  },
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
    message_count: 3,
    created_at: new Date("2026-04-15T20:00:00Z").toISOString(),
    updated_at: new Date("2026-04-15T20:00:00Z").toISOString(),
    last_active_at: new Date("2026-04-15T20:00:00Z").toISOString(),
    ...overrides,
  };
}

const collaboratorEnvelope: APIRoomConnectResponse = {
  data: {
    instruction_version: "2026-09-01",
    room_slug: "help-with-hermes-agent",
    room_url: "https://solvr.dev/rooms/help-with-hermes-agent",
    private: false,
    task: "Fix the Hermes agent buffer size",
    expected_planner_identity: "planner-bot",
    executor_prompt: "",
    prompt:
      "You are the Collaborator agent joining an existing Solvr room. " +
      "ROOM: https://solvr.dev/rooms/help-with-hermes-agent " +
      "POST https://api.solvr.dev/v1/agents/register " +
      "POST https://api.solvr.dev/v1/rooms/help-with-hermes-agent/handshake",
    role: "collaborator",
    first_message_id: 42,
    first_message_url: "https://solvr.dev/rooms/help-with-hermes-agent#message-1",
  },
};

describe("ConnectAgentPanel", () => {
  beforeEach(() => {
    getRoomConnectMock.mockReset();
    writeTextMock.mockClear();
  });

  it("reveals the collaborator join prompt for a live public room when clicked", async () => {
    getRoomConnectMock.mockResolvedValue(collaboratorEnvelope);
    render(<ConnectAgentPanel room={makeRoom()} />);

    // The prompt is not fetched until the visitor acts.
    expect(getRoomConnectMock).not.toHaveBeenCalled();

    fireEvent.click(
      screen.getByRole("button", { name: /connect an agent|get join prompt/i }),
    );

    await waitFor(() => {
      expect(getRoomConnectMock).toHaveBeenCalledWith(
        "help-with-hermes-agent",
        "collaborator",
      );
    });

    // The copyable prompt appears, bound to the real room, with the Collaborator role.
    const prompt = await screen.findByTestId("join-prompt");
    expect(prompt.textContent).toContain(
      "joining an existing Solvr room",
    );
    expect(prompt.textContent).toContain(
      "https://solvr.dev/rooms/help-with-hermes-agent",
    );
    expect(screen.getByTestId("connect-role").textContent?.toLowerCase()).toContain(
      "collaborator",
    );
    expect(screen.getByTestId("connect-room-url")).toHaveAttribute(
      "href",
      "https://solvr.dev/rooms/help-with-hermes-agent",
    );
    // Existing task context is shown.
    expect(screen.getByText(/Fix the Hermes agent buffer size/)).toBeInTheDocument();
  });

  it("copies the join prompt to the clipboard", async () => {
    getRoomConnectMock.mockResolvedValue(collaboratorEnvelope);
    render(<ConnectAgentPanel room={makeRoom()} />);

    fireEvent.click(
      screen.getByRole("button", { name: /connect an agent|get join prompt/i }),
    );
    await screen.findByTestId("join-prompt");

    fireEvent.click(screen.getByRole("button", { name: /copy join prompt/i }));

    await waitFor(() => {
      expect(writeTextMock).toHaveBeenCalledWith(collaboratorEnvelope.data.prompt);
    });
    expect(await screen.findByText(/copied/i)).toBeInTheDocument();
  });

  it("offers starting a new room for a finished (archived) room instead of joining", () => {
    render(
      <ConnectAgentPanel
        room={makeRoom({ archived_at: new Date("2026-04-16T10:00:00Z").toISOString() })}
      />,
    );

    // A finished room cannot be joined: no prompt fetch, an explicit new-room path.
    const newRoom = screen.getByRole("link", { name: /start a new room/i });
    expect(newRoom).toHaveAttribute("href", "/connect");
    expect(screen.queryByTestId("join-prompt")).not.toBeInTheDocument();
    expect(getRoomConnectMock).not.toHaveBeenCalled();
  });

  it("shows an error with retry when the prompt cannot load", async () => {
    getRoomConnectMock.mockRejectedValueOnce(new Error("network"));
    render(<ConnectAgentPanel room={makeRoom()} />);

    fireEvent.click(
      screen.getByRole("button", { name: /connect an agent|get join prompt/i }),
    );

    const retry = await screen.findByRole("button", { name: /retry/i });
    expect(screen.getByText(/could not load/i)).toBeInTheDocument();

    getRoomConnectMock.mockResolvedValueOnce(collaboratorEnvelope);
    fireEvent.click(retry);
    expect(await screen.findByTestId("join-prompt")).toBeInTheDocument();
  });
});
