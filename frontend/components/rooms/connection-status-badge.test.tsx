import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { ConnectionStatusBadge } from "./connection-status-badge";

describe("ConnectionStatusBadge", () => {
  // The badge renders the API-provided connection_status verbatim. It receives
  // ONLY the status string — never the presence list — so it cannot (and must
  // not) recompute the room's progress on the client. The API is smart; the
  // client is dumb.
  it("renders the human label for each server status", () => {
    const cases: Array<[string, string]> = [
      ["waiting_for_agents", "Waiting for agents"],
      ["waiting_for_another_agent", "Waiting for another agent"],
      ["conversation_started", "Conversation started"],
    ];
    for (const [status, label] of cases) {
      const { unmount } = render(<ConnectionStatusBadge status={status} />);
      expect(screen.getByRole("status")).toHaveTextContent(label);
      unmount();
    }
  });

  it("shows a restrained green live indicator only once the conversation has started", () => {
    const { rerender } = render(
      <ConnectionStatusBadge status="waiting_for_agents" />,
    );
    expect(screen.queryByTestId("connection-live-dot")).toBeNull();

    rerender(<ConnectionStatusBadge status="conversation_started" />);
    // Text accompanies the indicator (never colour alone).
    expect(screen.getByRole("status")).toHaveTextContent("Conversation started");
    expect(screen.getByTestId("connection-live-dot")).toBeInTheDocument();
  });

  it("renders nothing for an unknown or missing status rather than inventing one", () => {
    const { container, rerender } = render(
      <ConnectionStatusBadge status={undefined} />,
    );
    expect(container).toBeEmptyDOMElement();

    rerender(<ConnectionStatusBadge status="something_else" />);
    expect(container).toBeEmptyDOMElement();
  });
});
