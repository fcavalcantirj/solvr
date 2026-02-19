import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import PinsPage from './page';

// Mock next/navigation
vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: vi.fn() }),
  useSearchParams: () => new URLSearchParams(),
}));

// Mock next/link
vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

// Mock the header component
vi.mock('@/components/header', () => ({
  Header: () => <header data-testid="header">Header</header>,
}));

// Mock the auth hook
const mockUseAuth = vi.fn();
vi.mock('@/hooks/use-auth', () => ({
  useAuth: () => mockUseAuth(),
}));

// Mock the pins hook
const mockUsePins = vi.fn();
vi.mock('@/hooks/use-pins', () => ({
  usePins: (...args: unknown[]) => mockUsePins(...args),
}));

const defaultPinsResult = {
  pins: [],
  loading: false,
  error: null,
  totalCount: 0,
  storage: { used: 0, quota: 1073741824, percentage: 0 },
  createPin: vi.fn(),
  deletePin: vi.fn(),
  refetch: vi.fn(),
};

const pinFixtures = [
  {
    requestid: 'pin-1',
    status: 'pinned' as const,
    created: '2026-02-18T10:00:00Z',
    pin: { cid: 'QmTest123456789abcdefghijklmnopqrstuvwxyz12345', name: 'my-file' },
    delegates: [],
    info: { size_bytes: 2048 },
  },
  {
    requestid: 'pin-2',
    status: 'queued' as const,
    created: '2026-02-18T11:00:00Z',
    pin: { cid: 'bafyTestCID123456' },
    delegates: [],
  },
  {
    requestid: 'pin-3',
    status: 'failed' as const,
    created: '2026-02-18T12:00:00Z',
    pin: { cid: 'QmFailedPin9876543210' },
    delegates: [],
  },
];

