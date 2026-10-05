import { describe, it, expect } from 'vitest';
import { existsSync } from 'node:fs';
import { join } from 'node:path';
import {
  GUIDE_GROUPS,
  USE_CASE_GUIDES,
  WORKFLOW_GUIDES,
  guideForPreset,
  guidePath,
  guideLinks,
  guideTexts,
  workflowGuide,
} from './workflow-guides';
import { AGENTS, AGENT_VERSIONS, EXAMPLE_ROOMS, NOT_RUN_REASON, RUNS, runAgents } from './guide-runs';
import { CONNECT_EXAMPLES } from '@/components/connect/connect-fixture';

// SPEC.md 27.5 (owner, 2026-10-05): nine guides, grouped by use case and by agent. A guide
// names the agents that were really run for it and every one that was not; it quotes only
// what the runs recorded; the sentence it shows is always the API's, never typed here.

const AGENT_GUIDE_TOOL: Record<string, string> = {
  'claude-code': 'Claude Code',
  codex: 'Codex',
  'kimi-code': 'Kimi Code',
  hermes: 'Hermes',
  openclaw: 'OpenClaw',
};

describe('the guides', () => {
  it('are nine: three use cases and the resume guide, then one per agent', () => {
    expect(WORKFLOW_GUIDES.map((g) => [g.slug, g.kind])).toEqual([
      ['connect-planner-executor', 'use-case'],
      ['share-context-between-agents', 'use-case'],
      ['connect-builder-reviewer', 'use-case'],
      ['resume-across-two-clis', 'resume'],
      ['claude-code', 'agent'],
      ['codex', 'agent'],
      ['kimi-code', 'agent'],
      ['hermes', 'agent'],
      ['openclaw', 'agent'],
    ]);
    expect(new Set(WORKFLOW_GUIDES.map((g) => g.title)).size).toBe(9);
  });

  it('are listed in two groups, by use case and by agent, each guide once', () => {
    expect(GUIDE_GROUPS.map((group) => group.heading)).toEqual(['By use case', 'By agent']);
    expect(GUIDE_GROUPS.flatMap((group) => group.guides.map((g) => g.slug))).toEqual(WORKFLOW_GUIDES.map((g) => g.slug));
    expect(GUIDE_GROUPS[1].guides.every((g) => g.kind === 'agent')).toBe(true);
  });

  it("titles each per-agent guide in the searcher's words, the tool named in title and description", () => {
    const agentGuides = WORKFLOW_GUIDES.filter((g) => g.kind === 'agent');
    expect(Object.fromEntries(agentGuides.map((g) => [g.slug, g.title]))).toEqual({
      'claude-code': 'Make two Claude Code agents talk to each other',
      codex: 'Connect Claude Code and Codex',
      'kimi-code': 'Connect Kimi Code to another agent',
      hermes: 'Connect Hermes agents',
      openclaw: 'Connect OpenClaw agents',
    });
    for (const guide of agentGuides) {
      expect(guide.description, guide.slug).toContain(AGENT_GUIDE_TOOL[guide.slug]);
    }
  });

  it('describes every guide in one line a search result can show', () => {
    for (const guide of WORKFLOW_GUIDES) {
      expect(guide.description.length, guide.slug).toBeGreaterThan(40);
      expect(guide.description.length, guide.slug).toBeLessThanOrEqual(160);
    }
  });

  it('hands the home cards the guide of their use case, never an agent guide or the resume guide', () => {
    expect(USE_CASE_GUIDES.map((g) => [g.slug, g.preset])).toEqual([
      ['connect-planner-executor', 'plan-and-build'],
      ['share-context-between-agents', 'collaborate'],
      ['connect-builder-reviewer', 'build-and-review'],
    ]);
    expect(guideForPreset('plan-and-build')?.slug).toBe('connect-planner-executor');
    expect(guideForPreset('collaborate')?.slug).toBe('share-context-between-agents');
    expect(guideForPreset('build-and-review')?.slug).toBe('connect-builder-reviewer');
    expect(workflowGuide('claude-code')?.title).toBe('Make two Claude Code agents talk to each other');
    expect(workflowGuide('no-such-guide')).toBeUndefined();
    expect(guidePath('codex')).toBe('/docs/guides/codex');
  });

  it('never types a sentence by hand: the one a guide shows is the API\'s', () => {
    const data = JSON.stringify({ WORKFLOW_GUIDES, RUNS });
    for (const piece of [
      'Learn Solvr from',
      'skill.md',
      'Solvr room to',
      'join it as the',
      'answer me with a prompt',
      'install the Solvr skill and join your room',
    ]) {
      expect(data, piece).not.toContain(piece);
    }
    for (const example of CONNECT_EXAMPLES) expect(data).not.toContain(example.prompt.text);
  });

  it('says no hype words, never "seamless" or "simply", and no exclamation mark', () => {
    for (const guide of WORKFLOW_GUIDES) {
      const words = [guide.title, guide.description, ...guideTexts(guide)].join(' ');
      expect(words, guide.slug).not.toMatch(/seamless|simply|effortless|magic|revolution|blazing|instantly|guarantee/i);
      expect(words, guide.slug).not.toContain('!');
    }
  });
});

