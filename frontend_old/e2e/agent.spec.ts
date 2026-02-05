import { test, expect } from '@playwright/test';

/**
 * E2E tests for Agent Registration Flow
 *
 * Per SPEC.md Part 5.2 Authentication and Part 2.7 AI Agents:
 * - Human registers agent
 * - Verify API key returned
 *
 * Per PRD lines 587-589:
 * - E2E: Agent registration
 * - E2E: Agent searches
 * - E2E: Agent posts answer
 */

test.describe('Agent Registration Flow', () => {
  // Set up authenticated state before each test
  test.beforeEach(async ({ page }) => {
    // Set up authenticated state with a mock token
    await page.goto('/');
    await page.evaluate(() => {
      localStorage.setItem('solvr_auth_token', 'test_jwt_token_for_agent_tests');
    });
  });

  test.describe('Settings Agents Tab', () => {
    test('displays agents tab on settings page', async ({ page }) => {
      // Mock the auth/me endpoint to return a user
      await page.route('**/v1/auth/me', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: {
              id: 'user-123',
              username: 'testuser',
              display_name: 'Test User',
              email: 'test@example.com',
              bio: '',
              avatar_url: null,
            },
          }),
        });
      });

      // Mock the agents list endpoint
      await page.route('**/v1/users/*/agents', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([]),
        });
      });

      await page.goto('/settings');

      // Verify settings page loads
      await expect(page.getByRole('heading', { name: /settings/i })).toBeVisible();

      // Click on Agents tab
      const agentsTab = page.getByRole('tab', { name: /agents/i });
      await expect(agentsTab).toBeVisible();
      await agentsTab.click();

      // Verify agents tab content is shown
      await expect(page.getByText(/create new agent/i)).toBeVisible();
    });

    test('shows empty state when no agents exist', async ({ page }) => {
      // Mock the auth/me endpoint
      await page.route('**/v1/auth/me', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: {
              id: 'user-123',
              username: 'testuser',
              display_name: 'Test User',
              email: 'test@example.com',
              bio: '',
              avatar_url: null,
            },
          }),
        });
      });

      // Mock empty agents list
      await page.route('**/v1/users/*/agents', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([]),
        });
      });

      await page.goto('/settings');

      // Click on Agents tab
      await page.getByRole('tab', { name: /agents/i }).click();

      // Verify empty state message
      await expect(page.getByText(/no agents registered yet/i)).toBeVisible();
    });

    test('shows existing agents in the list', async ({ page }) => {
      // Mock the auth/me endpoint
      await page.route('**/v1/auth/me', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: {
              id: 'user-123',
              username: 'testuser',
              display_name: 'Test User',
              email: 'test@example.com',
              bio: '',
              avatar_url: null,
            },
          }),
        });
      });

      // Mock agents list with one agent
      await page.route('**/v1/users/*/agents', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([
            {
              id: 'my_agent',
              display_name: 'My Agent',
              bio: 'A helpful AI assistant',
              created_at: '2026-01-01T00:00:00Z',
              human_id: 'user-123',
            },
          ]),
        });
      });

      await page.goto('/settings');

      // Click on Agents tab
      await page.getByRole('tab', { name: /agents/i }).click();

      // Verify agent is shown in list
      await expect(page.getByText('My Agent')).toBeVisible();
      await expect(page.getByText('@my_agent')).toBeVisible();
      await expect(page.getByText('A helpful AI assistant')).toBeVisible();
    });
  });

  test.describe('Agent Creation Form', () => {
    test.beforeEach(async ({ page }) => {
      // Mock the auth/me endpoint
      await page.route('**/v1/auth/me', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: {
              id: 'user-123',
              username: 'testuser',
              display_name: 'Test User',
              email: 'test@example.com',
              bio: '',
              avatar_url: null,
            },
          }),
        });
      });

      // Mock empty agents list initially
      await page.route('**/v1/users/*/agents', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([]),
        });
      });
    });

    test('shows create agent form when clicking create button', async ({ page }) => {
      await page.goto('/settings');
      await page.getByRole('tab', { name: /agents/i }).click();

      // Click create new agent button
      await page.getByRole('button', { name: /create new agent/i }).click();

      // Verify form is shown
      await expect(page.getByLabel(/agent id/i)).toBeVisible();
      await expect(page.getByLabel(/display name/i)).toBeVisible();
      await expect(page.getByLabel(/bio/i)).toBeVisible();
    });

    test('validates required fields', async ({ page }) => {
      await page.goto('/settings');
      await page.getByRole('tab', { name: /agents/i }).click();
      await page.getByRole('button', { name: /create new agent/i }).click();

      // Try to submit without filling required fields
      await page.getByRole('button', { name: /^create$/i }).click();

      // Verify validation errors
      await expect(page.getByText(/agent id is required/i)).toBeVisible();
    });

    test('validates agent ID format', async ({ page }) => {
      await page.goto('/settings');
      await page.getByRole('tab', { name: /agents/i }).click();
      await page.getByRole('button', { name: /create new agent/i }).click();

      // Fill in invalid agent ID with spaces/special chars
      await page.getByLabel(/agent id/i).fill('my agent!');
      await page.getByLabel(/display name/i).fill('My Agent');
      await page.getByRole('button', { name: /^create$/i }).click();

      // Verify validation error for agent ID format
      await expect(
        page.getByText(/only letters, numbers, and underscores/i)
      ).toBeVisible();
    });

    test('can cancel agent creation', async ({ page }) => {
      await page.goto('/settings');
      await page.getByRole('tab', { name: /agents/i }).click();
      await page.getByRole('button', { name: /create new agent/i }).click();

      // Verify form is shown
      await expect(page.getByLabel(/agent id/i)).toBeVisible();

      // Click cancel
      await page.getByRole('button', { name: /cancel/i }).click();

      // Verify form is hidden
      await expect(page.getByLabel(/agent id/i)).not.toBeVisible();
    });
  });

  test.describe('Successful Agent Registration', () => {
    test('successfully creates agent and shows API key', async ({ page }) => {
      // Mock the auth/me endpoint
      await page.route('**/v1/auth/me', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: {
              id: 'user-123',
              username: 'testuser',
              display_name: 'Test User',
              email: 'test@example.com',
              bio: '',
              avatar_url: null,
            },
          }),
        });
      });

      // Mock empty agents list initially
      await page.route('**/v1/users/*/agents', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([]),
        });
      });

      // Mock the create agent endpoint
      await page.route('**/v1/agents', async (route) => {
        if (route.request().method() === 'POST') {
          await route.fulfill({
            status: 201,
            contentType: 'application/json',
            body: JSON.stringify({
              data: {
                agent: {
                  id: 'test_agent',
                  display_name: 'Test Agent',
                  bio: 'A test AI agent',
                  created_at: '2026-02-02T00:00:00Z',
                  human_id: 'user-123',
                },
                api_key: 'solvr_test_api_key_1234567890abcdef',
              },
            }),
          });
        } else {
          await route.continue();
        }
      });

      await page.goto('/settings');
      await page.getByRole('tab', { name: /agents/i }).click();
      await page.getByRole('button', { name: /create new agent/i }).click();

      // Fill in the form
      await page.getByLabel(/agent id/i).fill('test_agent');
      await page.getByLabel(/display name/i).fill('Test Agent');
      await page.getByLabel(/bio/i).fill('A test AI agent');

      // Submit the form
      await page.getByRole('button', { name: /^create$/i }).click();

      // Verify success message and API key display
      await expect(page.getByText(/agent created/i)).toBeVisible();
      await expect(page.getByText(/save this api key/i)).toBeVisible();

      // Verify API key is shown
      await expect(page.getByText(/solvr_test_api_key/i)).toBeVisible();

      // Verify the new agent appears in the list
      await expect(page.getByText('Test Agent')).toBeVisible();
      await expect(page.getByText('@test_agent')).toBeVisible();
    });

    test('API key can be copied to clipboard', async ({ page, context }) => {
      // Grant clipboard permissions
      await context.grantPermissions(['clipboard-read', 'clipboard-write']);

      // Mock the auth/me endpoint
      await page.route('**/v1/auth/me', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: {
              id: 'user-123',
              username: 'testuser',
              display_name: 'Test User',
              email: 'test@example.com',
              bio: '',
              avatar_url: null,
            },
          }),
        });
      });

      // Mock empty agents list
      await page.route('**/v1/users/*/agents', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([]),
        });
      });

      // Mock the create agent endpoint
      await page.route('**/v1/agents', async (route) => {
        if (route.request().method() === 'POST') {
          await route.fulfill({
            status: 201,
            contentType: 'application/json',
            body: JSON.stringify({
              data: {
                agent: {
                  id: 'copy_test_agent',
                  display_name: 'Copy Test Agent',
                  bio: '',
                  created_at: '2026-02-02T00:00:00Z',
                  human_id: 'user-123',
                },
                api_key: 'solvr_copyable_api_key_xyz',
              },
            }),
          });
        } else {
          await route.continue();
        }
      });

      await page.goto('/settings');
      await page.getByRole('tab', { name: /agents/i }).click();
      await page.getByRole('button', { name: /create new agent/i }).click();

      // Fill and submit form
      await page.getByLabel(/agent id/i).fill('copy_test_agent');
      await page.getByLabel(/display name/i).fill('Copy Test Agent');
      await page.getByRole('button', { name: /^create$/i }).click();

      // Wait for success message
      await expect(page.getByText(/agent created/i)).toBeVisible();

      // Click copy button
      await page.getByRole('button', { name: /copy/i }).click();

      // Verify copied feedback
      await expect(page.getByText(/copied/i)).toBeVisible();
    });

    test('API key display can be dismissed', async ({ page }) => {
      // Mock the auth/me endpoint
      await page.route('**/v1/auth/me', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: {
              id: 'user-123',
              username: 'testuser',
              display_name: 'Test User',
              email: 'test@example.com',
              bio: '',
              avatar_url: null,
            },
          }),
        });
      });

      // Mock empty agents list
      await page.route('**/v1/users/*/agents', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([]),
        });
      });

      // Mock the create agent endpoint
      await page.route('**/v1/agents', async (route) => {
        if (route.request().method() === 'POST') {
          await route.fulfill({
            status: 201,
            contentType: 'application/json',
            body: JSON.stringify({
              data: {
                agent: {
                  id: 'dismiss_test_agent',
                  display_name: 'Dismiss Test Agent',
                  bio: '',
                  created_at: '2026-02-02T00:00:00Z',
                  human_id: 'user-123',
                },
                api_key: 'solvr_dismissable_key_abc',
              },
            }),
          });
        } else {
          await route.continue();
        }
      });

      await page.goto('/settings');
      await page.getByRole('tab', { name: /agents/i }).click();
      await page.getByRole('button', { name: /create new agent/i }).click();

      // Fill and submit form
      await page.getByLabel(/agent id/i).fill('dismiss_test_agent');
      await page.getByLabel(/display name/i).fill('Dismiss Test Agent');
      await page.getByRole('button', { name: /^create$/i }).click();

      // Wait for success message
      await expect(page.getByText(/agent created/i)).toBeVisible();

      // Click dismiss button
      await page.getByText(/i've saved the key/i).click();

      // Verify API key display is hidden
      await expect(page.getByText(/save this api key/i)).not.toBeVisible();
    });
  });

  test.describe('Agent Registration Error Handling', () => {
    test('handles duplicate agent ID error', async ({ page }) => {
      // Mock the auth/me endpoint
      await page.route('**/v1/auth/me', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: {
              id: 'user-123',
              username: 'testuser',
              display_name: 'Test User',
              email: 'test@example.com',
              bio: '',
              avatar_url: null,
            },
          }),
        });
      });

      // Mock empty agents list
      await page.route('**/v1/users/*/agents', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([]),
        });
      });

      // Mock the create agent endpoint to return duplicate error
      await page.route('**/v1/agents', async (route) => {
        if (route.request().method() === 'POST') {
          await route.fulfill({
            status: 409,
            contentType: 'application/json',
            body: JSON.stringify({
              error: {
                code: 'DUPLICATE_ID',
                message: 'This agent ID is already taken',
              },
            }),
          });
        } else {
          await route.continue();
        }
      });

      await page.goto('/settings');
      await page.getByRole('tab', { name: /agents/i }).click();
      await page.getByRole('button', { name: /create new agent/i }).click();

      // Fill and submit form
      await page.getByLabel(/agent id/i).fill('existing_agent');
      await page.getByLabel(/display name/i).fill('Existing Agent');
      await page.getByRole('button', { name: /^create$/i }).click();

      // Verify error message
      await expect(page.getByText(/already taken/i)).toBeVisible();
    });

    test('handles API error gracefully', async ({ page }) => {
      // Mock the auth/me endpoint
      await page.route('**/v1/auth/me', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: {
              id: 'user-123',
              username: 'testuser',
              display_name: 'Test User',
              email: 'test@example.com',
              bio: '',
              avatar_url: null,
            },
          }),
        });
      });

      // Mock empty agents list
      await page.route('**/v1/users/*/agents', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([]),
        });
      });

      // Mock the create agent endpoint to return server error
      await page.route('**/v1/agents', async (route) => {
        if (route.request().method() === 'POST') {
          await route.fulfill({
            status: 500,
            contentType: 'application/json',
            body: JSON.stringify({
              error: {
                code: 'INTERNAL_ERROR',
                message: 'Internal server error',
              },
            }),
          });
        } else {
          await route.continue();
        }
      });

      await page.goto('/settings');
      await page.getByRole('tab', { name: /agents/i }).click();
      await page.getByRole('button', { name: /create new agent/i }).click();

      // Fill and submit form
      await page.getByLabel(/agent id/i).fill('error_agent');
      await page.getByLabel(/display name/i).fill('Error Agent');
      await page.getByRole('button', { name: /^create$/i }).click();

      // Verify error message is shown
      await expect(page.getByText(/internal server error/i)).toBeVisible();
    });
  });

  test.describe('Unauthenticated User', () => {
    test('redirects to login when not authenticated', async ({ page }) => {
      // Clear any existing token
      await page.goto('/');
      await page.evaluate(() => {
        localStorage.removeItem('solvr_auth_token');
      });

      // Mock the auth/me endpoint to return 401
      await page.route('**/v1/auth/me', async (route) => {
        await route.fulfill({
          status: 401,
          contentType: 'application/json',
          body: JSON.stringify({
            error: {
              code: 'UNAUTHORIZED',
              message: 'Not authenticated',
            },
          }),
        });
      });

      // Try to access settings page
      await page.goto('/settings');

      // Should redirect to login
      await page.waitForURL('**/login*', { timeout: 10000 });
      expect(page.url()).toContain('/login');
    });
  });

  test.describe('Complete Agent Registration Journey', () => {
    test('full flow: create agent, see API key, agent appears in list', async ({
      page,
    }) => {
      // Mock the auth/me endpoint
      await page.route('**/v1/auth/me', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: {
              id: 'user-123',
              username: 'testuser',
              display_name: 'Test User',
              email: 'test@example.com',
              bio: '',
              avatar_url: null,
            },
          }),
        });
      });

      // Track agents state to simulate agent being added
      let agents: unknown[] = [];

      // Mock agents list endpoint
      await page.route('**/v1/users/*/agents', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(agents),
        });
      });

      // Mock the create agent endpoint
      await page.route('**/v1/agents', async (route) => {
        if (route.request().method() === 'POST') {
          const newAgent = {
            id: 'journey_agent',
            display_name: 'Journey Agent',
            bio: 'Created during full journey test',
            created_at: '2026-02-02T00:00:00Z',
            human_id: 'user-123',
          };
          agents = [newAgent];

          await route.fulfill({
            status: 201,
            contentType: 'application/json',
            body: JSON.stringify({
              data: {
                agent: newAgent,
                api_key: 'solvr_journey_api_key_full_test_12345',
              },
            }),
          });
        } else {
          await route.continue();
        }
      });

      // Step 1: Navigate to settings and agents tab
      await page.goto('/settings');
      await page.getByRole('tab', { name: /agents/i }).click();

      // Step 2: Verify empty state
      await expect(page.getByText(/no agents registered yet/i)).toBeVisible();

      // Step 3: Open create form
      await page.getByRole('button', { name: /create new agent/i }).click();

      // Step 4: Fill the form
      await page.getByLabel(/agent id/i).fill('journey_agent');
      await page.getByLabel(/display name/i).fill('Journey Agent');
      await page.getByLabel(/bio/i).fill('Created during full journey test');

      // Step 5: Submit the form
      await page.getByRole('button', { name: /^create$/i }).click();

      // Step 6: Verify API key is displayed
      await expect(page.getByText(/agent created/i)).toBeVisible();
      await expect(
        page.getByText(/solvr_journey_api_key_full_test_12345/)
      ).toBeVisible();

      // Step 7: Verify agent appears in list (it should show even before dismissing)
      await expect(page.getByText('Journey Agent')).toBeVisible();
      await expect(page.getByText('@journey_agent')).toBeVisible();

      // Step 8: Dismiss the API key display
      await page.getByText(/i've saved the key/i).click();

      // Step 9: Verify list still shows the agent
      await expect(page.getByText('Journey Agent')).toBeVisible();
      await expect(page.getByText('@journey_agent')).toBeVisible();
      await expect(
        page.getByText('Created during full journey test')
      ).toBeVisible();

      // Step 10: Verify manage link exists
      await expect(page.getByRole('link', { name: /manage/i })).toBeVisible();
    });
  });
});
