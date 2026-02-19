import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import DashboardPage from "./page";

// Mock Next.js navigation
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => "/dashboard",
}));

// Mock auth hook
const mockUseAuth = vi.fn();
vi.mock("@/hooks/use-auth", () => ({
  useAuth: () => mockUseAuth(),
}));

// Mock API
const mockGetUserAgents = vi.fn();
const mockGetAgentBriefing = vi.fn();
const mockGetAgentPins = vi.fn();
const mockGetAgentStorage = vi.fn();
vi.mock("@/lib/api", () => ({
  api: {
    getUserAgents: (...args: unknown[]) => mockGetUserAgents(...args),
    getAgentBriefing: (...args: unknown[]) => mockGetAgentBriefing(...args),
    getAgentPins: (...args: unknown[]) => mockGetAgentPins(...args),
    getAgentStorage: (...args: unknown[]) => mockGetAgentStorage(...args),
  },
}));

// Mock Header and Footer to avoid complexity
vi.mock("@/components/header", () => ({
  Header: () => <div data-testid="header">Header</div>,
}));
vi.mock("@/components/footer", () => ({
  Footer: () => <div data-testid="footer">Footer</div>,
}));

// Mock AgentBriefing to check what props it receives
vi.mock("@/components/agents/agent-briefing", () => ({
  AgentBriefing: (props: { inbox?: unknown; myOpenItems?: unknown; opportunities?: unknown; reputationChanges?: unknown }) => (
    <div data-testid="agent-briefing">
      {props.inbox ? <span>has-inbox</span> : null}
      {props.myOpenItems ? <span>has-open-items</span> : null}
      {props.opportunities ? <span>has-opportunities</span> : null}
      {props.reputationChanges ? <span>has-reputation</span> : null}
    </div>
  ),
}));

const mockAgent = {
  id: "agent_test_1",
  display_name: "Test Agent",
  reputation: 100,
  status: "active",
  model: "claude-opus-4",
  has_human_backed_badge: true,
};

const mockBriefingData = {
  agent_id: "agent_test_1",
  display_name: "Test Agent",
  inbox: { unread_count: 2, items: [{ type: "answer_created", title: "New answer" }] },
  my_open_items: { problems_no_approaches: 1, questions_no_answers: 0, approaches_stale: 0, items: [] },
  suggested_actions: [],
  opportunities: { problems_in_my_domain: 3, items: [] },
  reputation_changes: { since_last_check: "+15", breakdown: [] },
};

const mockPinsData = {
  count: 3,
  results: [
    { requestid: "pin-1", status: "pinned", created: "2025-01-01T00:00:00Z", pin: { cid: "QmTest1" }, delegates: [] },
  ],
};

const mockStorageData = {
  data: { used: 52428800, quota: 1073741824, percentage: 4.88 },
};

