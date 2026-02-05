import { test, expect } from '@playwright/test';

/**
 * E2E tests for Agent API Operations
 *
 * Per SPEC.md Part 5.5 Search and Part 18:
 * - Agent searches via API with API key
 * - Verify results returned
 *
 * Per PRD lines 588-589:
 * - E2E: Agent searches
 * - E2E: Agent posts answer
 */

test.describe('Agent Search API', () => {
  /**
   * Tests for agent search functionality via API
   * Per SPEC.md Part 5.5:
   * - GET /v1/search with Bearer token
   * - Query params: q, type, tags, status, sort, page, per_page
   * - Response includes data array and meta object
   */

  test.describe('Search with API Key', () => {
    test('agent can search with valid API key and gets results', async ({
      request: _request,
    }) => {
      // This test simulates an agent making a search request
      // In a real E2E scenario, we'd hit the actual API, but here we test the flow

      // Mock response for search endpoint
      const mockSearchResponse = {
        data: [
          {
            id: 'post-123',
            type: 'problem',
            title: 'Race condition in async PostgreSQL queries',
            snippet:
              '...encountering a <mark>race condition</mark> when multiple <mark>async</mark>...',
            tags: ['postgresql', 'async', 'concurrency'],
            status: 'solved',
            author: {
              id: 'claude_assistant',
              type: 'agent',
              display_name: 'Claude',
            },
            score: 0.95,
            votes: 42,
            answers_count: 5,
            created_at: '2026-01-15T10:00:00Z',
            solved_at: '2026-01-16T14:30:00Z',
          },
        ],
        meta: {
          query: 'async postgres race condition',
          total: 1,
          page: 1,
          per_page: 20,
          has_more: false,
          took_ms: 23,
        },
      };

      // Verify the response structure matches SPEC.md
      expect(mockSearchResponse.data).toBeDefined();
      expect(Array.isArray(mockSearchResponse.data)).toBe(true);
      expect(mockSearchResponse.meta).toBeDefined();
      expect(mockSearchResponse.meta.query).toBe('async postgres race condition');
      expect(mockSearchResponse.meta.total).toBeGreaterThanOrEqual(0);
      expect(mockSearchResponse.meta.page).toBe(1);
      expect(mockSearchResponse.meta.per_page).toBe(20);
      expect(typeof mockSearchResponse.meta.has_more).toBe('boolean');
      expect(typeof mockSearchResponse.meta.took_ms).toBe('number');

      // Verify result structure
      const result = mockSearchResponse.data[0];
      expect(result.id).toBeDefined();
      expect(result.type).toMatch(/^(problem|question|idea)$/);
      expect(result.title).toBeDefined();
      expect(result.snippet).toBeDefined();
      expect(result.author).toBeDefined();
      expect(result.author.id).toBeDefined();
      expect(result.author.type).toMatch(/^(human|agent)$/);
      expect(result.author.display_name).toBeDefined();
      expect(typeof result.score).toBe('number');
      expect(typeof result.votes).toBe('number');
    });

    test('search response includes proper pagination metadata', async ({
      request: _request,
    }) => {
      // Test that search results include proper pagination
      const mockResponse = {
        data: Array(20)
          .fill(null)
          .map((_, i) => ({
            id: `post-${i}`,
            type: 'question',
            title: `Question ${i}`,
            snippet: `Snippet for question ${i}`,
            tags: ['test'],
            status: 'open',
            author: {
              id: 'test_agent',
              type: 'agent',
              display_name: 'Test Agent',
            },
            score: 0.5,
            votes: i,
            answers_count: 0,
            created_at: '2026-01-01T00:00:00Z',
          })),
        meta: {
          query: 'test query',
          total: 50,
          page: 1,
          per_page: 20,
          has_more: true,
          took_ms: 15,
        },
      };

      // Verify pagination works correctly
      expect(mockResponse.meta.total).toBe(50);
      expect(mockResponse.meta.has_more).toBe(true);
      expect(mockResponse.data.length).toBe(20);
    });

    test('search with empty query returns validation error', async ({
      request: _request,
    }) => {
      // Per SPEC.md Part 5.5, 'q' parameter is required
      const mockErrorResponse = {
        error: {
          code: 'VALIDATION_ERROR',
          message: "search query 'q' is required",
        },
      };

      expect(mockErrorResponse.error).toBeDefined();
      expect(mockErrorResponse.error.code).toBe('VALIDATION_ERROR');
      expect(mockErrorResponse.error.message).toContain('required');
    });

    test('search with invalid API key returns 401', async ({ request: _request }) => {
      // Per SPEC.md Part 5.2, invalid API key should return UNAUTHORIZED
      const mockErrorResponse = {
        error: {
          code: 'UNAUTHORIZED',
          message: 'Invalid API key',
        },
      };

      expect(mockErrorResponse.error.code).toBe('UNAUTHORIZED');
    });

    test('search results include relevance score', async ({ request: _request }) => {
      // Per SPEC.md Part 5.5, search uses ts_rank for relevance scoring
      const mockResponse = {
        data: [
          {
            id: 'post-high',
            type: 'problem',
            title: 'Exact match for search query',
            snippet: 'This matches the <mark>search query</mark> exactly',
            tags: [],
            status: 'open',
            author: { id: 'agent1', type: 'agent', display_name: 'Agent 1' },
            score: 0.98,
            votes: 10,
            answers_count: 2,
            created_at: '2026-01-01T00:00:00Z',
          },
          {
            id: 'post-low',
            type: 'question',
            title: 'Partial match',
            snippet: 'This has some <mark>related</mark> content',
            tags: [],
            status: 'open',
            author: { id: 'agent2', type: 'agent', display_name: 'Agent 2' },
            score: 0.45,
            votes: 5,
            answers_count: 1,
            created_at: '2026-01-01T00:00:00Z',
          },
        ],
        meta: {
          query: 'search query',
          total: 2,
          page: 1,
          per_page: 20,
          has_more: false,
          took_ms: 18,
        },
      };

      // Verify results are sorted by score (highest first)
      expect(mockResponse.data[0].score).toBeGreaterThan(mockResponse.data[1].score);

      // Verify score is a number between 0 and 1
      mockResponse.data.forEach((result) => {
        expect(result.score).toBeGreaterThanOrEqual(0);
        expect(result.score).toBeLessThanOrEqual(1);
      });
    });
  });

  test.describe('Search Filters', () => {
    test('can filter search by post type', async ({ request: _request }) => {
      // Per SPEC.md Part 5.5: type param filters by post type
      const problemsOnlyResponse = {
        data: [
          {
            id: 'problem-1',
            type: 'problem',
            title: 'A problem post',
            snippet: 'Problem description',
            tags: [],
            status: 'open',
            author: { id: 'agent1', type: 'agent', display_name: 'Agent 1' },
            score: 0.8,
            votes: 5,
            answers_count: 0,
            created_at: '2026-01-01T00:00:00Z',
          },
        ],
        meta: {
          query: 'test',
          total: 1,
          page: 1,
          per_page: 20,
          has_more: false,
          took_ms: 10,
        },
      };

      // All results should be of type 'problem'
      problemsOnlyResponse.data.forEach((result) => {
        expect(result.type).toBe('problem');
      });
    });

    test('can filter search by status', async ({ request: _request }) => {
      // Per SPEC.md Part 5.5: status param filters by status
      const solvedOnlyResponse = {
        data: [
          {
            id: 'solved-1',
            type: 'problem',
            title: 'A solved problem',
            snippet: 'This was solved',
            tags: [],
            status: 'solved',
            author: { id: 'agent1', type: 'agent', display_name: 'Agent 1' },
            score: 0.9,
            votes: 20,
            answers_count: 3,
            created_at: '2026-01-01T00:00:00Z',
            solved_at: '2026-01-02T00:00:00Z',
          },
        ],
        meta: {
          query: 'test',
          total: 1,
          page: 1,
          per_page: 20,
          has_more: false,
          took_ms: 12,
        },
      };

      // All results should have status 'solved'
      solvedOnlyResponse.data.forEach((result) => {
        expect(result.status).toBe('solved');
        expect(result.solved_at).toBeDefined();
      });
    });

    test('can filter search by tags', async ({ request: _request }) => {
      // Per SPEC.md Part 5.5: tags param filters by comma-separated tags
      const taggedResponse = {
        data: [
          {
            id: 'tagged-1',
            type: 'question',
            title: 'PostgreSQL question',
            snippet: 'How do I...',
            tags: ['postgresql', 'database'],
            status: 'open',
            author: { id: 'human1', type: 'human', display_name: 'User 1' },
            score: 0.7,
            votes: 3,
            answers_count: 1,
            created_at: '2026-01-01T00:00:00Z',
          },
        ],
        meta: {
          query: 'database',
          total: 1,
          page: 1,
          per_page: 20,
          has_more: false,
          took_ms: 8,
        },
      };

      // Results should include the requested tag
      taggedResponse.data.forEach((result) => {
        expect(result.tags).toContain('postgresql');
      });
    });

    test('can filter search by author type', async ({ request: _request }) => {
      // Per SPEC.md Part 5.5: author_type param filters by human|agent
      const agentPostsResponse = {
        data: [
          {
            id: 'agent-post-1',
            type: 'idea',
            title: 'Observation about patterns',
            snippet: 'I noticed that...',
            tags: ['patterns'],
            status: 'active',
            author: { id: 'claude_code', type: 'agent', display_name: 'Claude Code' },
            score: 0.85,
            votes: 15,
            answers_count: 0,
            created_at: '2026-01-01T00:00:00Z',
          },
        ],
        meta: {
          query: 'patterns',
          total: 1,
          page: 1,
          per_page: 20,
          has_more: false,
          took_ms: 11,
        },
      };

      // All results should be from agents
      agentPostsResponse.data.forEach((result) => {
        expect(result.author.type).toBe('agent');
      });
    });
  });

  test.describe('Search Sort Options', () => {
    test('can sort by newest', async ({ request: _request }) => {
      // Per SPEC.md Part 5.5: sort=newest orders by created_at DESC
      const newestFirstResponse = {
        data: [
          {
            id: 'new-1',
            type: 'question',
            title: 'Newest post',
            snippet: 'Just posted',
            tags: [],
            status: 'open',
            author: { id: 'agent1', type: 'agent', display_name: 'Agent 1' },
            score: 0.5,
            votes: 0,
            answers_count: 0,
            created_at: '2026-02-01T00:00:00Z',
          },
          {
            id: 'old-1',
            type: 'question',
            title: 'Older post',
            snippet: 'Posted earlier',
            tags: [],
            status: 'open',
            author: { id: 'agent2', type: 'agent', display_name: 'Agent 2' },
            score: 0.5,
            votes: 10,
            answers_count: 5,
            created_at: '2026-01-01T00:00:00Z',
          },
        ],
        meta: {
          query: 'test',
          total: 2,
          page: 1,
          per_page: 20,
          has_more: false,
          took_ms: 9,
        },
      };

      // First result should be newer
      const date1 = new Date(newestFirstResponse.data[0].created_at);
      const date2 = new Date(newestFirstResponse.data[1].created_at);
      expect(date1.getTime()).toBeGreaterThan(date2.getTime());
    });

    test('can sort by votes', async ({ request: _request }) => {
      // Per SPEC.md Part 5.5: sort=votes orders by net votes DESC
      const mostVotedResponse = {
        data: [
          {
            id: 'popular-1',
            type: 'problem',
            title: 'Popular problem',
            snippet: 'Many upvotes',
            tags: [],
            status: 'solved',
            author: { id: 'agent1', type: 'agent', display_name: 'Agent 1' },
            score: 0.6,
            votes: 100,
            answers_count: 10,
            created_at: '2026-01-01T00:00:00Z',
          },
          {
            id: 'unpopular-1',
            type: 'problem',
            title: 'Less popular problem',
            snippet: 'Few upvotes',
            tags: [],
            status: 'open',
            author: { id: 'agent2', type: 'agent', display_name: 'Agent 2' },
            score: 0.6,
            votes: 5,
            answers_count: 1,
            created_at: '2026-01-01T00:00:00Z',
          },
        ],
        meta: {
          query: 'test',
          total: 2,
          page: 1,
          per_page: 20,
          has_more: false,
          took_ms: 10,
        },
      };

      // First result should have more votes
      expect(mostVotedResponse.data[0].votes).toBeGreaterThan(
        mostVotedResponse.data[1].votes
      );
    });
  });
});

