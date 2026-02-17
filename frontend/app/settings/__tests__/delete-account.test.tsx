import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";

// Create mock functions using vi.hoisted for proper hoisting
const { mockDeleteMe, mockPush } = vi.hoisted(() => {
  return {
    mockDeleteMe: vi.fn(),
    mockPush: vi.fn(),
  };
});

// Mock the hooks
vi.mock("@/hooks/use-auth", () => ({
  useAuth: vi.fn(() => ({
    user: {
      id: "user_123",
      displayName: "Test User",
      email: "test@example.com",
      type: "human",
    },
    isLoading: false,
    isAuthenticated: true,
  })),
}));

vi.mock("@/hooks/use-profile-edit", () => ({
  useProfileEdit: vi.fn(() => ({
    saving: false,
    error: null,
    success: false,
    updateProfile: vi.fn(),
    clearStatus: vi.fn(),
  })),
}));

vi.mock("@/hooks/use-auth-methods", () => ({
  useAuthMethods: vi.fn(() => ({
    authMethods: [
      { provider: "google", linked_at: "2024-01-01T00:00:00Z" },
    ],
    loading: false,
  })),
}));

// Mock the API
vi.mock("@/lib/api", () => ({
  api: {
    deleteMe: mockDeleteMe,
  },
}));

// Mock next/navigation
vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: mockPush,
  }),
  usePathname: () => "/settings",
}));

// Import after all mocks are set up
import SettingsPage from "../page";

describe("Delete Account Feature", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders delete button in danger zone", () => {
    render(<SettingsPage />);

    // Check for danger zone section
    expect(screen.getByText("DANGER ZONE")).toBeInTheDocument();

    // Check for delete button
    expect(screen.getByText("DELETE MY ACCOUNT")).toBeInTheDocument();
  });

  it("clicking delete shows confirmation dialog", async () => {
    render(<SettingsPage />);

    // Click delete button
    const deleteButton = screen.getByText("DELETE MY ACCOUNT");
    fireEvent.click(deleteButton);

    // Check for confirmation dialog
    await waitFor(() => {
      expect(screen.getByText("Are you sure?")).toBeInTheDocument();
    });

    // Check for description
    expect(
      screen.getByText(/This will permanently delete your account/i)
    ).toBeInTheDocument();

    // Check for cancel and confirm buttons
    expect(screen.getByText("Cancel")).toBeInTheDocument();
    expect(screen.getByText("Yes, delete my account")).toBeInTheDocument();
  });

  it("canceling dialog closes without API call", async () => {
    render(<SettingsPage />);

    // Click delete button
    const deleteButton = screen.getByText("DELETE MY ACCOUNT");
    fireEvent.click(deleteButton);

    // Wait for dialog to appear
    await waitFor(() => {
      expect(screen.getByText("Are you sure?")).toBeInTheDocument();
    });

    // Click cancel
    const cancelButton = screen.getByText("Cancel");
    fireEvent.click(cancelButton);

    // Verify API was not called
    expect(mockDeleteMe).not.toHaveBeenCalled();

    // Verify dialog closed
    await waitFor(() => {
      expect(screen.queryByText("Are you sure?")).not.toBeInTheDocument();
    });
  });

  it("confirming dialog calls api.deleteMe() and redirects", async () => {
    mockDeleteMe.mockResolvedValueOnce(undefined);

    render(<SettingsPage />);

    // Click delete button
    const deleteButton = screen.getByText("DELETE MY ACCOUNT");
    fireEvent.click(deleteButton);

    // Wait for dialog to appear
    await waitFor(() => {
      expect(screen.getByText("Are you sure?")).toBeInTheDocument();
    });

    // Click confirm
    const confirmButton = screen.getByText("Yes, delete my account");
    fireEvent.click(confirmButton);

    // Verify API was called
    await waitFor(() => {
      expect(mockDeleteMe).toHaveBeenCalledTimes(1);
    });

    // Verify redirect to landing page
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith("/");
    });
  });

  it("shows error toast when deletion fails", async () => {
    mockDeleteMe.mockRejectedValueOnce(new Error("Failed to delete"));

    render(<SettingsPage />);

    // Click delete button
    const deleteButton = screen.getByText("DELETE MY ACCOUNT");
    fireEvent.click(deleteButton);

    // Wait for dialog to appear
    await waitFor(() => {
      expect(screen.getByText("Are you sure?")).toBeInTheDocument();
    });

    // Click confirm
    const confirmButton = screen.getByText("Yes, delete my account");
    fireEvent.click(confirmButton);

    // Verify error message appears
    await waitFor(() => {
      expect(screen.getByText(/Failed to delete account/i)).toBeInTheDocument();
    });

    // Verify no redirect occurred
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("shows loading state during deletion", async () => {
    // Mock a delayed response
    mockDeleteMe.mockImplementation(
      () => new Promise((resolve) => setTimeout(resolve, 100))
    );

    render(<SettingsPage />);

    // Click delete button
    const deleteButton = screen.getByText("DELETE MY ACCOUNT");
    fireEvent.click(deleteButton);

    // Wait for dialog to appear
    await waitFor(() => {
      expect(screen.getByText("Are you sure?")).toBeInTheDocument();
    });

    // Click confirm
    const confirmButton = screen.getByText("Yes, delete my account");
    fireEvent.click(confirmButton);

    // Verify loading state appears - button should be disabled and show "DELETING..."
    await waitFor(() => {
      const loadingButton = screen.getByRole("button", { name: /DELETING.../i });
      expect(loadingButton).toBeDisabled();
    });
  });
});
