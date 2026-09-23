import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { NewMessagesBadge } from "./new-messages-badge";

describe("NewMessagesBadge", () => {
  it("renders nothing when there are no unread messages", () => {
    const { container } = render(<NewMessagesBadge count={0} onClick={() => {}} />);
    expect(container.firstChild).toBeNull();
  });

  it("shows a Jump to latest indicator with the unread count", () => {
    render(<NewMessagesBadge count={3} onClick={() => {}} />);
    expect(screen.getByText(/jump to latest/i)).toBeInTheDocument();
    expect(screen.getByText(/3/)).toBeInTheDocument();
  });

  it("invokes onClick when pressed", () => {
    const onClick = vi.fn();
    render(<NewMessagesBadge count={1} onClick={onClick} />);
    fireEvent.click(screen.getByRole("button"));
    expect(onClick).toHaveBeenCalledOnce();
  });
});
