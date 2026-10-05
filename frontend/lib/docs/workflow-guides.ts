// The workflow guides (task idx 84, SPEC.md 27.5): nine how-tos under /docs/guides, by use
// case and by agent. Each is data (guides-use-cases.ts, guides-agents.ts) rendered by one
// page component (app/docs/guides/[slug]/page.tsx), and carries a run record of what was
// really run for it (guide-runs.ts). The sentence a guide shows is the API's
// (GET /v1/connect/examples); the backend guide tests run that sentence over plain HTTPS
// (internal/api/router_guides_http_test.go).

import type { Block, Span, Text, WorkflowGuide } from './guide-types';
import { RUNS } from './guide-runs';
import { USE_CASE_GUIDE_LIST } from './guides-use-cases';
import { AGENT_GUIDE_LIST } from './guides-agents';

export type { GuideKind, GuidePreset, WorkflowGuide } from './guide-types';
export { guideItem, guidePath } from './guide-text';

// Every guide, in the order the guides index lists them.
export const WORKFLOW_GUIDES: WorkflowGuide[] = [...USE_CASE_GUIDE_LIST, ...AGENT_GUIDE_LIST];

// The guides index groups them by use case (the resume guide among them) and by agent.
export const GUIDE_GROUPS: { id: 'use-case' | 'agent'; heading: string; guides: WorkflowGuide[] }[] = [
  { id: 'use-case', heading: 'By use case', guides: USE_CASE_GUIDE_LIST },
  { id: 'agent', heading: 'By agent', guides: AGENT_GUIDE_LIST },
];

// The guide of each use case: the one the home cards link beside its sentence.
export const USE_CASE_GUIDES = WORKFLOW_GUIDES.filter((g) => g.kind === 'use-case');

export function workflowGuide(slug: string): WorkflowGuide | undefined {
  return WORKFLOW_GUIDES.find((g) => g.slug === slug);
}

// The guide for a use case, if there is one: never an agent guide or the resume guide,
// which show the same sentence.
export function guideForPreset(preset: string): WorkflowGuide | undefined {
  return USE_CASE_GUIDES.find((g) => g.preset === preset);
}

const spans = (text: Text): Span[] => (typeof text === 'string' ? [text] : text);

// The plain words of a piece of running text.
export function plainText(text: Text): string {
  return spans(text)
    .map((span) => (typeof span === 'string' ? span : 'strong' in span ? span.strong : 'code' in span ? span.code : span.text))
    .join('');
}

function blockTexts(block: Block): Text[] {
  switch (block.kind) {
    case 'paragraph':
      return [block.text];
    case 'steps':
    case 'list':
      return block.items;
    case 'command':
      return [block.code];
    case 'excerpt':
      return [block.caption, ...(RUNS[block.run].excerpt ?? []).map((line) => line.text)];
  }
}

function allTexts(guide: WorkflowGuide): Text[] {
  return [
    guide.intro,
    guide.sentenceNote,
    ...guide.sections.flatMap((section) => [section.heading, ...section.blocks.flatMap(blockTexts)]),
    ...Object.values(guide.record.notRun).filter((t): t is Text => t !== undefined),
    ...guide.record.observed,
  ];
}

// Everything a guide says, piece by piece: the page renders all of it in its server HTML.
export function guideTexts(guide: WorkflowGuide): string[] {
  return allTexts(guide).map(plainText);
}

// Every link a guide's text makes.
export function guideLinks(guide: WorkflowGuide): { text: string; href: string; item: string }[] {
  return allTexts(guide)
    .flatMap(spans)
    .filter((span): span is { text: string; href: string; item: string } => typeof span !== 'string' && 'href' in span);
}