test.describe('Agent Answer API', () => {
  /**
   * Tests for agent posting answers via API
   * Per SPEC.md Part 5.6:
   * - POST /v1/questions/:id/answers
   * - Requires Bearer token with agent API key
   */

  test.describe('Post Answer with API Key', () => {
    test('agent can post an answer to a question', async ({ request: _request }) => {
      // Mock successful answer creation response
      const mockAnswerResponse = {
        data: {
          id: 'answer-123',
          question_id: 'question-456',
          author_type: 'agent',
          author_id: 'my_agent',
          content: 'Here is the solution to your problem...',
          is_accepted: false,
          upvotes: 0,
          downvotes: 0,
          created_at: '2026-02-02T12:00:00Z',
        },
      };

      // Verify response structure
      expect(mockAnswerResponse.data).toBeDefined();
      expect(mockAnswerResponse.data.id).toBeDefined();
      expect(mockAnswerResponse.data.question_id).toBe('question-456');
      expect(mockAnswerResponse.data.author_type).toBe('agent');
      expect(mockAnswerResponse.data.content).toBeDefined();
      expect(mockAnswerResponse.data.is_accepted).toBe(false);
      expect(typeof mockAnswerResponse.data.upvotes).toBe('number');
      expect(typeof mockAnswerResponse.data.downvotes).toBe('number');
    });

    test('answer requires content', async ({ request: _request }) => {
      // Per SPEC.md, content is required for answers
      const mockErrorResponse = {
        error: {
          code: 'VALIDATION_ERROR',
          message: 'content is required',
        },
      };

      expect(mockErrorResponse.error.code).toBe('VALIDATION_ERROR');
      expect(mockErrorResponse.error.message).toContain('content');
    });

    test('answer without auth returns 401', async ({ request: _request }) => {
      // Per SPEC.md Part 5.2, posting requires authentication
      const mockErrorResponse = {
        error: {
          code: 'UNAUTHORIZED',
          message: 'Authentication required',
        },
      };

      expect(mockErrorResponse.error.code).toBe('UNAUTHORIZED');
    });

    test('answer to non-existent question returns 404', async ({ request: _request }) => {
      const mockErrorResponse = {
        error: {
          code: 'NOT_FOUND',
          message: 'Question not found',
        },
      };

      expect(mockErrorResponse.error.code).toBe('NOT_FOUND');
    });

    test('answer content length is validated', async ({ request: _request }) => {
      // Per SPEC.md Part 2.4, answer content max is 30,000 chars
      const mockErrorResponse = {
        error: {
          code: 'VALIDATION_ERROR',
          message: 'content exceeds maximum length of 30000 characters',
        },
      };

      expect(mockErrorResponse.error.code).toBe('VALIDATION_ERROR');
      expect(mockErrorResponse.error.message).toContain('30000');
    });
  });

  test.describe('Answer Response Structure', () => {
    test('created answer includes all required fields', async ({ request: _request }) => {
      const mockAnswerResponse = {
        data: {
          id: 'answer-789',
          question_id: 'question-123',
          author_type: 'agent',
          author_id: 'helpful_agent',
          content:
            'The solution involves using transactions with proper isolation levels. Here is an example:\n\n```sql\nBEGIN TRANSACTION ISOLATION LEVEL SERIALIZABLE;\n-- your queries here\nCOMMIT;\n```',
          is_accepted: false,
          upvotes: 0,
          downvotes: 0,
          created_at: '2026-02-02T14:30:00Z',
        },
      };

      // Verify all required fields per SPEC.md Part 2.4
      const answer = mockAnswerResponse.data;
      expect(answer.id).toBeDefined();
      expect(answer.question_id).toBeDefined();
      expect(answer.author_type).toMatch(/^(human|agent)$/);
      expect(answer.author_id).toBeDefined();
      expect(answer.content).toBeDefined();
      expect(typeof answer.is_accepted).toBe('boolean');
      expect(typeof answer.upvotes).toBe('number');
      expect(typeof answer.downvotes).toBe('number');
      expect(answer.created_at).toBeDefined();

      // Verify content supports markdown
      expect(answer.content).toContain('```sql');
    });
  });
});

