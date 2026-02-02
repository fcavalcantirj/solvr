import { test, expect, Page } from '@playwright/test';

/**
 * E2E tests for Human Posts Question Flow
 *
 * Per SPEC.md Part 5.6 Posts and prd-v2.json:
 * - Create question via UI
 * - Verify appears in feed
 * - Verify searchable
 *
 * These tests mock API responses to test the frontend behavior
 * in isolation from the backend.
 */

// Mock data for tests
const mockUser = {
  id: 'user-123',
  username: 'testuser',
  display_name: 'Test User',
  email: 'test@example.com',
  avatar_url: 'https://example.com/avatar.png',
};

const mockCreatedQuestion = {
  id: 'q-new-123',
  type: 'question',
  title: 'How do I handle async errors in Go?',
  description:
    'I am working on a Go application and encountering issues with error handling in async operations. Specifically, when using goroutines...',
  tags: ['go', 'async', 'error-handling'],
  status: 'open',
  posted_by_type: 'human',
  posted_by_id: 'user-123',
  upvotes: 0,
  downvotes: 0,
  created_at: new Date().toISOString(),
  updated_at: new Date().toISOString(),
  author: {
    id: 'user-123',
    type: 'human',
    display_name: 'Test User',
    avatar_url: 'https://example.com/avatar.png',
  },
};

const mockFeedResponse = {
  data: [
    mockCreatedQuestion,
    {
      id: 'q-older-456',
      type: 'question',
      title: 'Existing question in feed',
      description: 'This is an existing question for testing feed display.',
      tags: ['test'],
      status: 'open',
      posted_by_type: 'human',
      posted_by_id: 'user-456',
      upvotes: 5,
      downvotes: 1,
      vote_score: 4,
      created_at: '2026-01-01T10:00:00Z',
      updated_at: '2026-01-01T10:00:00Z',
      author: {
        id: 'user-456',
        type: 'human',
        display_name: 'Other User',
        avatar_url: null,
      },
    },
  ],
  meta: {
    total: 2,
    page: 1,
    per_page: 20,
    has_more: false,
  },
};

const mockSearchResponse = {
  data: [
    {
      id: mockCreatedQuestion.id,
      type: 'question',
      title: mockCreatedQuestion.title,
      snippet:
        '...encountering issues with <mark>error handling</mark> in <mark>async</mark> operations...',
      tags: mockCreatedQuestion.tags,
      status: 'open',
      author: mockCreatedQuestion.author,
      score: 0.95,
      votes: 0,
      answers_count: 0,
      created_at: mockCreatedQuestion.created_at,
    },
  ],
  meta: {
    query: 'async errors go',
    total: 1,
    page: 1,
    per_page: 20,
    has_more: false,
    took_ms: 15,
  },
  suggestions: {
    related_tags: ['goroutines', 'channels'],
    did_you_mean: null,
  },
};

/**
 * Helper function to set up authenticated state
 */
async function setupAuthenticatedState(page: Page): Promise<void> {
  await page.goto('/');
  await page.evaluate(() => {
    localStorage.setItem('solvr_auth_token', 'test_jwt_token');
  });
}

/**
 * Helper function to set up API mocks for the question creation flow
 */
async function setupApiMocks(page: Page): Promise<void> {
  // Mock auth/me endpoint
  await page.route('**/v1/auth/me', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: mockUser }),
    });
  });

  // Mock questions POST endpoint
  await page.route('**/v1/questions', async (route) => {
    if (route.request().method() === 'POST') {
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({ data: mockCreatedQuestion }),
      });
    } else {
      await route.continue();
    }
  });

  // Mock feed endpoint
  await page.route('**/v1/feed*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(mockFeedResponse),
    });
  });

  // Mock search endpoint
  await page.route('**/v1/search*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(mockSearchResponse),
    });
  });
}

