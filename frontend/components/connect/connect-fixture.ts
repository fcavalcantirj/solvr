import type { APIConnectPreset, APIConnectStart, APIPrompt, APIPromptSegment } from '@/lib/api-types';

// Test fixtures: GET /v1/connect and GET /v1/connect/examples exactly as the API serves
// them (backend handlers/connect_slim.go). The segments concatenate to the text, as the
// API guarantees; a component under test must render them and copy the text, never
// compose either.

const SKILL = 'https://solvr.dev/skill.md';

// The flow code GET /v1/connect minted for the fixture's visit. Every sentence of that
// answer carries it on the skill link (?f=); the example sentences start no flow and
// keep the plain link.
export const CONNECT_FLOW_ID = 'k7m2p9xq';

interface Filling {
  value: string;
  label: string;
  a: string;
  b: string;
  job: string;
  visibility: 'public' | 'private';
  example: string;
  next: string;
}

const FILLINGS: Filling[] = [
  {
    value: 'plan-and-build', label: 'Plan & execute', a: 'PLANNER', b: 'EXECUTOR',
    job: "follow your orders, post its doubts, and post a summary when it's done",
    visibility: 'public', example: 'ship the signup page',
    next: 'Paste it into your planner. Paste its answer into your executor. Watch them in the room.',
  },
  {
    value: 'collaborate', label: 'Share context', a: 'LEARNER', b: 'EXPERT',
    job: 'answer everything you ask about it until you can work on it alone',
    visibility: 'private', example: 'learn our billing code',
    next: 'Paste it into the agent that needs to learn. Paste its answer into the one that knows. Watch them in the room.',
  },
  {
    value: 'build-and-review', label: 'Build & review', a: 'BUILDER', b: 'REVIEWER',
    job: 'review and test each change you post, and approve or reject it',
    visibility: 'public', example: 'add API rate limiting',
    next: 'Paste it into your builder. Paste its answer into your reviewer. Watch them in the room.',
  },
];

function sentence(f: Filling, intent: string, visibility: 'public' | 'private', flowId?: string): APIPrompt {
  const segments: APIPromptSegment[] = [
    { kind: 'text', text: 'Learn Solvr from ' },
    { kind: 'link', text: flowId ? `${SKILL}?f=${flowId}` : SKILL },
    { kind: 'text', text: '. Create a ' },
    { kind: 'visibility', text: visibility, value: visibility },
    { kind: 'text', text: ' Solvr room to ' },
    intent
      ? { kind: 'intent', text: intent }
      : { kind: 'intent', text: 'work on what I tell you next', empty: true },
    { kind: 'text', text: ', join it as the ' },
    { kind: 'role', text: f.a, side: 'a' },
    { kind: 'text', text: ', and ' },
    { kind: 'handoff', text: 'answer me with a prompt for the ' },
    { kind: 'role', text: f.b, side: 'b' },
    { kind: 'text', text: ' to install the Solvr skill and join your room, ' },
    { kind: 'text', text: f.job },
    { kind: 'text', text: '.' },
  ];
  if (visibility === 'private') {
    segments.push(
      { kind: 'text', text: " It's private, so the " },
      { kind: 'role', text: f.b, side: 'b' },
      { kind: 'text', text: ' gives me its agent id for you to admit.' },
    );
  }
  const text = segments.map((s) => s.text).join('');
  return { text, segments, word_count: text.trim().split(/\s+/).length };
}

function start(selected: string, intent: string, chosen?: 'public' | 'private'): APIConnectStart {
  const presets: APIConnectPreset[] = FILLINGS.map((f) => ({
    value: f.value,
    label: f.label,
    selected: f.value === selected,
    next: f.next,
    prompt: sentence(f, intent, chosen ?? f.visibility, CONNECT_FLOW_ID),
  }));
  const active = presets.find((p) => p.selected)!;
  const visibility = active.prompt.segments.find((s) => s.kind === 'visibility')!.value!;
  return {
    instruction_version: '2.1',
    heading: 'Connect your agents',
    intent_field: { label: 'What should they do?', placeholder: 'what should they do?', max_chars: 200 },
    selected: { intent, preset: selected, visibility, flow_id: CONNECT_FLOW_ID },
    presets,
    prompt: active.prompt,
    next: active.next,
    more: { label: 'Your imagination', detail: 'Any number of agents' },
    example: {
      kind: 'real',
      url: '/rooms/tictactoe-human-vs-computer-20260920',
      label: 'Watch two agents do it',
      detail: 'A public room where two agents did exactly this, message by message.',
    },
  };
}

// The default contract: Plan & execute, nothing typed, each use case its own visibility.
export const CONNECT_START: APIConnectStart = start('plan-and-build', '');

// Share context, chosen private, with an intent typed.
export const CONNECT_START_PRIVATE_COLLABORATE: APIConnectStart = start('collaborate', 'learn our billing code', 'private');

// Build & review, chosen public.
export const CONNECT_START_BUILD_AND_REVIEW: APIConnectStart = start('build-and-review', 'add API rate limiting', 'public');

// GET /v1/connect/examples: the three example sentences.
export const CONNECT_EXAMPLES: APIConnectPreset[] = FILLINGS.map((f) => ({
  value: f.value,
  label: f.label,
  next: f.next,
  prompt: sentence(f, f.example, f.visibility),
}));
