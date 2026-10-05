import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { act, fireEvent, render, screen, within } from '@testing-library/react';
import { renderToString } from 'react-dom/server';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

// The consent bar asks once, for everyone (SPEC.md 27.7): a non-modal region fixed to the
// bottom of the window, one sentence, a link to /privacy and two equal buttons. It shows only
// while nothing was chosen, only in the browser, never on an untracked page; "Cookie
// settings" brings it back with the current choice so it can be changed.

const mockPathname = vi.fn<() => string | null>();
vi.mock('next/navigation', () => ({
  usePathname: () => mockPathname(),
}));

vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

import { ConsentBar } from './consent-bar';
import { CookieSettingsButton } from './cookie-settings-button';
import { getConsent, openConsentSettings, setConsent } from '@/lib/consent';

const SENTENCE =
  'Solvr would like to use Google Analytics to learn which pages help. Nothing goes to Google unless you accept.';

function setGpc(value: unknown) {
  Object.defineProperty(window.navigator, 'globalPrivacyControl', { value, configurable: true });
}

const bar = () => screen.queryByRole('region', { name: /analytics consent/i });

describe('ConsentBar', () => {
  beforeEach(() => {
    window.localStorage.clear();
    setGpc(undefined);
    mockPathname.mockReturnValue('/');
  });

  afterEach(() => {
    setGpc(undefined);
  });

  describe('while nothing was chosen', () => {
    it('shows the sentence, exactly, followed by a Privacy link', () => {
      render(<ConsentBar />);
      const region = bar() as HTMLElement;
      expect(region).toBeInTheDocument();
      expect(within(region).getByText(SENTENCE, { exact: false })).toBeInTheDocument();
      expect(region.textContent).toContain(`${SENTENCE} Privacy`);
      expect(within(region).getByRole('link', { name: 'Privacy' })).toHaveAttribute('href', '/privacy');
    });

    it('offers Decline and Accept, in that order, as the same control', () => {
      render(<ConsentBar />);
      const buttons = within(bar() as HTMLElement).getAllByRole('button');
      expect(buttons.map((b) => b.textContent)).toEqual(['Decline', 'Accept']);
      // Equal size and style: one class list, and a two-column grid gives both the same width.
      expect(buttons[0].className).toBe(buttons[1].className);
      expect(buttons[0].parentElement?.className).toMatch(/\bgrid\b/);
      expect(buttons[0].parentElement?.className).toMatch(/\bgrid-cols-2\b/);
      for (const button of buttons) {
        expect(button).toHaveAttribute('type', 'button');
        expect(button.className).toMatch(/\bborder\b/);
        expect(button.className).toMatch(/\bfont-mono\b/);
        expect(button.className).toMatch(/\buppercase\b/);
        expect(button.className).toMatch(/focus-visible:outline/);
      }
    });

    it('is a region fixed to the bottom, never a dialog: no modal, no backdrop, no focus taken', () => {
      render(
        <>
          <button>page control</button>
          <ConsentBar />
        </>,
      );
      const region = bar() as HTMLElement;
      expect(region.tagName).toBe('SECTION');
      expect(screen.queryByRole('dialog')).toBeNull();
      expect(screen.queryByRole('alertdialog')).toBeNull();
      expect(region).not.toHaveAttribute('aria-modal');
      expect(region.className).toMatch(/\bfixed\b/);
      expect(region.className).toMatch(/\bbottom-0\b/);
      expect(region.className).toMatch(/\binset-x-0\b/);
      // It covers a strip at the bottom and nothing else: no full-window layer comes with it.
      expect(region.className).not.toMatch(/\binset-0\b|\btop-0\b|h-screen|min-h-screen/);
      expect(document.querySelector('[data-state="open"], .backdrop, [class*="bg-black"]')).toBeNull();
      // The page keeps the focus it had.
      expect(document.activeElement).toBe(document.body);
      // The rest of the page stays operable.
      expect(document.body.style.pointerEvents).not.toBe('none');
      expect(document.body.style.overflow).not.toBe('hidden');
    });

    it("follows the site's look: hairline top border, the page's background, square, flat", () => {
      render(<ConsentBar />);
      const region = bar() as HTMLElement;
      expect(region.className).toMatch(/\bborder-t\b/);
      expect(region.className).toMatch(/\bborder-border\b/);
      expect(region.className).toMatch(/\bbg-background\b/);
      expect(region.outerHTML).not.toMatch(/\brounded|\bshadow/);
    });

    it('reaches Privacy, Decline and Accept with the keyboard, in that order', () => {
      render(<ConsentBar />);
      const focusable = Array.from((bar() as HTMLElement).querySelectorAll<HTMLElement>('a[href], button'));
      expect(focusable.map((el) => el.textContent)).toEqual(['Privacy', 'Decline', 'Accept']);
      for (const el of focusable) {
        expect(el).not.toHaveAttribute('tabindex', '-1');
        expect(el).not.toBeDisabled();
      }
    });

    it('stores granted on Accept and goes away', () => {
      render(<ConsentBar />);
      fireEvent.click(screen.getByRole('button', { name: 'Accept' }));
      expect(getConsent()).toBe('granted');
      expect(bar()).toBeNull();
    });

    it('stores denied on Decline and goes away', () => {
      render(<ConsentBar />);
      fireEvent.click(screen.getByRole('button', { name: 'Decline' }));
      expect(getConsent()).toBe('denied');
      expect(bar()).toBeNull();
    });

    it('ignores Escape: it stays until the visitor chooses', () => {
      render(<ConsentBar />);
      fireEvent.keyDown(screen.getByRole('button', { name: 'Accept' }), { key: 'Escape' });
      expect(bar()).toBeInTheDocument();
      expect(getConsent()).toBe('unset');
    });

    it('goes away when the choice is made in another tab', () => {
      render(<ConsentBar />);
      act(() => {
        window.localStorage.setItem('solvr_consent', JSON.stringify({ analytics: 'denied', at: '2026-10-05T00:00:00.000Z', v: 1 }));
        window.dispatchEvent(new StorageEvent('storage', { key: 'solvr_consent' }));
      });
      expect(bar()).toBeNull();
    });
  });

  // Measured on the home page at 1280x800: the full-width bar covered the hero's primary call
  // to action. From the lg breakpoint up it is a compact card in the bottom right corner;
  // below lg it stays the full-width bar. Class names are what a unit test can see: the
  // boxes themselves are measured in a real browser (the batch's check).
  describe('from the lg breakpoint up: a compact card in the bottom right corner', () => {
    const classes = (el: Element | null | undefined) => (el?.className ?? '').split(/\s+/);

    it('leaves the left edge and sits 1.5rem from the right and the bottom, about 26rem wide', () => {
      render(<ConsentBar />);
      const region = classes(bar());
      expect(region).toEqual(expect.arrayContaining(['lg:left-auto', 'lg:right-6', 'lg:bottom-6', 'lg:w-[26rem]']));
      // Below lg nothing changed: the same full-width strip on the bottom edge.
      expect(region).toEqual(expect.arrayContaining(['fixed', 'inset-x-0', 'bottom-0', 'border-t', 'border-border']));
      // It stays a corner card: never centred, never full height, at any width.
      expect(region.join(' ')).not.toMatch(/(^|\s)(\w+:)?(inset-0|top-0|inset-y-0|h-screen|min-h-screen|left-1\/2|-translate-)/);
    });

    it('has a 1px border in the foreground colour on every side, the page background, no radius, no shadow', () => {
      render(<ConsentBar />);
      const region = bar() as HTMLElement;
      expect(classes(region)).toEqual(expect.arrayContaining(['lg:border', 'lg:border-foreground', 'bg-background']));
      expect(region.outerHTML).not.toMatch(/\brounded|\bshadow/);
    });

    it('stacks the sentence and the Privacy link over the two equal buttons, side by side', () => {
      render(<ConsentBar />);
      const region = bar() as HTMLElement;
      const row = region.firstElementChild as HTMLElement;
      // A row between md and lg, a column again from lg up.
      expect(classes(row)).toEqual(expect.arrayContaining(['flex', 'flex-col', 'md:flex-row', 'lg:flex-col', 'lg:items-stretch']));
      const [text, buttons] = Array.from(row.children);
      expect(text.textContent).toContain(`${SENTENCE} Privacy`);
      expect(classes(buttons)).toEqual(expect.arrayContaining(['grid', 'grid-cols-2', 'lg:w-full']));
      expect(Array.from(buttons.children).map((b) => b.textContent)).toEqual(['Decline', 'Accept']);
      // Still one class list for both: the card did not make either choice the bigger one.
      expect(buttons.children[0].className).toBe(buttons.children[1].className);
    });

    it('takes no room in the page at any width: it is fixed, and nothing reserves space for it', () => {
      const { container } = render(
        <>
          <main>page</main>
          <ConsentBar />
        </>,
      );
      expect(classes(bar())).toContain('fixed');
      expect(container.querySelector('main')?.className ?? '').toBe('');
      expect(document.body.style.paddingBottom).toBe('');
    });

    // Measured on /login and /join (1024x768 to 1440x900): their form fills the right half
    // from lg up, and a card in the right corner covered SIGN IN and CREATE ACCOUNT. In the
    // left corner it covers the brand panel's text and no control.
    it.each(['/login', '/join', '/login/', '/join/'])('takes the bottom LEFT corner on %s, where the form is on the right', (path) => {
      mockPathname.mockReturnValue(path);
      render(<ConsentBar />);
      const region = classes(bar());
      expect(region).toEqual(expect.arrayContaining(['lg:left-6', 'lg:right-auto', 'lg:bottom-6', 'lg:w-[26rem]', 'lg:border', 'lg:border-foreground']));
      expect(region).not.toContain('lg:right-6');
      expect(region).not.toContain('lg:left-auto');
      // Below lg it is the same strip as everywhere.
      expect(region).toEqual(expect.arrayContaining(['fixed', 'inset-x-0', 'bottom-0', 'border-t']));
    });

    it.each(['/', '/connect', '/rooms', '/posts/abc', '/skill', '/login-help', '/joined', '/settings'])(
      'keeps the bottom right corner on %s',
      (path) => {
        mockPathname.mockReturnValue(path);
        render(<ConsentBar />);
        const region = classes(bar());
        expect(region).toEqual(expect.arrayContaining(['lg:left-auto', 'lg:right-6']));
        expect(region).not.toContain('lg:left-6');
        expect(region).not.toContain('lg:right-auto');
      },
    );

    it('opens in the left corner too when Cookie settings is pressed on a sign-in page', () => {
      setConsent('denied');
      mockPathname.mockReturnValue('/login');
      render(<ConsentBar />);
      act(() => openConsentSettings());
      expect(classes(bar())).toEqual(expect.arrayContaining(['lg:left-6', 'lg:right-auto']));
    });

    it('lays out the reopened bar the same way, with its status line inside the card', () => {
      setConsent('granted');
      render(<ConsentBar />);
      act(() => openConsentSettings());
      const region = bar() as HTMLElement;
      expect(classes(region)).toEqual(
        expect.arrayContaining(['lg:left-auto', 'lg:right-6', 'lg:bottom-6', 'lg:w-[26rem]', 'lg:border', 'lg:border-foreground']),
      );
      const row = region.firstElementChild as HTMLElement;
      expect(classes(row)).toContain('lg:flex-col');
      expect(within(row.children[0] as HTMLElement).getByText('Analytics is on.')).toBeInTheDocument();
      expect(classes(row.children[1])).toContain('lg:w-full');
    });
  });

  describe('when it does not show', () => {
    it.each(['granted', 'denied'] as const)('stays away once the choice is %s', (choice) => {
      setConsent(choice);
      render(<ConsentBar />);
      expect(bar()).toBeNull();
    });

    it('stays away from a browser that sends Global Privacy Control, and stores nothing for it', () => {
      setGpc(true);
      render(<ConsentBar />);
      expect(bar()).toBeNull();
      expect(getConsent()).toBe('denied');
      expect(window.localStorage.getItem('solvr_consent')).toBeNull();
    });

    it.each(['/claim', '/claim/', '/auth/callback', '/email/unsubscribe'])('stays away on the untracked path %s', (path) => {
      mockPathname.mockReturnValue(path);
      render(<ConsentBar />);
      expect(bar()).toBeNull();
    });

    it('stays away while the page is unknown', () => {
      mockPathname.mockReturnValue(null);
      render(<ConsentBar />);
      expect(bar()).toBeNull();
    });

    it('is not in the server HTML: pages are shared between visitors', () => {
      expect(renderToString(<ConsentBar />)).toBe('');
    });

    it('appears only after hydration', () => {
      const container = document.createElement('div');
      document.body.appendChild(container);
      container.innerHTML = renderToString(<ConsentBar />);
      expect(container.innerHTML).toBe('');

      render(<ConsentBar />, { container, hydrate: true });
      expect(bar()).toBeInTheDocument();
      container.remove();
    });
  });

  describe('Cookie settings', () => {
    it('brings the bar back after Accept, saying "Analytics is on."', () => {
      setConsent('granted');
      render(<ConsentBar />);
      expect(bar()).toBeNull();

      act(() => openConsentSettings());
      const region = bar() as HTMLElement;
      expect(within(region).getByText('Analytics is on.')).toBeInTheDocument();
      expect(within(region).getByText(SENTENCE, { exact: false })).toBeInTheDocument();
      expect(within(region).getAllByRole('button').map((b) => b.textContent)).toEqual(['Decline', 'Accept']);
    });

    it('brings the bar back after Decline, saying "Analytics is off."', () => {
      setConsent('denied');
      render(<ConsentBar />);
      act(() => openConsentSettings());
      expect(within(bar() as HTMLElement).getByText('Analytics is off.')).toBeInTheDocument();
    });

    it('says "Analytics is off." to a browser that sends Global Privacy Control, and lets it accept', () => {
      setGpc(true);
      render(<ConsentBar />);
      act(() => openConsentSettings());
      expect(within(bar() as HTMLElement).getByText('Analytics is off.')).toBeInTheDocument();

      fireEvent.click(screen.getByRole('button', { name: 'Accept' }));
      // A stored choice always wins over the signal.
      expect(getConsent()).toBe('granted');
      expect(bar()).toBeNull();
    });

    it('changes the choice: Decline after Accept', () => {
      setConsent('granted');
      render(<ConsentBar />);
      act(() => openConsentSettings());
      fireEvent.click(screen.getByRole('button', { name: 'Decline' }));
      expect(getConsent()).toBe('denied');
      expect(bar()).toBeNull();
    });

    it('closes when the same choice is confirmed', () => {
      setConsent('granted');
      render(<ConsentBar />);
      act(() => openConsentSettings());
      fireEvent.click(screen.getByRole('button', { name: 'Accept' }));
      expect(getConsent()).toBe('granted');
      expect(bar()).toBeNull();
    });

    it('says no choice while nothing was chosen, and opens on an untracked page too: the visitor asked', () => {
      mockPathname.mockReturnValue('/claim');
      render(<ConsentBar />);
      expect(bar()).toBeNull();

      act(() => openConsentSettings());
      const region = bar() as HTMLElement;
      expect(region).toBeInTheDocument();
      expect(within(region).queryByText(/Analytics is/)).toBeNull();
    });

    it('moves focus to the bar when asked for, and back to the control that asked', () => {
      setConsent('granted');
      render(
        <>
          <CookieSettingsButton />
          <ConsentBar />
        </>,
      );
      const opener = screen.getByRole('button', { name: 'Cookie settings' });
      opener.focus();
      fireEvent.click(opener);

      const region = bar() as HTMLElement;
      expect(document.activeElement).toBe(region);

      fireEvent.click(screen.getByRole('button', { name: 'Decline' }));
      expect(bar()).toBeNull();
      expect(document.activeElement).toBe(opener);
    });

    it('closes on Escape without changing the choice', () => {
      setConsent('granted');
      render(<ConsentBar />);
      act(() => openConsentSettings());
      fireEvent.keyDown(bar() as HTMLElement, { key: 'Escape' });
      expect(bar()).toBeNull();
      expect(getConsent()).toBe('granted');
    });
  });
});

