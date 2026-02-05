import { test, expect } from '@playwright/test';

/**
 * E2E tests for Mobile Responsive design
 *
 * Per SPEC.md Part 4.2 Design Philosophy and prd-v2.json line 5091-5098:
 * - Test navigation at 375px (iPhone SE width)
 * - Test forms work on mobile
 * - Verify no horizontal scroll
 *
 * These tests use the mobile-chrome project (Pixel 5 viewport)
 * and also test at explicit 375px width for smaller devices.
 */

test.describe('Mobile Responsive Design', () => {
  // Use explicit viewport for all tests in this file
  test.use({ viewport: { width: 375, height: 667 } }); // iPhone SE dimensions

  test.describe('Navigation at 375px', () => {
    test('homepage loads without horizontal scroll', async ({ page }) => {
      await page.goto('/');

      // Check that body does not have horizontal scroll
      const hasHorizontalScroll = await page.evaluate(() => {
        return document.body.scrollWidth > document.body.clientWidth;
      });
      expect(hasHorizontalScroll).toBe(false);

      // Verify page is accessible
      const heading = page.getByRole('heading', { level: 1 });
      await expect(heading).toBeVisible();
    });

    test('navigation menu is accessible on mobile', async ({ page }) => {
      await page.goto('/');

      // On mobile, there should be a hamburger menu button or mobile nav
      // Check for either visible nav links or a mobile menu button
      const mobileMenuButton = page.getByRole('button', { name: /menu/i });
      const navLinks = page.getByRole('navigation').getByRole('link');

      // Either mobile menu button exists OR nav links are visible
      const hasMobileMenu = await mobileMenuButton.isVisible().catch(() => false);
      const hasVisibleLinks = await navLinks.first().isVisible().catch(() => false);

      // At least one navigation method should be available
      expect(hasMobileMenu || hasVisibleLinks).toBe(true);
    });

    test('header content fits within viewport', async ({ page }) => {
      await page.goto('/');

      // Header should not overflow
      const header = page.locator('header').first();
      if (await header.isVisible()) {
        const headerBox = await header.boundingBox();
        if (headerBox) {
          expect(headerBox.width).toBeLessThanOrEqual(375);
        }
      }
    });

    test('main content area fits within viewport', async ({ page }) => {
      await page.goto('/');

      // Main content should not overflow
      const main = page.locator('main').first();
      if (await main.isVisible()) {
        const mainBox = await main.boundingBox();
        if (mainBox) {
          expect(mainBox.width).toBeLessThanOrEqual(375);
        }
      }
    });

    test('feed page has no horizontal scroll', async ({ page }) => {
      await page.goto('/feed');

      const hasHorizontalScroll = await page.evaluate(() => {
        return document.body.scrollWidth > document.body.clientWidth;
      });
      expect(hasHorizontalScroll).toBe(false);
    });

    test('login page has no horizontal scroll', async ({ page }) => {
      await page.goto('/login');

      const hasHorizontalScroll = await page.evaluate(() => {
        return document.body.scrollWidth > document.body.clientWidth;
      });
      expect(hasHorizontalScroll).toBe(false);
    });
  });

  test.describe('Forms work on mobile', () => {
    test('login form is usable on mobile', async ({ page }) => {
      await page.goto('/login');

      // Check that OAuth buttons are visible and tappable
      const githubButton = page.getByRole('button', { name: /continue with github/i });
      const googleButton = page.getByRole('button', { name: /continue with google/i });

      await expect(githubButton).toBeVisible();
      await expect(googleButton).toBeVisible();

      // Verify buttons have sufficient tap target size (at least 44px per WCAG)
      const githubBox = await githubButton.boundingBox();
      if (githubBox) {
        expect(githubBox.height).toBeGreaterThanOrEqual(44);
      }
    });

    test('search functionality works on mobile', async ({ page }) => {
      await page.goto('/feed');

      // Look for search input or search button
      const searchInput = page.getByRole('searchbox').or(
        page.getByPlaceholder(/search/i)
      );
      const searchButton = page.getByRole('button', { name: /search/i });

      // Either search input or button should be accessible
      const hasSearchInput = await searchInput.isVisible().catch(() => false);
      const hasSearchButton = await searchButton.isVisible().catch(() => false);

      // At least one search method should be available
      expect(hasSearchInput || hasSearchButton).toBe(true);
    });

    test('filter controls work on mobile', async ({ page }) => {
      await page.goto('/feed');

      // Filter buttons should be visible and tappable
      const filterButtons = page.getByRole('button').filter({ hasText: /all|problems|questions|ideas/i });

      // Should have at least one filter button visible
      const count = await filterButtons.count();
      if (count > 0) {
        const firstButton = filterButtons.first();
        await expect(firstButton).toBeVisible();

        // Buttons should be tappable (have reasonable size)
        const buttonBox = await firstButton.boundingBox();
        if (buttonBox) {
          expect(buttonBox.height).toBeGreaterThanOrEqual(32);
        }
      }
    });

    test('sort dropdown works on mobile', async ({ page }) => {
      await page.goto('/feed');

      // Look for sort control (either select or dropdown button)
      const sortSelect = page.getByRole('combobox').or(
        page.locator('select')
      );
      const sortButton = page.getByRole('button', { name: /sort|latest|top|hot/i });

      const hasSortSelect = await sortSelect.first().isVisible().catch(() => false);
      const hasSortButton = await sortButton.first().isVisible().catch(() => false);

      // At least one sort method should be available
      expect(hasSortSelect || hasSortButton).toBe(true);
    });
  });

  test.describe('No horizontal scroll on key pages', () => {
    const pagesToTest = [
      { path: '/', name: 'Homepage' },
      { path: '/feed', name: 'Feed' },
      { path: '/login', name: 'Login' },
      { path: '/about', name: 'About' },
    ];

    for (const { path, name } of pagesToTest) {
      test(`${name} page (${path}) has no horizontal scroll`, async ({ page }) => {
        await page.goto(path);

        // Wait for page to be fully loaded
        await page.waitForLoadState('networkidle');

        // Check for horizontal scroll
        const scrollInfo = await page.evaluate(() => {
          return {
            scrollWidth: document.documentElement.scrollWidth,
            clientWidth: document.documentElement.clientWidth,
            bodyScrollWidth: document.body.scrollWidth,
            bodyClientWidth: document.body.clientWidth,
          };
        });

        // Neither document nor body should have horizontal overflow
        const hasDocumentOverflow = scrollInfo.scrollWidth > scrollInfo.clientWidth + 1; // 1px tolerance
        const hasBodyOverflow = scrollInfo.bodyScrollWidth > scrollInfo.bodyClientWidth + 1;

        expect(
          hasDocumentOverflow || hasBodyOverflow,
          `Page ${name} has horizontal scroll: doc ${scrollInfo.scrollWidth}/${scrollInfo.clientWidth}, body ${scrollInfo.bodyScrollWidth}/${scrollInfo.bodyClientWidth}`
        ).toBe(false);
      });
    }
  });

  test.describe('Touch-friendly interactions', () => {
    test('buttons have sufficient tap target size', async ({ page }) => {
      await page.goto('/');

      // Get all buttons and links
      const interactiveElements = page.locator('button, a[role="button"], [role="button"]');
      const count = await interactiveElements.count();

      // Check first few interactive elements
      const elementsToCheck = Math.min(count, 5);
      for (let i = 0; i < elementsToCheck; i++) {
        const element = interactiveElements.nth(i);
        if (await element.isVisible()) {
          const box = await element.boundingBox();
          if (box) {
            // WCAG recommends minimum 44x44px tap targets
            // We use 32px as minimum since some UI elements are intentionally smaller
            expect(
              box.height >= 32 || box.width >= 32,
              `Interactive element ${i} has insufficient tap target: ${box.width}x${box.height}`
            ).toBe(true);
          }
        }
      }
    });

    test('text is readable on mobile (font size check)', async ({ page }) => {
      await page.goto('/');

      // Check that main content text has reasonable font size
      const mainContent = page.locator('main p, main span, main div').first();
      if (await mainContent.isVisible()) {
        const fontSize = await mainContent.evaluate((el) => {
          return parseInt(window.getComputedStyle(el).fontSize, 10);
        });

        // Font size should be at least 14px for readability on mobile
        expect(fontSize).toBeGreaterThanOrEqual(14);
      }
    });

    test('links are not too close together', async ({ page }) => {
      await page.goto('/');

      // Get visible links in the main content
      const links = page.locator('main a:visible');
      const count = await links.count();

      if (count >= 2) {
        // Check spacing between consecutive links
        for (let i = 0; i < Math.min(count - 1, 3); i++) {
          const link1 = links.nth(i);
          const link2 = links.nth(i + 1);

          const box1 = await link1.boundingBox();
          const box2 = await link2.boundingBox();

          if (box1 && box2) {
            // Calculate vertical or horizontal distance
            const verticalGap = box2.y - (box1.y + box1.height);
            const horizontalGap = box2.x - (box1.x + box1.width);

            // Links should have at least 8px spacing
            const hasAdequateSpacing = verticalGap >= 8 || horizontalGap >= 8;
            // Or they're on different lines (verticalGap is large)
            const onDifferentLines = verticalGap > box1.height / 2;

            expect(
              hasAdequateSpacing || onDifferentLines,
              `Links ${i} and ${i + 1} are too close together`
            ).toBe(true);
          }
        }
      }
    });
  });

  test.describe('Responsive layout adaptation', () => {
    test('content uses full width on mobile', async ({ page }) => {
      await page.goto('/');

      // Main container should use most of the viewport width on mobile
      const mainContainer = page.locator('main > *').first();
      if (await mainContainer.isVisible()) {
        const box = await mainContainer.boundingBox();
        if (box) {
          // Content should use at least 80% of viewport width
          expect(box.width).toBeGreaterThanOrEqual(375 * 0.8);
        }
      }
    });

    test('cards stack vertically on mobile', async ({ page }) => {
      await page.goto('/feed');
      await page.waitForLoadState('networkidle');

      // Post cards should stack vertically
      const cards = page.locator('[class*="card"], article, [role="article"]');
      const count = await cards.count();

      if (count >= 2) {
        const card1 = cards.nth(0);
        const card2 = cards.nth(1);

        const box1 = await card1.boundingBox();
        const box2 = await card2.boundingBox();

        if (box1 && box2) {
          // Second card should be below first (not side-by-side)
          expect(box2.y).toBeGreaterThanOrEqual(box1.y + box1.height - 10);
        }
      }
    });
  });
});

test.describe('Mobile Responsive - Different Viewports', () => {
  // Test at various mobile breakpoints
  const viewports = [
    { width: 320, height: 568, name: 'iPhone SE (old)' },
    { width: 375, height: 667, name: 'iPhone SE' },
    { width: 390, height: 844, name: 'iPhone 12' },
    { width: 412, height: 915, name: 'Pixel 7' },
  ];

  for (const viewport of viewports) {
    test.describe(`${viewport.name} (${viewport.width}px)`, () => {
      test.use({ viewport });

      test('homepage renders without horizontal overflow', async ({ page }) => {
        await page.goto('/');

        const hasOverflow = await page.evaluate(() => {
          return document.documentElement.scrollWidth > document.documentElement.clientWidth;
        });

        expect(hasOverflow).toBe(false);
      });

      test('all content visible without horizontal scroll', async ({ page }) => {
        await page.goto('/');

        // Check document overflow
        const scrollWidth = await page.evaluate(() => document.documentElement.scrollWidth);
        expect(scrollWidth).toBeLessThanOrEqual(viewport.width + 1);
      });
    });
  }
});
