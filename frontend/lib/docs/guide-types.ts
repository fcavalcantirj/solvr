// The shape of a workflow guide (SPEC.md 27.5). A guide is data: the page component
// (app/docs/guides/[slug]/page.tsx) renders every guide the same way, on the server.
//
// A guide never holds the sentence itself: the page reads it from the API and shows it
// with GuidePrompt. The run facts it quotes live in guide-runs.ts.

export type GuidePreset = 'plan-and-build' | 'collaborate' | 'build-and-review';

// use-case: one of the three sentences; resume: picking a stopped collaboration back up;
// agent: one guide per agent tool.
export type GuideKind = 'use-case' | 'resume' | 'agent';

export type AgentName = 'Claude Code' | 'Codex' | 'Kimi Code' | 'Hermes' | 'OpenClaw';

// The real two-agent runs of 2026-10-05 (guide-runs.ts).
export type RunId = 'p5' | 'p2' | 'p3';

// A piece of running text: plain words, words set in bold or as code, or a link marked for
// the click listener (`item`, SPEC.md 27.7). A link goes to a page of the site or to the
// contact address.
export type Span = string | { strong: string } | { code: string } | { text: string; href: string; item: string };

export type Text = string | Span[];

export type Block =
  | { kind: 'paragraph'; text: Text }
  // An ordered list: steps, in the order they happen.
  | { kind: 'steps'; items: Text[] }
  | { kind: 'list'; items: Text[] }
  // A command line, shown as code. A sentence in it is a placeholder, never the sentence.
  | { kind: 'command'; code: string }
  // A trimmed excerpt of a run's room (guide-runs.ts), with the line that says where it is from.
  | { kind: 'excerpt'; run: RunId; caption: Text };

export interface GuideSection {
  // The anchor of the section's heading.
  id: string;
  heading: string;
  blocks: Block[];
}

// The run record every guide carries: the real-agent runs it draws on (their agents,
// versions, task and timing come from guide-runs.ts), the backend guide tests that run it,
// every agent that was not run for it and why, and what was observed.
export interface GuideRecord {
  runs: RunId[];
  tests: string[];
  notRun: Partial<Record<AgentName, Text>>;
  observed: Text[];
}

export interface WorkflowGuide {
  slug: string;
  kind: GuideKind;
  // The title tag and the page's one heading.
  title: string;
  // The meta description, and the guide's one line on the guides index.
  description: string;
  // The use case whose sentence the guide shows.
  preset: GuidePreset;
  // The first paragraph: what the guide is for.
  intro: Text;
  // The line under the sentence: who gets it, and where to make it your own.
  sentenceNote: Text;
  sections: GuideSection[];
  record: GuideRecord;
}
