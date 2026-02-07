import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { ApproachesList } from './approaches-list';
import type { ProblemApproach } from '@/hooks/use-problem';

// Mock Next.js router
const mockPush = vi.fn();
vi.mock('next/navigation', () => ({
  useRouter: () => ({
    push: mockPush,
  }),
}));

// Mock useAuth hook
let mockIsAuthenticated = false;
vi.mock('@/hooks/use-auth', () => ({
  useAuth: () => ({
    isAuthenticated: mockIsAuthenticated,
    isLoading: false,
    user: mockIsAuthenticated ? { id: 'user-1', displayName: 'Test User' } : null,
  }),
}));

// Mock the useApproachForm hook
vi.mock('@/hooks/use-approach-form', () => ({
  useApproachForm: () => ({
    angle: '',
    setAngle: vi.fn(),
    method: '',
    setMethod: vi.fn(),
    assumptions: [],
    setAssumptions: vi.fn(),
    error: null,
    isSubmitting: false,
    submit: vi.fn(),
  }),
}));

// Mock useProgressNoteForm hook
const mockProgressNoteSubmit = vi.fn();
const mockProgressNoteReset = vi.fn();
let mockProgressNoteContent = '';
let mockProgressNoteError: string | null = null;
let mockProgressNoteIsSubmitting = false;

vi.mock('@/hooks/use-progress-note-form', () => ({
  useProgressNoteForm: () => ({
    content: mockProgressNoteContent,
    setContent: (val: string) => { mockProgressNoteContent = val; },
    isSubmitting: mockProgressNoteIsSubmitting,
    error: mockProgressNoteError,
    submit: mockProgressNoteSubmit,
    reset: mockProgressNoteReset,
  }),
}));

const createMockApproach = (overrides: Partial<ProblemApproach> = {}): ProblemApproach => ({
  id: 'approach-1',
  angle: 'Test approach angle',
  method: 'Test method',
  assumptions: [],
  status: 'working',
  outcome: null,
  solution: null,
  progressNotes: [],
  author: {
    id: 'user-1',
    type: 'human',
    displayName: 'Test User',
  },
  createdAt: '2025-01-10T10:00:00Z',
  updatedAt: '2025-01-15T14:30:00Z',
  time: '5d ago',
  ...overrides,
});

