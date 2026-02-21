import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import PinsPage from './page';

// Mock next/navigation — default empty params, tests can override
const mockSearchParams = vi.fn(() => new URLSearchParams());
vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: vi.fn() }),
  useSearchParams: () => mockSearchParams(),
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
      expect(mockCreate).toHaveBeenCalledWith({
        cid: 'QmValidCIDTest123456789abcdefghijklmnopqrs12',
        name: 'my-content',
      });
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
    // "ALL" appears in both status and meta filter tabs
    const allButtons = screen.getAllByText('ALL');
    expect(allButtons.length).toBeGreaterThanOrEqual(1);
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

  it('passes agentId to usePins when ?agent= param is set', () => {
    mockSearchParams.mockReturnValue(new URLSearchParams('agent=ClaudiusThePirateEmperor'));
    mockUsePins.mockReturnValue(defaultPinsResult);
    render(<PinsPage />);

    // usePins should be called with agentId
    expect(mockUsePins).toHaveBeenCalledWith(
      expect.objectContaining({ agentId: 'ClaudiusThePirateEmperor' })
    );
  });

  it('shows agent name in header when viewing agent pins', () => {
    mockSearchParams.mockReturnValue(new URLSearchParams('agent=ClaudiusThePirateEmperor'));
    mockUsePins.mockReturnValue({
      ...defaultPinsResult,
      pins: [pinFixtures[0]],
      totalCount: 1,
    });
    render(<PinsPage />);

    expect(screen.getByText(/ClaudiusThePirateEmperor/)).toBeInTheDocument();
  });

  it('renders CID as clickable IPFS gateway link', () => {
    mockUsePins.mockReturnValue({
      ...defaultPinsResult,
      pins: [pinFixtures[0]],
      totalCount: 1,
    });
    render(<PinsPage />);

    const cidLink = screen.getByRole('link', { name: /QmTest1.*12345/i });
    expect(cidLink).toHaveAttribute('href', expect.stringContaining('ipfs.io/ipfs/QmTest123'));
  });

  // ====== Meta features tests ======

  it('renders METADATA collapsible section in create pin dialog', () => {
    render(<PinsPage />);
    fireEvent.click(screen.getByText('PIN NEW CONTENT'));

    const metadataToggle = screen.getByText('METADATA');
    expect(metadataToggle).toBeInTheDocument();

    // Initially collapsed — no key/value inputs visible
    expect(screen.queryByPlaceholderText(/key/i)).not.toBeInTheDocument();

    // Click to expand
    fireEvent.click(metadataToggle);

    // Now shows add row button
    expect(screen.getByText('ADD FIELD')).toBeInTheDocument();
  });

  it('submits pin with meta data via CreatePinParams', async () => {
    const mockCreate = vi.fn().mockResolvedValue(undefined);
    mockUsePins.mockReturnValue({
      ...defaultPinsResult,
      createPin: mockCreate,
    });

    render(<PinsPage />);
    fireEvent.click(screen.getByText('PIN NEW CONTENT'));

    // Fill CID
    const cidInput = screen.getByPlaceholderText(/Qm\.\.\. or bafy\.\.\./i);
    fireEvent.change(cidInput, { target: { value: 'QmValidCIDTest123456789abcdefghijklmnopqrs12' } });

    // Fill Name
    const nameInput = screen.getByPlaceholderText(/optional name/i);
    fireEvent.change(nameInput, { target: { value: 'meta-test' } });

    // Expand metadata and add a key-value pair
    fireEvent.click(screen.getByText('METADATA'));
    fireEvent.click(screen.getByText('ADD FIELD'));

    const keyInputs = screen.getAllByPlaceholderText(/key/i);
    const valueInputs = screen.getAllByPlaceholderText(/value/i);
    fireEvent.change(keyInputs[0], { target: { value: 'type' } });
    fireEvent.change(valueInputs[0], { target: { value: 'amcp_checkpoint' } });

    // Submit
    const dialogButtons = screen.getAllByRole('button');
    const submitButton = dialogButtons.find(b => b.textContent === 'PIN');
    fireEvent.click(submitButton!);

    await waitFor(() => {
      expect(mockCreate).toHaveBeenCalledWith({
        cid: 'QmValidCIDTest123456789abcdefghijklmnopqrs12',
        name: 'meta-test',
        meta: { type: 'amcp_checkpoint' },
      });
    });
  });

  it('submits pin without name (backend auto-generates)', async () => {
    const mockCreate = vi.fn().mockResolvedValue(undefined);
    mockUsePins.mockReturnValue({
      ...defaultPinsResult,
      createPin: mockCreate,
    });

    render(<PinsPage />);
    fireEvent.click(screen.getByText('PIN NEW CONTENT'));

    const cidInput = screen.getByPlaceholderText(/Qm\.\.\. or bafy\.\.\./i);
    fireEvent.change(cidInput, { target: { value: 'QmValidCIDTest123456789abcdefghijklmnopqrs12' } });

    // Leave name empty — backend should auto-generate
    const dialogButtons = screen.getAllByRole('button');
    const submitButton = dialogButtons.find(b => b.textContent === 'PIN');
    fireEvent.click(submitButton!);

    await waitFor(() => {
      expect(mockCreate).toHaveBeenCalledWith({
        cid: 'QmValidCIDTest123456789abcdefghijklmnopqrs12',
      });
    });
  });

  it('renders meta tags as badges in PinRow', () => {
    const pinWithMeta = {
      requestid: 'pin-meta-1',
      status: 'pinned' as const,
      created: '2026-02-18T10:00:00Z',
      pin: {
        cid: 'QmTest123456789abcdefghijklmnopqrstuvwxyz12345',
        name: 'checkpoint-pin',
        meta: { type: 'amcp_checkpoint', agent_id: 'claudius' },
      },
      delegates: [],
    };

    mockUsePins.mockReturnValue({
      ...defaultPinsResult,
      pins: [pinWithMeta],
      totalCount: 1,
    });
    render(<PinsPage />);

    // Meta key:value should render as badges
    expect(screen.getByText('type: amcp_checkpoint')).toBeInTheDocument();
    expect(screen.getByText('agent_id: claudius')).toBeInTheDocument();
  });

  it('renders system meta keys with emerald badge style', () => {
    const pinWithSystemMeta = {
      requestid: 'pin-sys-1',
      status: 'pinned' as const,
      created: '2026-02-18T10:00:00Z',
      pin: {
        cid: 'QmTest123456789abcdefghijklmnopqrstuvwxyz12345',
        name: 'sys-pin',
        meta: { type: 'amcp_checkpoint' },
      },
      delegates: [],
    };

    mockUsePins.mockReturnValue({
      ...defaultPinsResult,
      pins: [pinWithSystemMeta],
      totalCount: 1,
    });
    render(<PinsPage />);

    const badge = screen.getByText('type: amcp_checkpoint');
    expect(badge.className).toContain('bg-emerald-500/20');
  });

  it('renders user-defined meta keys with secondary badge style', () => {
    const pinWithUserMeta = {
      requestid: 'pin-user-1',
      status: 'pinned' as const,
      created: '2026-02-18T10:00:00Z',
      pin: {
        cid: 'QmTest123456789abcdefghijklmnopqrstuvwxyz12345',
        name: 'user-pin',
        meta: { project: 'my-app' },
      },
      delegates: [],
    };

    mockUsePins.mockReturnValue({
      ...defaultPinsResult,
      pins: [pinWithUserMeta],
      totalCount: 1,
    });
    render(<PinsPage />);

    const badge = screen.getByText('project: my-app');
    expect(badge.className).toContain('bg-secondary');
  });

  it('renders meta type filter pills (ALL | CHECKPOINTS)', () => {
    render(<PinsPage />);

    // Look for meta type filter pills separate from status tabs
    expect(screen.getByTestId('meta-filter-all')).toBeInTheDocument();
    expect(screen.getByTestId('meta-filter-checkpoints')).toBeInTheDocument();
  });

  it('clicking CHECKPOINTS filter passes meta param to usePins', () => {
    render(<PinsPage />);

    const checkpointsFilter = screen.getByTestId('meta-filter-checkpoints');
    fireEvent.click(checkpointsFilter);

    // usePins should be called with meta filter
    expect(mockUsePins).toHaveBeenCalledWith(
      expect.objectContaining({ meta: { type: 'amcp_checkpoint' } })
    );
  });
});
