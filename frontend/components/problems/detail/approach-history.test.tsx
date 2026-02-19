import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { ApproachHistory } from './approach-history';
import type { APIApproachVersionHistory } from '@/lib/api-types';

const createMockHistory = (overrides?: Partial<APIApproachVersionHistory>): APIApproachVersionHistory => ({
  current: {
    id: 'approach-v3',
    problem_id: 'problem-1',
    author_type: 'agent',
    author_id: 'agent-1',
    angle: 'Use connection pooling v3',
    method: 'PgBouncer with transaction mode',
    assumptions: [],
    status: 'working',
    outcome: null,
    solution: null,
    created_at: '2025-01-20T10:00:00Z',
    updated_at: '2025-01-20T12:00:00Z',
    author: { type: 'agent', id: 'agent-1', display_name: 'Claude' },
  },
  history: [
    {
      id: 'approach-v1',
      problem_id: 'problem-1',
      author_type: 'agent',
      author_id: 'agent-1',
      angle: 'Use connection pooling v1',
      method: 'Basic pool config',
      assumptions: [],
      status: 'failed',
      outcome: 'Pool size too small',
      solution: null,
      created_at: '2025-01-10T10:00:00Z',
      updated_at: '2025-01-12T10:00:00Z',
      author: { type: 'agent', id: 'agent-1', display_name: 'Claude' },
    },
    {
      id: 'approach-v2',
      problem_id: 'problem-1',
      author_type: 'agent',
      author_id: 'agent-1',
      angle: 'Use connection pooling v2',
      method: 'PgBouncer session mode',
      assumptions: [],
      status: 'failed',
      outcome: 'Session mode caused issues',
      solution: null,
      created_at: '2025-01-15T10:00:00Z',
      updated_at: '2025-01-17T10:00:00Z',
      author: { type: 'agent', id: 'agent-1', display_name: 'Claude' },
    },
  ],
  relationships: [
    {
      id: 'rel-1',
      from_approach_id: 'approach-v3',
      to_approach_id: 'approach-v2',
      relation_type: 'updates',
      created_at: '2025-01-20T10:00:00Z',
    },
    {
      id: 'rel-2',
      from_approach_id: 'approach-v2',
      to_approach_id: 'approach-v1',
      relation_type: 'updates',
      created_at: '2025-01-15T10:00:00Z',
    },
  ],
  ...overrides,
});

describe('ApproachHistory', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders version timeline with correct order', () => {
    const history = createMockHistory();
    render(<ApproachHistory history={history} />);

    expect(screen.getByText('VERSION HISTORY')).toBeInTheDocument();
    // All three versions should be visible
    expect(screen.getByText('Use connection pooling v1')).toBeInTheDocument();
    expect(screen.getByText('Use connection pooling v2')).toBeInTheDocument();
    expect(screen.getByText('Use connection pooling v3')).toBeInTheDocument();
  });

  it('shows relationship badges with correct labels', () => {
    const history = createMockHistory();
    render(<ApproachHistory history={history} />);

    // "updates" relationships should show UPDATES badges
    const badges = screen.getAllByText('UPDATES');
    expect(badges.length).toBe(2);
  });

  it('highlights current version', () => {
    const history = createMockHistory();
    render(<ApproachHistory history={history} />);

    // Current version should have CURRENT label
    expect(screen.getByText('CURRENT')).toBeInTheDocument();
  });

  it('shows empty state when no history', () => {
    const history = createMockHistory({
      history: [],
      relationships: [],
    });
    render(<ApproachHistory history={history} />);

    expect(screen.getByText('No version history')).toBeInTheDocument();
  });

  it('renders different relationship types', () => {
    const history = createMockHistory({
      relationships: [
        {
          id: 'rel-1',
          from_approach_id: 'approach-v3',
          to_approach_id: 'approach-v2',
          relation_type: 'extends',
          created_at: '2025-01-20T10:00:00Z',
        },
        {
          id: 'rel-2',
          from_approach_id: 'approach-v2',
          to_approach_id: 'approach-v1',
          relation_type: 'derives',
          created_at: '2025-01-15T10:00:00Z',
        },
      ],
    });
    render(<ApproachHistory history={history} />);

    expect(screen.getByText('EXTENDS')).toBeInTheDocument();
    expect(screen.getByText('DERIVES')).toBeInTheDocument();
  });
});
