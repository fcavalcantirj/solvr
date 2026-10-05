import { describe, it, expect, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join, relative, sep } from 'node:path';

// /privacy, "Cookies & Tracking": the page says what the code does, and only that
// (SPEC.md 27.7). Google Analytics after Accept, Cloudflare Web Analytics, Solvr's own
// counts, and every key the site keeps in the browser. The last part is checked against
// the source: a new storage key fails here until the page lists it.

vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

import { BROWSER_STORAGE, PrivacyTrackingSection } from './privacy-tracking-section';
import { onConsentSettingsRequest } from '@/lib/consent';

const ROOT = join(__dirname, '..', '..');
const squish = (s: string | null | undefined) => (s ?? '').replace(/\s+/g, ' ').trim();

function pageText(): string {
  const { container } = render(<PrivacyTrackingSection />);
  return squish(container.textContent);
}

describe('PrivacyTrackingSection', () => {
  it('keeps the section the table of contents links to', () => {
    const { container } = render(<PrivacyTrackingSection />);
    expect(container.querySelector('section#cookies')).not.toBeNull();
    expect(screen.getByRole('heading', { level: 2, name: /cookies & tracking/i })).toBeInTheDocument();
    expect(readFileSync(join(ROOT, 'app', 'privacy', 'page.tsx'), 'utf8')).toContain('{ id: "cookies", title: "Cookies & Tracking" }');
  });

  it('says Solvr itself sets no cookie', () => {
    expect(pageText()).toContain('Solvr itself sets no cookie');
  });

  it('says Google Analytics loads only after Accept, and what happens without it', () => {
    const text = pageText();
    expect(text).toContain('Google Analytics');
    expect(text).toMatch(/Unless you press Accept, nothing is sent to Google and Google sets no cookie/);
    expect(text).toContain('_ga');
  });

  it('says signals and ad personalisation are off', () => {
    expect(pageText()).toMatch(/Google signals and ad personalisation are off/);
  });

  it('says how to change the choice, and offers the control right there', () => {
    const asked = vi.fn();
    const stop = onConsentSettingsRequest(asked);
    const { container } = render(<PrivacyTrackingSection />);

    expect(squish(container.textContent)).toMatch(/Cookie settings/);
    within(container).getByRole('button', { name: 'Cookie settings' }).click();
    expect(asked).toHaveBeenCalledTimes(1);
    stop();
  });

  it('says what Decline does', () => {
    expect(pageText()).toMatch(/After Decline, nothing you do is sent and Google's cookies are deleted from this browser/);
  });

  it('says Global Privacy Control is honoured', () => {
    expect(pageText()).toMatch(/Global Privacy Control is treated as Decline/);
  });

  it('names Cloudflare Web Analytics and Solvr’s own counts', () => {
    const text = pageText();
    expect(text).toContain('Cloudflare Web Analytics');
    expect(text).toMatch(/sets no cookie/);
    expect(text).toMatch(/Solvr’s own counts|Solvr's own counts/);
    expect(text).toMatch(/No IP address and no user agent is stored with a view or a step/);
  });

  it('does not hide that a search is logged with the IP address and the user agent', () => {
    expect(pageText()).toMatch(/A search is logged with its words, the number of results, the IP address and the user agent/);
  });

  it('no longer says what the code never did', () => {
    const text = pageText();
    for (const gone of [
      'privacy-focused',
      'Essential',
      'Functional',
      'Session / 1 year',
      'Remember your settings and preferences',
      'disabling essential cookies',
      'We use cookies and similar technologies to provide and improve our services',
    ]) {
      expect(text).not.toContain(gone);
    }
  });
});

describe('the Analytics purpose on /privacy', () => {
  const page = readFileSync(join(ROOT, 'app', 'privacy', 'page.tsx'), 'utf8');

  it('rests Google Analytics on consent, not on legitimate interest alone', () => {
    const start = page.indexOf('purpose: "Analytics"');
    expect(start).toBeGreaterThan(-1);
    const row = page.slice(start, page.indexOf('},', start));
    expect(row).toMatch(/Google Analytics only if you accept it/);
    expect(row).toMatch(/legal: "Consent \/ Legitimate interest"/);
  });
});

// Every key the site keeps in localStorage or sessionStorage is listed on the page. The
// scan resolves each key from the source: a literal, a template (its fixed prefix), or a
// constant defined in the same file.
describe('browser storage keys', () => {
  const sources: string[] = [];
  const walk = (dir: string) => {
    for (const entry of readdirSync(dir)) {
      const full = join(dir, entry);
      if (statSync(full).isDirectory()) {
        if (entry !== 'node_modules' && !entry.startsWith('.')) walk(full);
      } else if (/\.(ts|tsx)$/.test(entry) && !/\.test\.(ts|tsx)$/.test(entry)) {
        sources.push(full);
      }
    }
  };
  for (const dir of ['app', 'components', 'lib', 'hooks']) walk(join(ROOT, dir));

  const USE = /\b(localStorage|sessionStorage)\s*\.\s*(?:getItem|setItem|removeItem)\(\s*([^,)\s]+)/g;

  function literal(token: string): string | null {
    const quoted = /^(['"])(.*)\1$/.exec(token);
    if (quoted) return quoted[2];
    const template = /^`([^`$]*)/.exec(token);
    if (template) return template[1];
    return null;
  }

  type Found = { where: string; store: string; key: string | null; token: string };
  const found: Found[] = [];
  for (const file of sources) {
    const src = readFileSync(file, 'utf8');
    for (const match of src.matchAll(USE)) {
      const token = match[2];
      let key = literal(token);
      if (key === null && /^\w+$/.test(token)) {
        const constant = new RegExp(`\\bconst\\s+${token}\\s*=\\s*([^;\\n]+)`).exec(src);
        if (constant) key = literal(constant[1].trim());
      }
      found.push({ where: relative(ROOT, file).split(sep).join('/'), store: match[1], key, token });
    }
  }

  it('finds the keys it is meant to be checking', () => {
    expect(found.length).toBeGreaterThan(20);
    expect(found.filter((f) => f.key === null).map((f) => `${f.where}: ${f.token}`)).toEqual([]);
  });

  it('lists every key the code uses, under the storage it is in', () => {
    const missing = found
      .filter((f) => !BROWSER_STORAGE.some((entry) => entry.store === f.store && (f.key ?? '').startsWith(entry.key.replace(/<.*$/, ''))))
      .map((f) => `${f.where}: ${f.store} ${f.key}`);
    expect(missing).toEqual([]);
  });

  it('lists no key the code no longer uses', () => {
    const stale = BROWSER_STORAGE.filter(
      (entry) => !found.some((f) => f.store === entry.store && (f.key ?? '').startsWith(entry.key.replace(/<.*$/, ''))),
    ).map((entry) => entry.key);
    expect(stale).toEqual([]);
  });

  it('shows each key on the page, with what it is for', () => {
    const { container } = render(<PrivacyTrackingSection />);
    const rows = Array.from(container.querySelectorAll('tbody tr'));
    expect(rows).toHaveLength(BROWSER_STORAGE.length);
    for (const entry of BROWSER_STORAGE) {
      const row = rows.find((r) => squish(r.querySelector('td')?.textContent) === entry.key);
      expect(row, entry.key).toBeDefined();
      expect(squish(row?.textContent)).toContain(entry.purpose);
      expect(entry.purpose.length).toBeGreaterThan(20);
    }
    expect(BROWSER_STORAGE.map((e) => e.key)).toEqual(
      expect.arrayContaining(['auth_token', 'auth_return_url', 'solvr_referral_code', 'solvr:recently-viewed-rooms', 'solvr_consent', 'solvr_session_id', 'solvr_viewed_posts', 'solvr_pending_claim_token', 'solvr_pending_events']),
    );
  });
});
