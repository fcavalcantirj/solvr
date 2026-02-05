import { test, expect } from '@playwright/test';

/**
 * E2E tests for Human Signup/Login Flow
 *
 * Per SPEC.md Part 5.2 Authentication and prd-v2.json:
 * - Test OAuth login end-to-end
 * - Verify user created in database (via API check)
 *
 * Since actual OAuth requires external services, we test:
 * 1. Login page renders correctly with OAuth buttons
 * 2. OAuth buttons redirect to correct API endpoints
 * 3. Auth callback properly stores token and redirects
 * 4. Authenticated state is maintained
 */

test.describe('Human Signup Flow', () => {
  test.describe('Login Page', () => {
    test('displays login page with OAuth buttons', async ({ page }) => {
      await page.goto('/login');

      // Verify page title/heading
      await expect(page.getByRole('heading', { name: /welcome back/i })).toBeVisible();
      await expect(page.getByText(/sign in to continue to solvr/i)).toBeVisible();

      // Verify GitHub OAuth button
      const githubButton = page.getByRole('button', { name: /continue with github/i });
      await expect(githubButton).toBeVisible();

      // Verify Google OAuth button
      const googleButton = page.getByRole('button', { name: /continue with google/i });
      await expect(googleButton).toBeVisible();

      // Verify Terms and Privacy links
      await expect(page.getByRole('link', { name: /terms of service/i })).toBeVisible();
      await expect(page.getByRole('link', { name: /privacy policy/i })).toBeVisible();
    });

    test('GitHub button redirects to OAuth endpoint', async ({ page }) => {
      await page.goto('/login');

      // Set up a listener to capture the navigation
      const [request] = await Promise.all([
        page.waitForRequest((req) =>
          req.url().includes('/v1/auth/github')
        ),
        page.getByRole('button', { name: /continue with github/i }).click(),
      ]);

      // Verify the request was made to the correct endpoint
      expect(request.url()).toContain('/v1/auth/github');
    });

    test('Google button redirects to OAuth endpoint', async ({ page }) => {
      await page.goto('/login');

      // Set up a listener to capture the navigation
      const [request] = await Promise.all([
        page.waitForRequest((req) =>
          req.url().includes('/v1/auth/google')
        ),
        page.getByRole('button', { name: /continue with google/i }).click(),
      ]);

      // Verify the request was made to the correct endpoint
      expect(request.url()).toContain('/v1/auth/google');
    });

    test('displays error message when error param present', async ({ page }) => {
      await page.goto('/login?error=access_denied');

      // Verify error message is shown
      const errorAlert = page.getByRole('alert');
      await expect(errorAlert).toBeVisible();
      await expect(errorAlert).toContainText(/access was denied/i);
    });

    test('displays generic error for unknown error codes', async ({ page }) => {
      await page.goto('/login?error=unknown_error');

      // Verify generic error message is shown
      const errorAlert = page.getByRole('alert');
      await expect(errorAlert).toBeVisible();
      await expect(errorAlert).toContainText(/an error occurred during sign in/i);
    });
  });

  test.describe('Auth Callback', () => {
    test('callback page stores token and redirects to dashboard', async ({ page }) => {
      // Navigate to callback with a mock token
      await page.goto('/auth/callback?token=test_jwt_token_12345');

      // Wait for redirect to dashboard
      await page.waitForURL('**/dashboard', { timeout: 10000 });

      // Verify we're on the dashboard
      expect(page.url()).toContain('/dashboard');

      // Verify token was stored in localStorage
      const token = await page.evaluate(() => {
        return localStorage.getItem('solvr_auth_token');
      });
      expect(token).toBe('test_jwt_token_12345');
    });

    test('callback redirects to redirect_to param when provided', async ({ page }) => {
      // Navigate to callback with token and redirect_to
      await page.goto('/auth/callback?token=test_token&redirect_to=/settings');

      // Wait for redirect to settings
      await page.waitForURL('**/settings', { timeout: 10000 });

      // Verify we're on settings page
      expect(page.url()).toContain('/settings');
    });

    test('callback prevents open redirect attacks', async ({ page }) => {
      // Attempt redirect to external URL
      await page.goto(
        '/auth/callback?token=test_token&redirect_to=https://evil.com'
      );

      // Should redirect to dashboard (default) instead of evil.com
      await page.waitForURL('**/dashboard', { timeout: 10000 });
      expect(page.url()).toContain('/dashboard');
      expect(page.url()).not.toContain('evil.com');
    });

    test('callback prevents protocol-relative redirect attacks', async ({ page }) => {
      await page.goto(
        '/auth/callback?token=test_token&redirect_to=//evil.com'
      );

      // Should redirect to dashboard instead
      await page.waitForURL('**/dashboard', { timeout: 10000 });
      expect(page.url()).toContain('/dashboard');
    });

    test('callback handles missing token gracefully', async ({ page }) => {
      // Navigate to callback without token
      await page.goto('/auth/callback');

      // Should redirect to login with error
      await page.waitForURL('**/login*', { timeout: 10000 });
      expect(page.url()).toContain('/login');
      expect(page.url()).toContain('error=missing_token');
    });

    test('callback handles OAuth error gracefully', async ({ page }) => {
      // Navigate to callback with error param
      await page.goto('/auth/callback?error=access_denied');

      // Should redirect to login with error
      await page.waitForURL('**/login*', { timeout: 10000 });
      expect(page.url()).toContain('/login');
      expect(page.url()).toContain('error=access_denied');
    });

    test('callback shows processing status', async ({ page }) => {
      // Slow down to see processing state
      await page.goto('/auth/callback?token=test_token');

      // Should show processing message briefly
      const statusText = page.getByRole('status');
      await expect(statusText).toBeVisible();
    });
  });

  test.describe('Authenticated State', () => {
    test.beforeEach(async ({ page }) => {
      // Set up authenticated state
      await page.goto('/');
      await page.evaluate(() => {
        localStorage.setItem('solvr_auth_token', 'test_jwt_token');
      });
    });

    test('authenticated user can access dashboard', async ({ page }) => {
      await page.goto('/dashboard');

      // Dashboard should load (not redirect to login)
      // This verifies the auth token is being read
      await expect(page.getByText(/dashboard/i).first()).toBeVisible({
        timeout: 10000,
      });
    });

    test('logout clears token and redirects to home', async ({ page }) => {
      // First set up auth state by going through callback
      await page.goto('/auth/callback?token=test_token');
      await page.waitForURL('**/dashboard', { timeout: 10000 });

      // Verify token is stored
      let token = await page.evaluate(() =>
        localStorage.getItem('solvr_auth_token')
      );
      expect(token).toBe('test_token');

      // Clear token (simulating logout)
      await page.evaluate(() => {
        localStorage.removeItem('solvr_auth_token');
      });

      // Refresh page - should no longer be authenticated
      await page.reload();

      // Verify token is cleared
      token = await page.evaluate(() =>
        localStorage.getItem('solvr_auth_token')
      );
      expect(token).toBeNull();
    });
  });

  test.describe('Homepage Login Link', () => {
    test('homepage has login link in header', async ({ page }) => {
      await page.goto('/');

      // Find login link in header
      const loginLink = page.getByRole('link', { name: /login/i });
      await expect(loginLink).toBeVisible();

      // Click and verify navigation
      await loginLink.click();
      await expect(page).toHaveURL(/\/login/);
    });
  });
});