describe('CookieSettingsButton', () => {
  beforeEach(() => {
    window.localStorage.clear();
    mockPathname.mockReturnValue('/');
  });

  it('is a real button named "Cookie settings" that wears the classes it is given', () => {
    render(<CookieSettingsButton className="font-mono text-xs" />);
    const button = screen.getByRole('button', { name: 'Cookie settings' });
    expect(button).toHaveAttribute('type', 'button');
    expect(button.className).toBe('font-mono text-xs');
  });

  it('opens the bar, after telling its own menu to close', () => {
    setConsent('denied');
    const calls: string[] = [];
    const onOpen = vi.fn(() => calls.push(bar() ? 'bar already open' : 'menu closes first'));
    render(
      <>
        <CookieSettingsButton onOpen={onOpen} />
        <ConsentBar />
      </>,
    );
    fireEvent.click(screen.getByRole('button', { name: 'Cookie settings' }));
    expect(calls).toEqual(['menu closes first']);
    expect(bar()).toBeInTheDocument();
  });

  it('can carry an icon beside its words', () => {
    render(
      <CookieSettingsButton>
        <svg data-testid="icon" />
        Cookie settings
      </CookieSettingsButton>,
    );
    const button = screen.getByRole('button', { name: 'Cookie settings' });
    expect(within(button).getByTestId('icon')).toBeInTheDocument();
  });
});

describe('the bar is mounted once, for every page', () => {
  const layout = readFileSync(join(__dirname, '..', 'app', 'layout.tsx'), 'utf8');

  it('sits in the root layout, inside <body>, after the page', () => {
    expect(layout.match(/<ConsentBar\b/g)).toHaveLength(1);
    const body = layout.slice(layout.indexOf('<body'), layout.indexOf('</body>'));
    expect(body).toMatch(/<Providers>\{children\}<\/Providers>[\s\S]*<ConsentBar \/>/);
  });
});
