import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { SseStatusBadge } from "./sse-status-badge";

// Task "Provide bounded waiting and clear recovery instructions": the web UI must
// DISTINGUISH browser reconnecting from a give-up API failure. The transport
// badge is the surface for both, plus the healthy live state.
describe("SseStatusBadge", () => {
  it("shows a live indicator when connected", () => {
    render(<SseStatusBadge status="connected" />);
    expect(screen.getByText(/live/i)).toBeInTheDocument();
  });

  it("shows a transient reconnecting indicator", () => {
    render(<SseStatusBadge status="reconnecting" />);
    expect(screen.getByText(/reconnecting/i)).toBeInTheDocument();
  });

  it("distinguishes a permanent API failure from reconnecting", () => {
    render(<SseStatusBadge status="disconnected" />);
    // A distinct offline/failure message, NOT the transient reconnecting one.
    expect(screen.getByText(/live updates offline/i)).toBeInTheDocument();
    expect(screen.queryByText(/reconnecting/i)).not.toBeInTheDocument();
  });

  it("renders nothing during the initial connecting phase", () => {
    const { container } = render(<SseStatusBadge status="connecting" />);
    expect(container).toBeEmptyDOMElement();
  });
});
