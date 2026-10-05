import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

import { ANALYTICS_EVENTS } from './analytics';
import { TRACK_PARAM_NAMES } from './analytics-redact';

// SPEC.md 27.7 holds the one table of analytics events: when each is sent, with which
// parameters, and the funnel step it pairs with. The code and the table are kept equal
// here: an event wired without a row, or a row for an event that no longer exists, fails.

const spec = readFileSync(join(__dirname, '..', '..', 'SPEC.md'), 'utf8');
const section = spec.slice(spec.indexOf('## 27.7 Analytics events and consent'));
const tableStart = section.indexOf('| Event | When | Parameters | Funnel step |');
const table = section.slice(tableStart, section.indexOf('\n\n', tableStart));

const rows = table
  .split('\n')
  .slice(2)
  .map((line) => line.split('|').slice(1, -1).map((cell) => cell.trim()))
  .map(([event, when, parameters, funnel]) => ({ event: event.replace(/`/g, ''), when, parameters, funnel }));

const namesIn = (cell: string) => [...cell.matchAll(/`([a-z_]+)`/g)].map((m) => m[1]);

describe('SPEC.md 27.7, the event table', () => {
  it('is there, with a row per event', () => {
    expect(tableStart).toBeGreaterThan(-1);
    expect(rows.length).toBeGreaterThan(25);
  });

  it('lists every event the code can send, in the code\'s order, and page_view, which the tag sends itself', () => {
    expect(rows.map((row) => row.event)).toEqual(['page_view', ...ANALYTICS_EVENTS]);
  });

  it('says when each event is sent', () => {
    for (const row of rows) expect(row.when.length, row.event).toBeGreaterThan(15);
  });

  it('names only parameters from the closed set (and content_group, which every event carries)', () => {
    for (const row of rows) {
      // The first backticked word of each "name: values" group is a parameter name.
      const parameters = row.parameters === '' ? [] : row.parameters.split(';').map((group) => namesIn(group)[0]);
      for (const name of parameters) {
        expect([...TRACK_PARAM_NAMES, 'content_group'], `${row.event}: ${name}`).toContain(name);
      }
    }
  });

  it('pairs exactly the events lib/funnel.ts sends with a funnel step', () => {
    const paired = Object.fromEntries(rows.filter((row) => row.funnel.startsWith('`')).map((row) => [row.event, namesIn(row.funnel)[0]]));
    expect(paired).toEqual({
      connect_start: 'connection_started',
      prompt_copy: 'starter_prompt_copied',
      join_prompt_copy: 'join_prompt_copied',
      room_share: 'share_link_copied',
      outcome_copy: 'share_link_copied',
      share_visit: 'share_visit',
    });
  });
});

describe('SPEC.md 27.7, what the owner sets in the Google Analytics property', () => {
  const start = section.indexOf('**For the Google Analytics property (owner\'s action).**');
  const admin = section.slice(start, section.indexOf('\n---', start));

  it('is the last part of 27.7', () => {
    expect(start).toBeGreaterThan(tableStart);
    expect(start).toBeGreaterThan(section.indexOf('**`/privacy`**'));
    // Nothing else of the section follows it.
    expect(admin.slice(4)).not.toMatch(/\n\*\*|\n#/);
  });

  it('names the four key events', () => {
    const line = admin.split('\n').find((l) => /key events/i.test(l)) ?? '';
    expect(namesIn(admin.slice(admin.indexOf(line), admin.indexOf('custom dimensions')))).toEqual(
      expect.arrayContaining(['sign_up', 'prompt_copy', 'room_create', 'agent_claim']),
    );
    for (const event of ['sign_up', 'prompt_copy', 'room_create', 'agent_claim']) {
      expect(ANALYTICS_EVENTS as readonly string[]).toContain(event);
    }
  });

  it('lists every parameter of the closed set as a custom dimension to register, except search_term', () => {
    const dimensions = admin.slice(admin.indexOf('custom dimensions'));
    const listed = namesIn(dimensions.slice(0, dimensions.indexOf('\n\n') === -1 ? undefined : dimensions.indexOf('\n\n')));
    for (const name of TRACK_PARAM_NAMES.filter((n) => n !== 'search_term')) {
      expect(listed, name).toContain(name);
    }
    expect(admin).toMatch(/`search_term`[^.]*already/);
  });
});
