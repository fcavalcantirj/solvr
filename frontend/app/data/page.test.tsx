import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, act } from "@testing-library/react";
import DataPage from "./page";

// Mock Header component to avoid AuthProvider dependency
vi.mock("@/components/header", () => ({
  Header: () => <header data-testid="header" />,
}));

// Mock useAuth hook
vi.mock("@/hooks/use-auth", () => ({
  useAuth: vi.fn(() => ({
    isAuthenticated: false,
    user: null,
    loading: false,
  })),
}));

// Mock Tabs so TabsTrigger clicks directly invoke onValueChange via a global registry
// Tabs stores its onValueChange callback keyed by a stable id so TabsTrigger can call it.
const tabsCallbacks: Record<string, (v: string) => void> = {};
let tabsIdCounter = 0;

vi.mock("@/components/ui/tabs", () => {
  return {
    Tabs: ({
      children,
      onValueChange,
    }: {
      children: unknown;
      onValueChange?: (v: string) => void;
      defaultValue?: string;
    }) => {
      const id = String(++tabsIdCounter);
      if (onValueChange) tabsCallbacks[id] = onValueChange;
      return (
        <div data-testid="tabs" data-tabs-id={id}>
          {children as React.ReactNode}
        </div>
      );
    },
    TabsList: ({ children }: { children: unknown }) => (
      <div data-testid="tabs-list">{children as React.ReactNode}</div>
    ),
    TabsTrigger: ({
      children,
      value,
    }: {
      children: unknown;
      value: string;
    }) => (
      <button
        data-testid={`tab-${value}`}
        onClick={() => {
          // Call all registered onValueChange callbacks
          Object.values(tabsCallbacks).forEach((cb) => cb(value));
        }}
      >
        {children as React.ReactNode}
      </button>
    ),
  };
});

// Mock recharts to avoid canvas issues in JSDOM
vi.mock("recharts", () => ({
  PieChart: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="pie-chart">{children}</div>
  ),
  Pie: ({ children }: { children?: React.ReactNode }) => (
    <div data-testid="pie">{children}</div>
  ),
  Cell: () => <div data-testid="cell" />,
  BarChart: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="bar-chart">{children}</div>
  ),
  Bar: () => <div data-testid="bar" />,
  XAxis: () => <div data-testid="xaxis" />,
  YAxis: () => <div data-testid="yaxis" />,
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="responsive-container">{children}</div>
  ),
  Tooltip: () => <div data-testid="tooltip" />,
  Legend: () => <div data-testid="legend" />,
}));

// Mock Next.js environment
vi.stubEnv("NEXT_PUBLIC_API_URL", "http://test-api.example.com");

const mockBreakdownData = {
  data: {
    total_searches: 100,
    zero_result_rate: 0.15,
    by_searcher_type: { agent: 50, human: 40, anonymous: 10 },
    window: "24h",
  },
};

const mockTrendingData = {
  data: {
    trending: [
      { query: "golang error handling", count: 15 },
      { query: "postgres connection pool", count: 8 },
      { query: "react hooks pattern", count: 5 },
    ],
    window: "24h",
  },
};

const mockCategoriesData = {
  data: {
    categories: [
      { category: "problem", search_count: 45 },
      { category: "idea", search_count: 30 },
      { category: "unfiltered", search_count: 25 },
    ],
    window: "24h",
  },
};

function makeFetchMock() {
  return vi.fn().mockImplementation((url: string) => {
    if (url.includes("/v1/data/trending")) {
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve(mockTrendingData),
      });
    }
    if (url.includes("/v1/data/breakdown")) {
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve(mockBreakdownData),
      });
    }
    if (url.includes("/v1/data/categories")) {
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve(mockCategoriesData),
      });
    }
    return Promise.resolve({ ok: false, json: () => Promise.resolve({}) });
  });
}

beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("DataPage", () => {
  it("renders stat cards (Total Searches, Agent, Human, Zero Results) from mock API data", async () => {
    global.fetch = makeFetchMock();

    await act(async () => {
      render(<DataPage />);
      // Allow the 100ms setTimeout inside fetchAll to fire
      await new Promise((r) => setTimeout(r, 150));
    });

    await waitFor(() => {
      expect(screen.getByText("100")).toBeInTheDocument();
    });

    // Stat card labels
    expect(screen.getByText("TOTAL SEARCHES")).toBeInTheDocument();
    expect(screen.getByText("AGENT")).toBeInTheDocument();
    expect(screen.getByText("HUMAN")).toBeInTheDocument();
    expect(screen.getByText("GUEST")).toBeInTheDocument();
  });

  it("renders trending queries table rows from mock trending data", async () => {
    global.fetch = makeFetchMock();

    await act(async () => {
      render(<DataPage />);
      await new Promise((r) => setTimeout(r, 150));
    });

    await waitFor(() => {
      expect(screen.getByText("golang error handling")).toBeInTheDocument();
    });

    expect(screen.getByText("postgres connection pool")).toBeInTheDocument();
    expect(screen.getByText("react hooks pattern")).toBeInTheDocument();
  });

  it("renders PieChart and BarChart components", async () => {
    global.fetch = makeFetchMock();

    await act(async () => {
      render(<DataPage />);
      await new Promise((r) => setTimeout(r, 150));
    });

    await waitFor(() => {
      expect(screen.getByTestId("pie-chart")).toBeInTheDocument();
    });

    expect(screen.getByTestId("bar-chart")).toBeInTheDocument();
  });

  it("calls all three /v1/data/* endpoints (trending, breakdown, categories) on mount", async () => {
    const fetchMock = makeFetchMock();
    global.fetch = fetchMock;

    await act(async () => {
      render(<DataPage />);
      await new Promise((r) => setTimeout(r, 50));
    });

    await waitFor(() => {
      const calls = fetchMock.mock.calls.map(([url]) => url as string);
      expect(calls.some((u) => u.includes("/v1/data/trending"))).toBe(true);
      expect(calls.some((u) => u.includes("/v1/data/breakdown"))).toBe(true);
      expect(calls.some((u) => u.includes("/v1/data/categories"))).toBe(true);
    });
  });

  it("time range toggle (1h/24h/7d) calls fetch with new window parameter", async () => {
    const fetchMock = makeFetchMock();
    global.fetch = fetchMock;

    await act(async () => {
      render(<DataPage />);
      await new Promise((r) => setTimeout(r, 150));
    });

    // Wait for initial load
    await waitFor(() => {
      expect(screen.getByText("100")).toBeInTheDocument();
    });

    const initialCallCount = fetchMock.mock.calls.length;

    // Click the 1h toggle
    await act(async () => {
      const oneHourButton = screen.getByText("1h");
      fireEvent.click(oneHourButton);
      await new Promise((r) => setTimeout(r, 150));
    });

    await waitFor(() => {
      expect(fetchMock.mock.calls.length).toBeGreaterThan(initialCallCount);
    });

    // Should have fetched with window=1h
    const urlsCalled = fetchMock.mock.calls.map(([url]) => url as string);
    expect(urlsCalled.some((u) => u.includes("window=1h"))).toBe(true);
  });

  it("shows loading skeleton when data is loading", () => {
    // Fetch never resolves during this test
    global.fetch = vi.fn().mockImplementation(() => new Promise(() => {}));
    render(<DataPage />);

    // Loading skeletons should be visible
    const skeletons = document.querySelectorAll('[data-slot="skeleton"]');
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it("shows empty state message when trending array is empty", async () => {
    global.fetch = vi.fn().mockImplementation((url: string) => {
      if (url.includes("/v1/data/trending")) {
        return Promise.resolve({
          ok: true,
          json: () =>
            Promise.resolve({ data: { trending: [], window: "1h" } }),
        });
      }
      if (url.includes("/v1/data/breakdown")) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve(mockBreakdownData),
        });
      }
      if (url.includes("/v1/data/categories")) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve(mockCategoriesData),
        });
      }
      return Promise.resolve({ ok: false, json: () => Promise.resolve({}) });
    });

    await act(async () => {
      render(<DataPage />);
      await new Promise((r) => setTimeout(r, 150));
    });

    await waitFor(() => {
      expect(screen.getByText(/no activity/i)).toBeInTheDocument();
    });
  });

  it("shows error state with retry button when fetch fails", async () => {
    global.fetch = vi.fn().mockRejectedValue(new Error("Network error"));

    await act(async () => {
      render(<DataPage />);
      await new Promise((r) => setTimeout(r, 50));
    });

    await waitFor(() => {
      expect(screen.getByText(/could not load/i)).toBeInTheDocument();
    });

    // Retry button should be present
    expect(
      screen.getByRole("button", { name: /try again/i })
    ).toBeInTheDocument();
  });
});
