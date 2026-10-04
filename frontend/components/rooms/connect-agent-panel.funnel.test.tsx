import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { ConnectAgentPanel } from "./connect-agent-panel";
import type { APIRoom, APIRoomConnectResponse } from "@/lib/api-types";

// Recruiting another agent reports one BROWSER funnel step, join_prompt_copied,
// only after the join prompt is copied — for the role it recruits.

const getRoomConnectMock = vi.fn();
const postFunnelEventMock = vi.fn();

vi.mock("@/lib/api", () => ({
  api: {
    getRoomConnect: (...args: unknown[]) => getRoomConnectMock(...args),
    postFunnelEvent: (...args: unknown[]) => postFunnelEventMock(...args),
  },
}));

const writeTextMock = vi.fn().mockResolvedValue(undefined);
Object.assign(navigator, { clipboard: { writeText: writeTextMock } });

const room: APIRoom = {
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
};

const envelope: APIRoomConnectResponse = {
  data: {
    instruction_version: "2026-09-01",
    room_slug: "help-with-hermes-agent",
    room_url: "https://solvr.dev/rooms/help-with-hermes-agent",
    private: false,
    task: "Fix the Hermes agent buffer size",
    prompt: {
      text: "Learn Solvr from https://solvr.dev/skill.md. Join the public Solvr room.",
      segments: [{ kind: "text", text: "Learn Solvr from https://solvr.dev/skill.md. Join the public Solvr room." }],
      word_count: 10,
    },
    role: "collaborator",
  },
};

describe("ConnectAgentPanel connection funnel", () => {
  beforeEach(() => {
    getRoomConnectMock.mockReset();
    postFunnelEventMock.mockReset();
    writeTextMock.mockClear();
  });

  it("reports join_prompt_copied after the join prompt is copied", async () => {
    getRoomConnectMock.mockResolvedValue(envelope);
    render(<ConnectAgentPanel room={room} />);

    fireEvent.click(
      screen.getByRole("button", { name: /connect an agent|get join prompt/i }),
    );
    await screen.findByTestId("join-prompt");

    fireEvent.click(screen.getByRole("button", { name: /copy join prompt/i }));

    await waitFor(() => {
      expect(writeTextMock).toHaveBeenCalledWith(envelope.data.prompt.text);
    });
    await waitFor(() => {
      expect(postFunnelEventMock).toHaveBeenCalledWith(
        expect.objectContaining({
          event: "join_prompt_copied",
          role: "collaborator",
          entry_surface: "room_page",
        }),
      );
    });
  });
});