describe('the run record of every guide', () => {
  it('names every agent: run for the guide, or not run', () => {
    for (const guide of WORKFLOW_GUIDES) {
      const ran = runAgents(guide.record.runs);
      const notRun = Object.keys(guide.record.notRun);
      expect(notRun.filter((agent) => ran.includes(agent as never)), guide.slug).toEqual([]);
      expect([...ran, ...notRun].sort(), guide.slug).toEqual([...AGENTS].sort());
    }
  });

  it('names Hermes and OpenClaw as not run, with the reason the runs found', () => {
    for (const guide of WORKFLOW_GUIDES) {
      expect(JSON.stringify(guide.record.notRun.Hermes), guide.slug).toContain(NOT_RUN_REASON.Hermes);
      expect(JSON.stringify(guide.record.notRun.OpenClaw), guide.slug).toContain(NOT_RUN_REASON.OpenClaw);
    }
    expect(NOT_RUN_REASON.Hermes).toMatch(/not logged in to a model provider/);
    expect(NOT_RUN_REASON.OpenClaw).toMatch(/not installed/);
  });

  it('runs only agents whose version was recorded', () => {
    for (const run of Object.values(RUNS)) {
      for (const side of [run.first, run.second]) {
        expect(AGENT_VERSIONS[side.agent as keyof typeof AGENT_VERSIONS], `${run.id} ${side.agent}`).toBeTruthy();
      }
    }
  });

  it('draws the use-case guides on runs of their own use case', () => {
    for (const guide of USE_CASE_GUIDES) {
      for (const id of guide.record.runs) expect(RUNS[id].preset, `${guide.slug} ${id}`).toBe(guide.preset);
    }
    // The share-context use case was not run with a named agent, and says so.
    expect(workflowGuide('share-context-between-agents')?.record.runs).toEqual([]);
  });

  it("draws each run-backed agent guide on runs that agent took part in", () => {
    for (const guide of WORKFLOW_GUIDES.filter((g) => g.kind === 'agent')) {
      const tool = AGENT_GUIDE_TOOL[guide.slug];
      for (const id of guide.record.runs) expect(runAgents([id]), `${guide.slug} ${id}`).toContain(tool);
    }
    expect(workflowGuide('hermes')?.record.runs).toEqual([]);
    expect(workflowGuide('openclaw')?.record.runs).toEqual([]);
  });

  it('quotes a room only from a run that recorded one, in that run\'s roles and agents', () => {
    for (const guide of WORKFLOW_GUIDES) {
      for (const section of guide.sections) {
        for (const block of section.blocks) {
          if (block.kind !== 'excerpt') continue;
          const run = RUNS[block.run];
          expect(guide.record.runs, guide.slug).toContain(block.run);
          expect(run.excerpt?.length, block.run).toBeGreaterThan(0);
          for (const line of run.excerpt ?? []) {
            const side = [run.first, run.second].find((s) => s.role === line.role);
            expect(side?.agent, `${block.run} ${line.role}`).toBe(line.agent);
          }
        }
      }
    }
  });
});

describe('links in the guides', () => {
  const slugs = WORKFLOW_GUIDES.map((g) => g.slug);
  const allowed = (href: string) =>
    (href.startsWith('/docs/guides/') && slugs.includes(href.slice('/docs/guides/'.length))) ||
    /^\/connect(\?preset=(plan-and-build|collaborate|build-and-review))?$/.test(href) ||
    href === '/rooms' ||
    EXAMPLE_ROOMS.some((room) => room.path === href) ||
    href === 'mailto:hello@solvr.dev';

  it('point only at pages that exist: other guides, Connect, the rooms and the contact address', () => {
    for (const guide of WORKFLOW_GUIDES) {
      for (const link of guideLinks(guide)) expect(allowed(link.href), `${guide.slug} ${link.href}`).toBe(true);
    }
    // The example rooms are rooms pages, which the app has.
    expect(existsSync(join(process.cwd(), 'app', 'rooms', '[slug]', 'page.tsx'))).toBe(true);
  });

  it('carry a stable lowercase item for the click listener', () => {
    for (const guide of WORKFLOW_GUIDES) {
      for (const link of guideLinks(guide)) expect(link.item, `${guide.slug} ${link.href}`).toMatch(/^[a-z][a-z0-9_]*$/);
    }
  });

  it('link the public rooms of 2026-10-03 from the Claude Code and Codex guides', () => {
    expect(EXAMPLE_ROOMS.map((room) => room.path)).toEqual([
      '/rooms/acceptance-1003-p67',
      '/rooms/acceptance-1003-rc67',
      '/rooms/acceptance-1003-g24',
    ]);
    for (const slug of ['claude-code', 'codex']) {
      const hrefs = guideLinks(workflowGuide(slug)!).map((link) => link.href);
      for (const room of EXAMPLE_ROOMS) expect(hrefs, slug).toContain(room.path);
    }
  });

  it('ask the reader of the two guides nobody ran to tell us, at the address the site already lists', () => {
    for (const slug of ['hermes', 'openclaw']) {
      expect(guideLinks(workflowGuide(slug)!).map((link) => link.href), slug).toContain('mailto:hello@solvr.dev');
    }
  });
});