describe('ApproachesList', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Reset auth state
    mockIsAuthenticated = false;
    // Reset progress note form state
    mockProgressNoteContent = '';
    mockProgressNoteError = null;
    mockProgressNoteIsSubmitting = false;
    // Mock Date for consistent duration tests
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2025-01-20T12:00:00Z'));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('renders empty state when no approaches', () => {
    render(<ApproachesList approaches={[]} problemId="problem-1" />);

    expect(screen.getByText('No approaches yet. Be the first to propose a solution!')).toBeInTheDocument();
  });

  it('renders active approaches with ADD PROGRESS NOTE button', () => {
    const activeApproach = createMockApproach({ status: 'working' });
    render(<ApproachesList approaches={[activeApproach]} problemId="problem-1" />);

    // Approach should be expanded by default (first one)
    expect(screen.getByText('ADD PROGRESS NOTE')).toBeInTheDocument();
  });

  it('renders non-active approaches with duration and updated time', () => {
    const failedApproach = createMockApproach({
      status: 'failed',
      createdAt: '2025-01-10T10:00:00Z',
      updatedAt: '2025-01-15T14:30:00Z',
    });
    render(<ApproachesList approaches={[failedApproach]} problemId="problem-1" />);

    // Should show duration (5 days, 4 hours)
    expect(screen.getByText('TOOK 5D 4H')).toBeInTheDocument();
    // Should show updated time (4-5 days ago from Jan 20 12:00 - Jan 15 14:30 is ~4.9 days = 4D)
    expect(screen.getByText('UPDATED 4D AGO')).toBeInTheDocument();
  });

  it('renders succeeded approaches with duration', () => {
    const succeededApproach = createMockApproach({
      status: 'succeeded',
      createdAt: '2025-01-18T10:00:00Z',
      updatedAt: '2025-01-18T15:00:00Z',
    });
    render(<ApproachesList approaches={[succeededApproach]} problemId="problem-1" />);

    // Should show duration (5 hours)
    expect(screen.getByText('TOOK 5H')).toBeInTheDocument();
  });

  it('renders abandoned approaches with duration', () => {
    const abandonedApproach = createMockApproach({
      status: 'abandoned',
      createdAt: '2025-01-10T10:00:00Z',
      updatedAt: '2025-01-10T10:30:00Z',
    });
    render(<ApproachesList approaches={[abandonedApproach]} problemId="problem-1" />);

    // Should show "Less than 1h"
    expect(screen.getByText('TOOK LESS THAN 1H')).toBeInTheDocument();
  });

  it('separates active and non-active approaches', () => {
    const activeApproach = createMockApproach({ id: 'active-1', status: 'working' });
    const failedApproach = createMockApproach({ id: 'failed-1', status: 'failed' });

    render(<ApproachesList approaches={[activeApproach, failedApproach]} problemId="problem-1" />);

    expect(screen.getByText('ACTIVE')).toBeInTheDocument();
    expect(screen.getByText('COMPLETED / ABANDONED')).toBeInTheDocument();
  });

  it('shows START APPROACH button', () => {
    render(<ApproachesList approaches={[]} problemId="problem-1" />);

    // Two buttons: one in empty state, one in header
    const buttons = screen.getAllByText('START APPROACH');
    expect(buttons.length).toBeGreaterThanOrEqual(1);
  });

  it('displays approach count correctly', () => {
    const approaches = [
      createMockApproach({ id: '1', status: 'working' }),
      createMockApproach({ id: '2', status: 'failed' }),
      createMockApproach({ id: '3', status: 'starting' }),
    ];

    render(<ApproachesList approaches={approaches} problemId="problem-1" />);

    expect(screen.getByText('3 TOTAL — 2 ACTIVE')).toBeInTheDocument();
  });

  it('toggles approach expansion on click', () => {
    const approach = createMockApproach({ method: 'Detailed method description' });
    render(<ApproachesList approaches={[approach]} problemId="problem-1" />);

    // First approach is expanded by default
    expect(screen.getByText('Detailed method description')).toBeInTheDocument();

    // Click to collapse
    const header = screen.getByRole('button', { name: /Test approach angle/i });
    fireEvent.click(header);

    // Method should be hidden now
    expect(screen.queryByText('Detailed method description')).not.toBeInTheDocument();
  });

  it('shows FAILED status badge', () => {
    const failedApproach = createMockApproach({ status: 'failed' });
    render(<ApproachesList approaches={[failedApproach]} problemId="problem-1" />);

    expect(screen.getByText('FAILED')).toBeInTheDocument();
  });

  it('shows SUCCEEDED status badge', () => {
    const succeededApproach = createMockApproach({ status: 'succeeded' });
    render(<ApproachesList approaches={[succeededApproach]} problemId="problem-1" />);

    expect(screen.getByText('SUCCEEDED')).toBeInTheDocument();
  });

  it('shows WORKING status badge for active approaches', () => {
    const workingApproach = createMockApproach({ status: 'working' });
    render(<ApproachesList approaches={[workingApproach]} problemId="problem-1" />);

    expect(screen.getByText('WORKING')).toBeInTheDocument();
  });

  it('renders outcome section when outcome exists', () => {
    const approachWithOutcome = createMockApproach({
      status: 'failed',
      outcome: 'The approach failed because of X',
    });
    render(<ApproachesList approaches={[approachWithOutcome]} problemId="problem-1" />);

    expect(screen.getByText('OUTCOME')).toBeInTheDocument();
    expect(screen.getByText('The approach failed because of X')).toBeInTheDocument();
  });

  it('does not render outcome section when outcome is null', () => {
    const approachWithoutOutcome = createMockApproach({ outcome: null });
    render(<ApproachesList approaches={[approachWithoutOutcome]} problemId="problem-1" />);

    expect(screen.queryByText('OUTCOME')).not.toBeInTheDocument();
  });

  it('renders assumptions when they exist', () => {
    const approachWithAssumptions = createMockApproach({
      assumptions: ['First assumption', 'Second assumption'],
    });
    render(<ApproachesList approaches={[approachWithAssumptions]} problemId="problem-1" />);

    expect(screen.getByText('ASSUMPTIONS')).toBeInTheDocument();
    expect(screen.getByText('First assumption')).toBeInTheDocument();
    expect(screen.getByText('Second assumption')).toBeInTheDocument();
  });

  it('does not render assumptions section when empty', () => {
    const approachWithoutAssumptions = createMockApproach({ assumptions: [] });
    render(<ApproachesList approaches={[approachWithoutAssumptions]} problemId="problem-1" />);

    expect(screen.queryByText('ASSUMPTIONS')).not.toBeInTheDocument();
  });
});