describe("DashboardPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows login prompt when not authenticated", async () => {
    mockUseAuth.mockReturnValue({
      user: null,
      isAuthenticated: false,
      isLoading: false,
    });

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText(/log in to view/i)).toBeInTheDocument();
    });
  });

  it("shows empty state when human has no claimed agents", async () => {
    mockUseAuth.mockReturnValue({
      user: { id: "user-1", type: "human", displayName: "Test Human" },
      isAuthenticated: true,
      isLoading: false,
    });
    mockGetUserAgents.mockResolvedValue({ data: [] });

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText(/no claimed agents/i)).toBeInTheDocument();
    });
  });

  it("renders agent briefing when human has claimed agents", async () => {
    mockUseAuth.mockReturnValue({
      user: { id: "user-1", type: "human", displayName: "Test Human" },
      isAuthenticated: true,
      isLoading: false,
    });
    mockGetUserAgents.mockResolvedValue({ data: [mockAgent] });
    mockGetAgentBriefing.mockResolvedValue({ data: mockBriefingData });
    mockGetAgentPins.mockResolvedValue(mockPinsData);
    mockGetAgentStorage.mockResolvedValue(mockStorageData);

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText("Test Agent")).toBeInTheDocument();
    });

    // AgentBriefing component should be rendered with briefing data
    expect(screen.getByTestId("agent-briefing")).toBeInTheDocument();
    expect(screen.getByText("has-inbox")).toBeInTheDocument();
    expect(screen.getByText("has-open-items")).toBeInTheDocument();
    expect(screen.getByText("has-opportunities")).toBeInTheDocument();
    expect(screen.getByText("has-reputation")).toBeInTheDocument();
  });

  it("fetches briefing for each claimed agent", async () => {
    mockUseAuth.mockReturnValue({
      user: { id: "user-1", type: "human", displayName: "Test Human" },
      isAuthenticated: true,
      isLoading: false,
    });

    const agent2 = { ...mockAgent, id: "agent_test_2", display_name: "Agent Two" };
    mockGetUserAgents.mockResolvedValue({ data: [mockAgent, agent2] });
    mockGetAgentBriefing.mockResolvedValue({ data: mockBriefingData });
    mockGetAgentPins.mockResolvedValue(mockPinsData);
    mockGetAgentStorage.mockResolvedValue(mockStorageData);

    render(<DashboardPage />);

    await waitFor(() => {
      expect(mockGetAgentBriefing).toHaveBeenCalledWith("agent_test_1");
      expect(mockGetAgentBriefing).toHaveBeenCalledWith("agent_test_2");
    });
  });

  it("shows error when briefing fetch fails for an agent", async () => {
    mockUseAuth.mockReturnValue({
      user: { id: "user-1", type: "human", displayName: "Test Human" },
      isAuthenticated: true,
      isLoading: false,
    });
    mockGetUserAgents.mockResolvedValue({ data: [mockAgent] });
    mockGetAgentBriefing.mockRejectedValue(new Error("Forbidden"));
    mockGetAgentPins.mockResolvedValue(mockPinsData);
    mockGetAgentStorage.mockResolvedValue(mockStorageData);

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText("Forbidden")).toBeInTheDocument();
    });
  });

  it("shows AGENT DASHBOARD heading", async () => {
    mockUseAuth.mockReturnValue({
      user: { id: "user-1", type: "human", displayName: "Test Human" },
      isAuthenticated: true,
      isLoading: false,
    });
    mockGetUserAgents.mockResolvedValue({ data: [mockAgent] });
    mockGetAgentBriefing.mockResolvedValue({ data: mockBriefingData });
    mockGetAgentPins.mockResolvedValue(mockPinsData);
    mockGetAgentStorage.mockResolvedValue(mockStorageData);

    render(<DashboardPage />);

    expect(screen.getByText("AGENT DASHBOARD")).toBeInTheDocument();
  });

  it("links agent name to agent profile page", async () => {
    mockUseAuth.mockReturnValue({
      user: { id: "user-1", type: "human", displayName: "Test Human" },
      isAuthenticated: true,
      isLoading: false,
    });
    mockGetUserAgents.mockResolvedValue({ data: [mockAgent] });
    mockGetAgentBriefing.mockResolvedValue({ data: mockBriefingData });
    mockGetAgentPins.mockResolvedValue(mockPinsData);
    mockGetAgentStorage.mockResolvedValue(mockStorageData);

    render(<DashboardPage />);

    await waitFor(() => {
      const agentLink = screen.getByRole("link", { name: "Test Agent" });
      expect(agentLink).toHaveAttribute("href", "/agents/agent_test_1");
    });
  });

  it("renders agent storage usage", async () => {
    mockUseAuth.mockReturnValue({
      user: { id: "user-1", type: "human", displayName: "Test Human" },
      isAuthenticated: true,
      isLoading: false,
    });
    mockGetUserAgents.mockResolvedValue({ data: [mockAgent] });
    mockGetAgentBriefing.mockResolvedValue({ data: mockBriefingData });
    mockGetAgentPins.mockResolvedValue(mockPinsData);
    mockGetAgentStorage.mockResolvedValue(mockStorageData);

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText(/50\.0 MB/)).toBeInTheDocument();
      expect(screen.getByText(/1\.0 GB/)).toBeInTheDocument();
    });
  });

  it("renders agent pin count", async () => {
    mockUseAuth.mockReturnValue({
      user: { id: "user-1", type: "human", displayName: "Test Human" },
      isAuthenticated: true,
      isLoading: false,
    });
    mockGetUserAgents.mockResolvedValue({ data: [mockAgent] });
    mockGetAgentBriefing.mockResolvedValue({ data: mockBriefingData });
    mockGetAgentPins.mockResolvedValue(mockPinsData);
    mockGetAgentStorage.mockResolvedValue(mockStorageData);

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText(/3 pins/i)).toBeInTheDocument();
    });
  });

  it("handles storage fetch error gracefully", async () => {
    mockUseAuth.mockReturnValue({
      user: { id: "user-1", type: "human", displayName: "Test Human" },
      isAuthenticated: true,
      isLoading: false,
    });
    mockGetUserAgents.mockResolvedValue({ data: [mockAgent] });
    mockGetAgentBriefing.mockResolvedValue({ data: mockBriefingData });
    mockGetAgentPins.mockResolvedValue(mockPinsData);
    mockGetAgentStorage.mockRejectedValue(new Error("Storage unavailable"));

    render(<DashboardPage />);

    // Should still render agent name and briefing even if storage fails
    await waitFor(() => {
      expect(screen.getByText("Test Agent")).toBeInTheDocument();
    });
    expect(screen.getByTestId("agent-briefing")).toBeInTheDocument();
  });

  it("handles pins fetch error gracefully", async () => {
    mockUseAuth.mockReturnValue({
      user: { id: "user-1", type: "human", displayName: "Test Human" },
      isAuthenticated: true,
      isLoading: false,
    });
    mockGetUserAgents.mockResolvedValue({ data: [mockAgent] });
    mockGetAgentBriefing.mockResolvedValue({ data: mockBriefingData });
    mockGetAgentPins.mockRejectedValue(new Error("Pins unavailable"));
    mockGetAgentStorage.mockResolvedValue(mockStorageData);

    render(<DashboardPage />);

    // Should still render agent name and briefing even if pins fails
    await waitFor(() => {
      expect(screen.getByText("Test Agent")).toBeInTheDocument();
    });
    expect(screen.getByTestId("agent-briefing")).toBeInTheDocument();
  });

  it("renders agents in reputation order (highest first)", async () => {
    mockUseAuth.mockReturnValue({
      user: { id: "user-1", type: "human", displayName: "Test Human" },
      isAuthenticated: true,
      isLoading: false,
    });

    const lowRepAgent = { ...mockAgent, id: "agent_low", display_name: "Low Rep Agent", reputation: 10 };
    const midRepAgent = { ...mockAgent, id: "agent_mid", display_name: "Mid Rep Agent", reputation: 50 };
    const highRepAgent = { ...mockAgent, id: "agent_high", display_name: "High Rep Agent", reputation: 200 };

    // API returns in wrong order (backend will fix this, but frontend should preserve order)
    mockGetUserAgents.mockResolvedValue({ data: [highRepAgent, midRepAgent, lowRepAgent] });
    mockGetAgentBriefing.mockResolvedValue({ data: mockBriefingData });
    mockGetAgentPins.mockResolvedValue(mockPinsData);
    mockGetAgentStorage.mockResolvedValue(mockStorageData);

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText("High Rep Agent")).toBeInTheDocument();
    });

    // Get all agent name links and verify order
    const links = screen.getAllByRole("link", { name: /Rep Agent/ });
    expect(links[0]).toHaveTextContent("High Rep Agent");
    expect(links[1]).toHaveTextContent("Mid Rep Agent");
    expect(links[2]).toHaveTextContent("Low Rep Agent");
  });
});
