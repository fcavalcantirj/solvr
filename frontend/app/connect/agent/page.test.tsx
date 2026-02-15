import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";

// Mock next/navigation
const mockPush = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
}));

// Mock useAuth
let mockAuthState: {
  isAuthenticated: boolean;
  isLoading: boolean;
  user: { id: string; username: string } | null;
} = {
  isAuthenticated: false,
  isLoading: true,
  user: null,
};

vi.mock("@/hooks/use-auth", () => ({
  useAuth: () => mockAuthState,
}));

import ConnectAgentPage from "./page";

describe("ConnectAgentPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuthState = {
      isAuthenticated: false,
      isLoading: true,
      user: null,
    };
  });

  it("shows loading state while auth is loading", () => {
    mockAuthState = { isAuthenticated: false, isLoading: true, user: null };
    render(<ConnectAgentPage />);
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("redirects authenticated users to /settings/agents", () => {
    mockAuthState = {
      isAuthenticated: true,
      isLoading: false,
      user: { id: "user-1", username: "testuser" },
    };
    render(<ConnectAgentPage />);
    expect(mockPush).toHaveBeenCalledWith("/settings/agents");
  });

  it("redirects unauthenticated users to /login?next=/settings/agents", () => {
    mockAuthState = { isAuthenticated: false, isLoading: false, user: null };
    render(<ConnectAgentPage />);
    expect(mockPush).toHaveBeenCalledWith("/login?next=/settings/agents");
  });
});
