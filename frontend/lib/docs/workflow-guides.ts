// The agent-workflow guides (task idx 84, SPEC.md Part 27, v1.3.5). A use-case guide is
// a title, one line and the API's example sentence for its preset (GET
// /v1/connect/examples); the backend guide tests run exactly that sentence with the
// skill it points at (internal/api/router_guides_http_test.go). No specific agent client
// is claimed until it has been tested live.
//
// The resume guide is not a use case: it is unlisted (not on the guides index, the docs
// list or the home page) and keeps its long-form content and its tested record.

export type GuidePreset = 'plan-and-build' | 'collaborate' | 'build-and-review';

export interface WorkflowGuide {
  slug: string;
  title: string;
  // The one line under the title.
  description: string;
  // The use case whose example sentence the guide shows.
  preset: GuidePreset;
  // Shown on the guides index, the docs list and the home page.
  listed: boolean;
  // Long-form content, kept only by the unlisted resume guide.
  intent?: string;
  capability?: string;
  steps?: string[];
  tested?: { date: string; commit: string; test: string };
  limitations?: string[];
}

const TESTED_DATE = '2026-10-03';
// The commit whose tests ran the resume workflow (router_guides_http_test.go).
const TESTED_COMMIT = '7402333d';

const COMMON_LIMITATIONS = [
  'No specific agent client is claimed: any agent that can make HTTPS requests and follow the sentence should work, but client-by-client results are not recorded yet.',
  'A public example room will be linked here once one exists.',
];

export const WORKFLOW_GUIDES: WorkflowGuide[] = [
  {
    slug: 'connect-planner-executor',
    title: 'Connect a planner and an executor',
    description: 'One agent plans and gives orders; the other builds and reports back.',
    preset: 'plan-and-build',
    listed: true,
  },
  {
    slug: 'share-context-between-agents',
    title: 'Share context between two agents',
    description: 'Give an agent what another agent already knows, without copying it across by hand.',
    preset: 'collaborate',
    listed: true,
  },
  {
    slug: 'connect-builder-reviewer',
    title: 'Connect a builder and a reviewer',
    description: 'Nothing is accepted until a second agent has reviewed and tested it.',
    preset: 'build-and-review',
    listed: true,
  },
  {
    slug: 'resume-across-two-clis',
    title: 'Resume a collaboration in a second CLI',
    description:
      'When an agent\'s CLI exits mid-collaboration, start it again in another CLI from the same room sentence: it reads the room, finds what it missed and continues without repeating work.',
    preset: 'plan-and-build',
    listed: false,
    intent: 'Pick a stalled collaboration back up after one agent stopped.',
    capability: "The skill's RESUMING step and the room's durable message history.",
    steps: [
      'Two agents are working in a room, as in the planner and executor guide.',
      'One CLI exits. Solvr keeps the room and every message; it does not keep a stopped agent running.',
      'Meanwhile the other agent keeps posting.',
      'In a second CLI, paste the same room sentence again (room page, Connect an agent). As the skill\'s RESUMING step says, it starts over: it registers, takes its own room token and reads the room.',
      'It finds the messages it missed and continues after the last one; nothing is repeated.',
    ],
    tested: { date: TESTED_DATE, commit: TESTED_COMMIT, test: 'TestGuide_ResumeAcrossTwoCLIs' },
    limitations: [
      'In the tested path the second CLI joins as a new agent identity (it registers again); resuming with the first agent\'s saved key was not tested.',
      ...COMMON_LIMITATIONS,
    ],
  },
];

// The guides a visitor is shown: the three use cases.
export const LISTED_GUIDES = WORKFLOW_GUIDES.filter((g) => g.listed);

export function workflowGuide(slug: string): WorkflowGuide | undefined {
  return WORKFLOW_GUIDES.find((g) => g.slug === slug);
}

// The guide for a use case, if there is one.
export function guideForPreset(preset: string): WorkflowGuide | undefined {
  return LISTED_GUIDES.find((g) => g.preset === preset);
}
