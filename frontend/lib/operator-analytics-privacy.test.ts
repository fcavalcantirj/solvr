import { describe, it, expect } from 'vitest';
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join } from 'node:path';

// The browser collects; it never reads the reports.
//
// Solvr's operator analytics — traffic, acquisition, retention, cost, raw
// diagnostics, growth planning — lives behind the server's own gate, proven in
// backend/internal/api/router_operator_analytics_test.go. This file holds the
// other half of the line: the frontend is code a visitor downloads, so
// anything it carries is public whether or not a page draws it.
//
// The distinction that matters here is COLLECTION versus REPORTING. The gtag
// measurement tag in app/layout.tsx and the sendGAEvent calls in lib/analytics
// are the collection side: they run in every visitor's browser by design, and
// a measurement id is not a secret. Reading what was collected is the other
// side — it takes a credential, a reporting property and an export, and none
// of those may exist in code that ships to a browser.
//
// The patterns below deliberately mirror the Go list in
// backend/internal/api/handlers/operator_analytics.go rather than importing
// it: a frontend test that imported the rule from the thing it is checking
// would prove nothing.

const SOURCE_ROOTS = ['app', 'components', 'lib', 'hooks'];
const SOURCE_EXTENSIONS = ['.ts', '.tsx', '.js', '.jsx', '.mjs'];

// Credentials, reporting properties and exports. Never the collection tag.
const OPERATOR_MATERIAL: Array<[string, RegExp]> = [
  ['a Google Analytics reporting property', /\bga[ _-]?4?[ _-]?property[ _-]?id\b/i],
  ['a reporting property resource', /\bproperties\/\d+/i],
  ['the Analytics Data API', /\banalyticsdata\b/i],
  ['Search Console', /\bsearch[ _-]?console\b/i],
  ['the webmasters API', /\bwebmasters?\b/i],
  // A REPORTING credential, not any credential: Solvr issues private keys to
  // agents and refresh tokens to people, and those belong in the browser.
  ['a service account', /\bservice[ _-]?account\b/i],
  ['a client secret', /\bclient[ _-]?secret\b/i],
  ['an API secret', /\bapi[ _-]?secret\b/i],
  ['application default credentials', /\bgoogle[ _-]?application[ _-]?credentials\b/i],
  ['a key pasted into the source', /-----BEGIN [A-Z ]*PRIVATE KEY-----/],
  ['an analytics export', /\b(analytics|traffic|audience|ga4|search[ _-]?console)[ _-]?exports?\b/i],
  ['a warehouse copy', /\bbigquery\b/i],
];

// The operator's own credential. If it ever appeared in frontend code it would
// be downloadable by anyone who opened the page.
const OPERATOR_CREDENTIAL: Array<[string, RegExp]> = [
  ['the operator key header', /X-Admin-API-Key/i],
  ['the operator key itself', /ADMIN_API_KEY/],
];

// The operator report paths, mirrored from the Go registry.
const OPERATOR_REPORT_PATHS = [
  '/admin/search-analytics/trending',
  '/admin/search-analytics/summary',
  '/admin/email/history',
  '/admin/users/deleted',
  '/admin/agents/deleted',
  '/admin/query',
];

/** shippedSources lists every source file a build turns into browser code. */
function shippedSources(): string[] {
  const files: string[] = [];

  const walk = (dir: string) => {
    let entries: string[];
    try {
      entries = readdirSync(dir);
    } catch {
      return;
    }
    for (const entry of entries) {
      const full = join(dir, entry);
      if (statSync(full).isDirectory()) {
        if (entry === 'node_modules' || entry === '.next') continue;
        walk(full);
        continue;
      }
      // A test never ships. Everything else does.
      if (/\.test\.(ts|tsx|js|jsx)$/.test(entry)) continue;
      if (SOURCE_EXTENSIONS.some((ext) => entry.endsWith(ext))) files.push(full);
    }
  };

  for (const root of SOURCE_ROOTS) walk(root);
  return files;
}

describe('operator analytics never reach the browser', () => {
  const sources = shippedSources();

  it('finds the frontend source to check', () => {
    expect(sources.length).toBeGreaterThan(50);
  });

  it('ships no Google Analytics or Search Console credential, property or export', () => {
    const offenders: string[] = [];
    for (const file of sources) {
      const contents = readFileSync(file, 'utf8');
      for (const [concept, pattern] of OPERATOR_MATERIAL) {
        if (pattern.test(contents)) offenders.push(`${file} carries ${concept}`);
      }
    }
    expect(offenders).toEqual([]);
  });

  it('ships no operator credential', () => {
    const offenders: string[] = [];
    for (const file of sources) {
      const contents = readFileSync(file, 'utf8');
      for (const [concept, pattern] of OPERATOR_CREDENTIAL) {
        if (pattern.test(contents)) offenders.push(`${file} carries ${concept}`);
      }
    }
    expect(offenders).toEqual([]);
  });

  it('calls no operator report endpoint', () => {
    const offenders: string[] = [];
    for (const file of sources) {
      const contents = readFileSync(file, 'utf8');
      for (const path of OPERATOR_REPORT_PATHS) {
        if (contents.includes(path)) offenders.push(`${file} calls ${path}`);
      }
    }
    expect(offenders).toEqual([]);
  });

  it('collects measurements without ever reading a report', () => {
    const analytics = readFileSync(join('lib', 'analytics.ts'), 'utf8');

    // Collection is the whole job: events go out, nothing comes back.
    expect(analytics).toContain('sendGAEvent');
    expect(analytics).not.toMatch(/fetch\s*\(/);
    expect(analytics).not.toMatch(/googleapis\.com/i);
  });

  it('serves no analytics or traffic export as a static asset', () => {
    const offenders: string[] = [];

    const walk = (dir: string) => {
      for (const entry of readdirSync(dir)) {
        const full = join(dir, entry);
        if (statSync(full).isDirectory()) {
          walk(full);
          continue;
        }
        if (/(analytics|traffic|audience|ga4|search[ _-]?console|visitors|sessions)/i.test(entry)) {
          offenders.push(full);
        }
        if (/\.(csv|tsv)$/i.test(entry)) offenders.push(full);
      }
    };
    walk('public');

    expect(offenders).toEqual([]);
  });
});