describe('PinsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseAuth.mockReturnValue({
      user: { id: 'user-1', displayName: 'Test User' },
      isAuthenticated: true,
    });
    mockUsePins.mockReturnValue(defaultPinsResult);
  });

  it('renders the page header with title', () => {
    render(<PinsPage />);
    expect(screen.getByText('MY PINS')).toBeInTheDocument();
  });

  it('shows loading skeleton when loading', () => {
    mockUsePins.mockReturnValue({ ...defaultPinsResult, loading: true });
    render(<PinsPage />);
    const skeletons = document.querySelectorAll('.animate-pulse');
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it('shows empty state when no pins exist', () => {
    render(<PinsPage />);
    expect(screen.getByText(/no pins yet/i)).toBeInTheDocument();
  });

  it('renders pin list with correct data', () => {
    mockUsePins.mockReturnValue({
      ...defaultPinsResult,
      pins: pinFixtures,
      totalCount: 3,
    });
    render(<PinsPage />);

    expect(screen.getByText('my-file')).toBeInTheDocument();
    // Use getAllByText since status labels appear in both filter tabs and badges
    const pinnedElements = screen.getAllByText('PINNED');
    expect(pinnedElements.length).toBeGreaterThanOrEqual(2); // tab + badge
    const queuedElements = screen.getAllByText('QUEUED');
    expect(queuedElements.length).toBeGreaterThanOrEqual(2); // tab + badge
    const failedElements = screen.getAllByText('FAILED');
    expect(failedElements.length).toBeGreaterThanOrEqual(2); // tab + badge
  });

  it('shows error state with message', () => {
    mockUsePins.mockReturnValue({
      ...defaultPinsResult,
      error: 'Network error occurred',
    });
    render(<PinsPage />);
    expect(screen.getByText(/network error occurred/i)).toBeInTheDocument();
  });

  it('shows storage usage bar', () => {
    mockUsePins.mockReturnValue({
      ...defaultPinsResult,
      storage: { used: 524288000, quota: 1073741824, percentage: 48.8 },
    });
    render(<PinsPage />);
    expect(screen.getByText(/500\.00 MB/)).toBeInTheDocument();
    expect(screen.getByText(/1\.00 GB/)).toBeInTheDocument();
  });

  it('opens create pin dialog when button clicked', () => {
    render(<PinsPage />);
    const createButton = screen.getByText('PIN NEW CONTENT');
    fireEvent.click(createButton);
    expect(screen.getByText('Pin Content to IPFS')).toBeInTheDocument();
  });

  it('validates CID format in create dialog', async () => {
    render(<PinsPage />);
    fireEvent.click(screen.getByText('PIN NEW CONTENT'));

    const cidInput = screen.getByPlaceholderText(/Qm\.\.\. or bafy\.\.\./i);
    fireEvent.change(cidInput, { target: { value: 'invalid-cid' } });

    const submitButton = screen.getByText('PIN');
    fireEvent.click(submitButton);

    expect(screen.getByText(/must start with Qm or bafy/i)).toBeInTheDocument();
  });

  it('submits valid CID through create dialog', async () => {
    const mockCreate = vi.fn().mockImplementation(() => Promise.resolve());
    mockUsePins.mockReturnValue({
      ...defaultPinsResult,
      createPin: mockCreate,
    });

    render(<PinsPage />);
    fireEvent.click(screen.getByText('PIN NEW CONTENT'));

    const cidInput = screen.getByPlaceholderText(/Qm\.\.\. or bafy\.\.\./i);
    fireEvent.change(cidInput, { target: { value: 'QmValidCIDTest123456789abcdefghijklmnopqrs12' } });

    const nameInput = screen.getByPlaceholderText(/optional name/i);
    fireEvent.change(nameInput, { target: { value: 'my-content' } });

    // Find the PIN submit button (not the filter tab) - it's inside the dialog
    const dialogButtons = screen.getAllByRole('button');
    const submitButton = dialogButtons.find(b => b.textContent === 'PIN');
    expect(submitButton).toBeTruthy();
    fireEvent.click(submitButton!);

    await waitFor(() => {
      expect(mockCreate).toHaveBeenCalledWith('QmValidCIDTest123456789abcdefghijklmnopqrs12', 'my-content');
    });
  });

  it('shows delete confirmation dialog', () => {
    mockUsePins.mockReturnValue({
      ...defaultPinsResult,
      pins: [pinFixtures[0]],
      totalCount: 1,
    });
    render(<PinsPage />);

    const deleteButtons = screen.getAllByLabelText(/delete pin/i);
    fireEvent.click(deleteButtons[0]);

    expect(screen.getByText(/this will unpin the content/i)).toBeInTheDocument();
  });

  it('calls deletePin on confirmation', async () => {
    const mockDelete = vi.fn().mockResolvedValue(undefined);
    mockUsePins.mockReturnValue({
      ...defaultPinsResult,
      pins: [pinFixtures[0]],
      totalCount: 1,
      deletePin: mockDelete,
    });
    render(<PinsPage />);

    const deleteButtons = screen.getAllByLabelText(/delete pin/i);
    fireEvent.click(deleteButtons[0]);

    const confirmButton = screen.getByText('UNPIN');
    fireEvent.click(confirmButton);

    await waitFor(() => {
      expect(mockDelete).toHaveBeenCalledWith('pin-1');
    });
  });

  it('shows unnamed label for pins without name', () => {
    mockUsePins.mockReturnValue({
      ...defaultPinsResult,
      pins: [pinFixtures[1]], // pin without name
      totalCount: 1,
    });
    render(<PinsPage />);
    expect(screen.getByText('Unnamed')).toBeInTheDocument();
  });

  it('displays truncated CID with full CID in title', () => {
    mockUsePins.mockReturnValue({
      ...defaultPinsResult,
      pins: [pinFixtures[0]],
      totalCount: 1,
    });
    render(<PinsPage />);

    const cidElement = screen.getByTitle('QmTest123456789abcdefghijklmnopqrstuvwxyz12345');
    expect(cidElement).toBeInTheDocument();
    // CID should be truncated
    expect(cidElement.textContent).not.toBe('QmTest123456789abcdefghijklmnopqrstuvwxyz12345');
  });

  it('shows status filter tabs', () => {
    render(<PinsPage />);
    expect(screen.getByText('ALL')).toBeInTheDocument();
    expect(screen.getByText(/PINNED/)).toBeInTheDocument();
    expect(screen.getByText(/QUEUED/)).toBeInTheDocument();
  });

  it('redirects to login when not authenticated', () => {
    mockUseAuth.mockReturnValue({
      user: null,
      isAuthenticated: false,
    });
    render(<PinsPage />);
    expect(screen.getByText(/sign in to manage/i)).toBeInTheDocument();
  });
});