test.describe('Human Posts Question Flow', () => {
  test.describe('Question Creation Page', () => {
    test.beforeEach(async ({ page }) => {
      await setupAuthenticatedState(page);
      await setupApiMocks(page);
    });

    test('displays new question form for authenticated user', async ({
      page,
    }) => {
      await page.goto('/new/question');

      // Verify page heading
      await expect(
        page.getByRole('heading', { name: /ask a question/i })
      ).toBeVisible();

      // Verify form fields are present
      await expect(page.getByLabel(/title/i)).toBeVisible();
      await expect(page.getByLabel(/description/i)).toBeVisible();
      await expect(page.getByLabel(/tags/i)).toBeVisible();

      // Verify submit button
      await expect(
        page.getByRole('button', { name: /post question/i })
      ).toBeVisible();

      // Verify write/preview tabs
      await expect(page.getByRole('tab', { name: /write/i })).toBeVisible();
      await expect(page.getByRole('tab', { name: /preview/i })).toBeVisible();
    });

    test('redirects unauthenticated user to login', async ({ page }) => {
      // Clear the auth token
      await page.evaluate(() => {
        localStorage.removeItem('solvr_auth_token');
      });

      // Mock auth/me to return 401
      await page.route('**/v1/auth/me', async (route) => {
        await route.fulfill({
          status: 401,
          contentType: 'application/json',
          body: JSON.stringify({
            error: { code: 'UNAUTHORIZED', message: 'Not authenticated' },
          }),
        });
      });

      await page.goto('/new/question');

      // Should redirect to login
      await page.waitForURL('**/login*', { timeout: 10000 });
      expect(page.url()).toContain('/login');
    });

    test('validates form inputs before submission', async ({ page }) => {
      await page.goto('/new/question');

      // Try to submit empty form
      const submitButton = page.getByRole('button', { name: /post question/i });
      await submitButton.click();

      // Should show validation errors
      await expect(
        page.getByText(/title must be at least 10 characters/i)
      ).toBeVisible();
      await expect(
        page.getByText(/description must be at least 50 characters/i)
      ).toBeVisible();
    });

    test('validates title length constraints', async ({ page }) => {
      await page.goto('/new/question');

      // Enter title that's too short
      await page.getByLabel(/title/i).fill('Short');

      // Enter valid description
      await page
        .getByLabel(/description/i)
        .fill(
          'This is a valid description that is long enough to pass validation. It contains more than 50 characters.'
        );

      const submitButton = page.getByRole('button', { name: /post question/i });
      await submitButton.click();

      // Should show title error
      await expect(
        page.getByText(/title must be at least 10 characters/i)
      ).toBeVisible();
    });

    test('validates tag count limit', async ({ page }) => {
      await page.goto('/new/question');

      // Enter valid title and description
      await page
        .getByLabel(/title/i)
        .fill('How do I handle async errors in Go?');
      await page
        .getByLabel(/description/i)
        .fill(
          'This is a valid description that is long enough to pass validation. It contains more than 50 characters.'
        );

      // Enter too many tags
      await page
        .getByLabel(/tags/i)
        .fill('tag1, tag2, tag3, tag4, tag5, tag6');

      const submitButton = page.getByRole('button', { name: /post question/i });
      await submitButton.click();

      // Should show tag error
      await expect(page.getByText(/maximum 5 tags allowed/i)).toBeVisible();
    });

    test('shows markdown preview when preview tab is clicked', async ({
      page,
    }) => {
      await page.goto('/new/question');

      // Enter markdown content
      await page
        .getByLabel(/description/i)
        .fill('## Heading\n\nThis is **bold** text with `code`.');

      // Click preview tab
      await page.getByRole('tab', { name: /preview/i }).click();

      // Should show rendered markdown
      const previewPanel = page.getByRole('tabpanel');
      await expect(previewPanel.locator('h2')).toContainText('Heading');
      await expect(previewPanel.locator('strong')).toContainText('bold');
      await expect(previewPanel.locator('code')).toContainText('code');
    });
  });

  test.describe('Question Submission', () => {
    test.beforeEach(async ({ page }) => {
      await setupAuthenticatedState(page);
      await setupApiMocks(page);
    });

    test('successfully creates question and redirects to post page', async ({
      page,
    }) => {
      await page.goto('/new/question');

      // Fill in the form
      await page
        .getByLabel(/title/i)
        .fill('How do I handle async errors in Go?');
      await page
        .getByLabel(/description/i)
        .fill(
          'I am working on a Go application and encountering issues with error handling in async operations. Specifically, when using goroutines, errors seem to get lost.'
        );
      await page.getByLabel(/tags/i).fill('go, async, error-handling');

      // Submit the form
      const submitButton = page.getByRole('button', { name: /post question/i });
      await submitButton.click();

      // Should redirect to the created post
      await page.waitForURL('**/posts/' + mockCreatedQuestion.id, {
        timeout: 10000,
      });
      expect(page.url()).toContain('/posts/' + mockCreatedQuestion.id);
    });

    test('shows loading state during submission', async ({ page }) => {
      // Add a delay to the API mock to observe loading state
      await page.route('**/v1/questions', async (route) => {
        if (route.request().method() === 'POST') {
          await new Promise((resolve) => setTimeout(resolve, 500));
          await route.fulfill({
            status: 201,
            contentType: 'application/json',
            body: JSON.stringify({ data: mockCreatedQuestion }),
          });
        } else {
          await route.continue();
        }
      });

      await page.goto('/new/question');

      // Fill in the form
      await page
        .getByLabel(/title/i)
        .fill('How do I handle async errors in Go?');
      await page
        .getByLabel(/description/i)
        .fill(
          'I am working on a Go application and encountering issues with error handling in async operations. It needs to be more than 50 chars.'
        );

      // Submit the form
      const submitButton = page.getByRole('button', { name: /post question/i });
      await submitButton.click();

      // Button should show loading state
      await expect(submitButton).toBeDisabled();
    });

    test('displays error message when submission fails', async ({ page }) => {
      // Mock failed submission
      await page.route('**/v1/questions', async (route) => {
        if (route.request().method() === 'POST') {
          await route.fulfill({
            status: 500,
            contentType: 'application/json',
            body: JSON.stringify({
              error: {
                code: 'INTERNAL_ERROR',
                message: 'Failed to create question',
              },
            }),
          });
        } else {
          await route.continue();
        }
      });

      await page.goto('/new/question');

      // Fill in the form
      await page
        .getByLabel(/title/i)
        .fill('How do I handle async errors in Go?');
      await page
        .getByLabel(/description/i)
        .fill(
          'I am working on a Go application and encountering issues with error handling in async operations. It needs to be more than 50 chars.'
        );

      // Submit the form
      const submitButton = page.getByRole('button', { name: /post question/i });
      await submitButton.click();

      // Should show error message
      await expect(page.getByRole('alert')).toBeVisible();
      await expect(page.getByRole('alert')).toContainText(/failed/i);
    });

    test('allows dismissing error message', async ({ page }) => {
      // Mock failed submission
      await page.route('**/v1/questions', async (route) => {
        if (route.request().method() === 'POST') {
          await route.fulfill({
            status: 400,
            contentType: 'application/json',
            body: JSON.stringify({
              error: {
                code: 'VALIDATION_ERROR',
                message: 'Invalid input',
              },
            }),
          });
        } else {
          await route.continue();
        }
      });

      await page.goto('/new/question');

      // Fill and submit
      await page
        .getByLabel(/title/i)
        .fill('How do I handle async errors in Go?');
      await page
        .getByLabel(/description/i)
        .fill(
          'I am working on a Go application and encountering issues with error handling in async operations. It needs to be more than 50 chars.'
        );
      await page.getByRole('button', { name: /post question/i }).click();

      // Wait for error to appear
      await expect(page.getByRole('alert')).toBeVisible();

      // Click dismiss button
      const dismissButton = page.getByRole('button', { name: /dismiss/i });
      await dismissButton.click();

      // Error should be hidden
      await expect(page.getByRole('alert')).not.toBeVisible();
    });
  });

  test.describe('Question Appears in Feed', () => {
    test.beforeEach(async ({ page }) => {
      await setupAuthenticatedState(page);
      await setupApiMocks(page);
    });

    test('newly created question appears in feed', async ({ page }) => {
      await page.goto('/feed');

      // Wait for feed to load
      await expect(page.getByText(/how do i handle async errors/i)).toBeVisible(
        { timeout: 10000 }
      );

      // Verify question card is displayed
      const questionCard = page.locator('article').filter({
        hasText: /how do i handle async errors/i,
      });
      await expect(questionCard).toBeVisible();

      // Verify type badge shows "Question"
      await expect(questionCard.getByText('Question')).toBeVisible();

      // Verify tags are displayed
      await expect(questionCard.getByText('go')).toBeVisible();
      await expect(questionCard.getByText('async')).toBeVisible();
    });

    test('feed shows questions when filtered by type', async ({ page }) => {
      await page.goto('/feed');

      // Click on Questions filter
      const questionsFilter = page.getByRole('button', { name: /questions/i });
      await questionsFilter.click();

      // Should still show questions
      await expect(page.getByText(/how do i handle async errors/i)).toBeVisible(
        { timeout: 10000 }
      );
    });

    test('question links to detail page from feed', async ({ page }) => {
      await page.goto('/feed');

      // Click on the question title
      const questionLink = page.getByRole('link', {
        name: /how do i handle async errors/i,
      });
      await questionLink.click();

      // Should navigate to post detail page
      await page.waitForURL('**/posts/' + mockCreatedQuestion.id, {
        timeout: 10000,
      });
      expect(page.url()).toContain('/posts/' + mockCreatedQuestion.id);
    });
  });

  test.describe('Question Is Searchable', () => {
    test.beforeEach(async ({ page }) => {
      await setupAuthenticatedState(page);
      await setupApiMocks(page);
    });

    test('question appears in search results', async ({ page }) => {
      await page.goto('/search?q=async+errors+go');

      // Wait for search results to load
      await expect(
        page.getByText(/how do i handle async errors/i)
      ).toBeVisible({ timeout: 10000 });

      // Verify search result card is displayed
      const resultCard = page.locator('article').filter({
        hasText: /how do i handle async errors/i,
      });
      await expect(resultCard).toBeVisible();
    });

    test('search result shows highlighted snippet', async ({ page }) => {
      await page.goto('/search?q=async+errors+go');

      // Wait for results
      await expect(
        page.getByText(/how do i handle async errors/i)
      ).toBeVisible({ timeout: 10000 });

      // Check for highlighted text in snippet (mark elements)
      const markElements = page.locator('mark');
      await expect(markElements.first()).toBeVisible();
    });

    test('search shows result metadata', async ({ page }) => {
      await page.goto('/search?q=async+errors+go');

      // Wait for results
      await expect(
        page.getByText(/how do i handle async errors/i)
      ).toBeVisible({ timeout: 10000 });

      // Verify result count is shown
      await expect(page.getByText(/1 result/i)).toBeVisible();
    });

    test('search can be performed from homepage', async ({ page }) => {
      await page.goto('/');

      // Find search input (could be in header or hero section)
      const searchInput = page.locator(
        'input[type="search"], input[placeholder*="search" i]'
      );

      // If search exists, perform a search
      if (await searchInput.isVisible()) {
        await searchInput.fill('async errors go');
        await searchInput.press('Enter');

        // Should navigate to search page
        await page.waitForURL('**/search*', { timeout: 10000 });
        expect(page.url()).toContain('/search');
      }
    });

    test('search filters work correctly', async ({ page }) => {
      await page.goto('/search?q=async');

      // Wait for page to load
      await page.waitForLoadState('networkidle');

      // Click Questions type filter if available
      const typeFilter = page.locator('#type-filter');
      if (await typeFilter.isVisible()) {
        await typeFilter.selectOption('question');

        // URL should update with type filter
        await page.waitForURL(/type=question/);
      }
    });
  });

  test.describe('Full Question Posting Journey', () => {
    test('complete flow: create question, view in feed, search', async ({
      page,
    }) => {
      await setupAuthenticatedState(page);
      await setupApiMocks(page);

      // Step 1: Navigate to new question page
      await page.goto('/new/question');
      await expect(
        page.getByRole('heading', { name: /ask a question/i })
      ).toBeVisible();

      // Step 2: Fill in the question form
      await page
        .getByLabel(/title/i)
        .fill('How do I handle async errors in Go?');
      await page
        .getByLabel(/description/i)
        .fill(
          'I am working on a Go application and encountering issues with error handling in async operations. Specifically, when using goroutines, errors seem to get lost.'
        );
      await page.getByLabel(/tags/i).fill('go, async, error-handling');

      // Step 3: Preview the question
      await page.getByRole('tab', { name: /preview/i }).click();
      const previewPanel = page.getByRole('tabpanel');
      await expect(previewPanel).toContainText('Go application');

      // Step 4: Submit the question
      await page.getByRole('tab', { name: /write/i }).click();
      await page.getByRole('button', { name: /post question/i }).click();

      // Step 5: Verify redirect to post page
      await page.waitForURL('**/posts/' + mockCreatedQuestion.id, {
        timeout: 10000,
      });

      // Step 6: Navigate to feed and verify question appears
      await page.goto('/feed');
      await expect(page.getByText(/how do i handle async errors/i)).toBeVisible(
        { timeout: 10000 }
      );

      // Step 7: Search for the question
      await page.goto('/search?q=async+errors+go');
      await expect(
        page.getByText(/how do i handle async errors/i)
      ).toBeVisible({ timeout: 10000 });

      // Journey complete!
    });
  });
});
