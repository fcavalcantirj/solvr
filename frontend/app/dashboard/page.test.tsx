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
vi.mock("@/lib/api", () => ({
  api: {
    getUserAgents: (...args: unknown[]) => mockGetUserAgents(...args),
    getAgentBriefing: (...args: unknown[]) => mockGetAgentBriefing(...args),
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

    render(<DashboardPage />);

    await waitFor(() => {
      const agentLink = screen.getByRole("link", { name: "Test Agent" });
      expect(agentLink).toHaveAttribute("href", "/agents/agent_test_1");
    });
  });
});