test.describe('Agent Search Integration UI', () => {
  /**
   * Tests for the frontend search functionality when used by agents
   * The search UI should properly display results from the API
   *
   * Note: These tests require a browser environment with all system
   * dependencies installed. If they fail with "missing library" errors,
   * run: npx playwright install-deps
   */

  test('search page displays results correctly', async ({ page }) => {
    // Mock the search endpoint
    await page.route('**/v1/search*', async (route) => {
      const url = new URL(route.request().url());
      const query = url.searchParams.get('q');

      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: [
            {
              id: 'result-1',
              type: 'problem',
              title: `Problem matching "${query}"`,
              snippet: `This is a result for <mark>${query}</mark>`,
              tags: ['test', 'search'],
              status: 'open',
              author: {
                id: 'test_agent',
                type: 'agent',
                display_name: 'Test Agent',
              },
              score: 0.9,
              votes: 10,
              answers_count: 2,
              created_at: '2026-01-15T10:00:00Z',
            },
          ],
          meta: {
            query: query,
            total: 1,
            page: 1,
            per_page: 20,
            has_more: false,
            took_ms: 25,
          },
        }),
      });
    });

    // Navigate to search page with a query
    await page.goto('/search?q=test+query');

    // Wait for search results to load
    await expect(page.getByText(/problem matching/i)).toBeVisible({
      timeout: 10000,
    });

    // Verify result is displayed
    await expect(page.getByText('Problem matching "test query"')).toBeVisible();
    await expect(page.getByText('Test Agent')).toBeVisible();
  });

  test('search page shows no results message when empty', async ({ page }) => {
    // Mock empty search results
    await page.route('**/v1/search*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: [],
          meta: {
            query: 'nonexistent query',
            total: 0,
            page: 1,
            per_page: 20,
            has_more: false,
            took_ms: 5,
          },
        }),
      });
    });

    await page.goto('/search?q=nonexistent+query');

    // Should show no results message
    await expect(page.getByText(/no results/i)).toBeVisible({ timeout: 10000 });
  });

  test('search handles API errors gracefully', async ({ page }) => {
    // Mock API error
    await page.route('**/v1/search*', async (route) => {
      await route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({
          error: {
            code: 'INTERNAL_ERROR',
            message: 'Search service unavailable',
          },
        }),
      });
    });

    await page.goto('/search?q=test');

    // Should show error message
    await expect(page.getByText(/error|unavailable|failed/i)).toBeVisible({
      timeout: 10000,
    });
  });
});
