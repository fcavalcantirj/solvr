import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { MessageList } from "./message-list";
import type { APIRoomMessage } from "@/lib/api-types";

vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

vi.mock("@/lib/api", () => ({
  api: {
    fetchRoomMessages: vi.fn(),
  },
}));

vi.mock("@/components/shared/markdown-content", () => ({
  MarkdownContent: ({ content }: { content: string }) => <div>{content}</div>,
}));

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

describe("MessageList", () => {
  it("renders messages from the initial prop", () => {
    const messages = [makeMsg(1, "hello world")];
    render(<MessageList messages={messages} slug="room-1" />);
    expect(screen.getByText("hello world")).toBeInTheDocument();
  });

  it("reflects messages prop updates when new messages arrive via rerender", () => {
    const first = [makeMsg(1, "first message")];
    const { rerender } = render(<MessageList messages={first} slug="room-1" />);
    expect(screen.getByText("first message")).toBeInTheDocument();

    // Simulate SSE: parent passes a new messages array with an additional message prepended
    const updated = [makeMsg(2, "second message"), makeMsg(1, "first message")];
    rerender(<MessageList messages={updated} slug="room-1" />);

    // Both messages must be visible — this is the prop-shadowing bug
    expect(screen.getByText("first message")).toBeInTheDocument();
    expect(screen.getByText("second message")).toBeInTheDocument();
  });

  it("renders newly arriving messages when starting from an empty list", () => {
    const { rerender } = render(<MessageList messages={[]} slug="room-1" />);
    expect(screen.queryByText("first real message")).not.toBeInTheDocument();

    rerender(<MessageList messages={[makeMsg(42, "first real message")]} slug="room-1" />);
    expect(screen.getByText("first real message")).toBeInTheDocument();
  });

  it("reads oldest to newest from top to bottom regardless of prop order", () => {
    const messages = [makeMsg(3, "c"), makeMsg(1, "a"), makeMsg(2, "b")];
    const { container } = render(<MessageList messages={messages} slug="room-1" />);
    const ids = Array.from(container.querySelectorAll("[data-message-id]")).map((el) =>
      el.getAttribute("data-message-id"),
    );
    expect(ids).toEqual(["1", "2", "3"]);
  });

  it("de-duplicates messages by persistent id (refresh/replay/local echo)", () => {
    const messages = [makeMsg(1, "a"), makeMsg(1, "a-again"), makeMsg(2, "b")];
    const { container } = render(<MessageList messages={messages} slug="room-1" />);
    expect(container.querySelectorAll("[data-message-id]")).toHaveLength(2);
  });

  it("shows a Load older control only when there is earlier history and calls back on click", () => {
    const onLoadOlder = vi.fn();
    const { rerender } = render(
      <MessageList messages={[makeMsg(5, "x")]} slug="room-1" />,
    );
    expect(screen.queryByText(/load older/i)).not.toBeInTheDocument();

    rerender(
      <MessageList
        messages={[makeMsg(5, "x")]}
        slug="room-1"
        hasOlder
        onLoadOlder={onLoadOlder}
      />,
    );
    fireEvent.click(screen.getByText(/load older/i));
    expect(onLoadOlder).toHaveBeenCalledOnce();
  });

  it("marks the deep-linked message as highlighted", () => {
    const { container } = render(
      <MessageList messages={[makeMsg(1, "a"), makeMsg(2, "b")]} slug="room-1" highlightId={2} />,
    );
    const highlighted = container.querySelector('[data-highlighted="true"]');
    expect(highlighted?.getAttribute("data-message-id")).toBe("2");
  });
});