// Test the formatDuration helper directly
describe('formatDuration helper (via component)', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2025-01-20T12:00:00Z'));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('formats multi-day duration correctly', () => {
    const approach = createMockApproach({
      status: 'failed',
      createdAt: '2025-01-10T10:00:00Z',
      updatedAt: '2025-01-15T20:00:00Z',
    });
    render(<ApproachesList approaches={[approach]} problemId="problem-1" />);

    // 5 days 10 hours
    expect(screen.getByText('TOOK 5D 10H')).toBeInTheDocument();
  });

  it('formats hours-only duration correctly', () => {
    const approach = createMockApproach({
      status: 'failed',
      createdAt: '2025-01-10T10:00:00Z',
      updatedAt: '2025-01-10T18:00:00Z',
    });
    render(<ApproachesList approaches={[approach]} problemId="problem-1" />);

    // 8 hours
    expect(screen.getByText('TOOK 8H')).toBeInTheDocument();
  });

  it('formats sub-hour duration as less than 1h', () => {
    const approach = createMockApproach({
      status: 'failed',
      createdAt: '2025-01-10T10:00:00Z',
      updatedAt: '2025-01-10T10:45:00Z',
    });
    render(<ApproachesList approaches={[approach]} problemId="problem-1" />);

    expect(screen.getByText('TOOK LESS THAN 1H')).toBeInTheDocument();
  });
});

// Test ADD PROGRESS NOTE button functionality
describe('ADD PROGRESS NOTE button', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockIsAuthenticated = false;
    mockProgressNoteContent = '';
    mockProgressNoteError = null;
    mockProgressNoteIsSubmitting = false;
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2025-01-20T12:00:00Z'));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('has cursor-pointer class on ADD PROGRESS NOTE button', () => {
    const activeApproach = createMockApproach({ status: 'working' });
    render(<ApproachesList approaches={[activeApproach]} problemId="problem-1" />);

    const button = screen.getByText('ADD PROGRESS NOTE');
    expect(button).toHaveClass('cursor-pointer');
  });

  it('redirects to /login when clicking ADD PROGRESS NOTE while not authenticated', () => {
    mockIsAuthenticated = false;
    const activeApproach = createMockApproach({ status: 'working' });
    render(<ApproachesList approaches={[activeApproach]} problemId="problem-1" />);

    const button = screen.getByText('ADD PROGRESS NOTE');
    fireEvent.click(button);

    expect(mockPush).toHaveBeenCalledWith('/login');
  });

  it('shows progress note form when clicking ADD PROGRESS NOTE while authenticated', () => {
    mockIsAuthenticated = true;
    const activeApproach = createMockApproach({ status: 'working' });
    render(<ApproachesList approaches={[activeApproach]} problemId="problem-1" />);

    const button = screen.getByText('ADD PROGRESS NOTE');
    fireEvent.click(button);

    // Should show the form with textarea and buttons
    expect(screen.getByPlaceholderText('Add a progress update...')).toBeInTheDocument();
    expect(screen.getByText('CANCEL')).toBeInTheDocument();
    expect(screen.getByText('POST NOTE')).toBeInTheDocument();
  });

  it('hides form when clicking CANCEL', () => {
    mockIsAuthenticated = true;
    const activeApproach = createMockApproach({ status: 'working' });
    render(<ApproachesList approaches={[activeApproach]} problemId="problem-1" />);

    // Open form
    const addButton = screen.getByText('ADD PROGRESS NOTE');
    fireEvent.click(addButton);

    // Verify form is shown
    expect(screen.getByPlaceholderText('Add a progress update...')).toBeInTheDocument();

    // Click cancel
    const cancelButton = screen.getByText('CANCEL');
    fireEvent.click(cancelButton);

    // Form should be hidden
    expect(screen.queryByPlaceholderText('Add a progress update...')).not.toBeInTheDocument();
    expect(mockProgressNoteReset).toHaveBeenCalled();
  });

  it('calls submit when clicking POST NOTE', () => {
    mockIsAuthenticated = true;
    const activeApproach = createMockApproach({ status: 'working' });
    render(<ApproachesList approaches={[activeApproach]} problemId="problem-1" />);

    // Open form
    const addButton = screen.getByText('ADD PROGRESS NOTE');
    fireEvent.click(addButton);

    // Click POST NOTE
    const postButton = screen.getByText('POST NOTE');
    fireEvent.click(postButton);

    expect(mockProgressNoteSubmit).toHaveBeenCalled();
  });

  it('does not redirect to login when authenticated', () => {
    mockIsAuthenticated = true;
    const activeApproach = createMockApproach({ status: 'working' });
    render(<ApproachesList approaches={[activeApproach]} problemId="problem-1" />);

    const button = screen.getByText('ADD PROGRESS NOTE');
    fireEvent.click(button);

    expect(mockPush).not.toHaveBeenCalled();
  });
});
