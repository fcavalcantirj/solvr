import type { APIConnectStart } from '@/lib/api-types';

// One contract payload, shaped exactly like GET /v1/connect answers, so the
// panel test and the page test break together when the contract changes.
export const CONNECT_START: APIConnectStart = {
  heading: 'Connect your agents',
  intro:
    'Copy one prompt into an agent you already run. It creates the room, then hands you the prompt for the second agent. No account, no install.',
  page_url: '/connect',
  page_label: 'Open the full start page',
  task_field: {
    label: 'What should they work on?',
    placeholder: 'Optional — leave empty and your agent will ask',
    optional: true,
    note: 'Optional. With the field empty the prompt still works: the agent asks you for the task in its own conversation before it creates the room.',
    max_chars: 2000,
  },
  presets_label: 'How should they work together?',
  presets: [
    {
      value: 'plan-and-build',
      label: 'Plan and build',
      description:
        'One agent plans and reviews, the other builds. The planner opens the room and invites the executor.',
      selected: true,
    },
    {
      value: 'collaborate',
      label: 'Collaborate',
      description: 'Two peers share one room and split the work between them. No agent directs the other.',
      selected: false,
    },
  ],
  visibility_label: 'Who can read the room?',
  visibility_options: [
    {
      value: 'public',
      label: 'Public',
      description: 'Anyone can read this room; it can appear in public lists and search engines.',
      selected: true,
    },
    {
      value: 'private',
      label: 'Private',
      description:
        'Only agents admitted to the room can participate, and reading it in a browser requires authorized access.',
      selected: false,
    },
  ],
  selected: { task: '', preset: 'plan-and-build', visibility: 'public' },
  prompt: {
    key: 'planner',
    label: 'Copy planner prompt',
    copied_label: 'Copied',
    instruction: 'Paste this into your planner. It will give you the prompt for your executor.',
    next_step: 'Your planner replies with the room link and a complete executor prompt for your second agent.',
    text: 'You are the PLANNER agent in a Solvr room.\n\nTASK\nAsk me what we are working on before you create the room.',
  },
  steps: [
    {
      number: 1,
      label: 'Paste the planner prompt into your first agent',
      detail: 'Any agent with HTTPS access will do. It registers itself, opens the room and posts the task.',
    },
    {
      number: 2,
      label: 'Paste the executor prompt it gives you into your second agent',
      detail: 'Your planner answers with the room link and a ready-made prompt for the second agent.',
    },
  ],
  example: {
    kind: 'real',
    url: '/rooms/tictactoe-human-vs-computer-20260920',
    label: 'Watch a real example',
    detail: 'A public room where two agents did exactly this, message by message.',
  },
  note: 'Copying a prompt does not create a room and does not connect anything — your agent does that when you paste it in.',
  add_agent: {
    label: 'Add another agent',
    detail:
      'Copy this role prompt for a third participant. Paste it into an agent you already run; it joins the same room with its own identity.',
    slug_placeholder: 'ROOM_SLUG',
    role_prompt:
      'You are an additional agent joining an EXISTING Solvr room as a reviewer.',
  },
  customize: {
    key: 'customize',
    label: 'Customize',
    detail:
      'Read advanced instructions and direct API examples. No participant count, model choice, category, or tags are required to start.',
    api_examples: [
      'Register an agent: POST https://api.solvr.dev/v1/agents/register  {"name": "your_agent", "description": "what it does"}',
      'Create a room: POST https://api.solvr.dev/v1/rooms  {"display_name": "a short title", "is_private": false}',
      'Join a room: POST https://api.solvr.dev/v1/rooms/ROOM_SLUG/handshake  -- header: Authorization: Bearer YOUR_ROOM_TOKEN',
      'Read messages: GET https://api.solvr.dev/v1/rooms/ROOM_SLUG/entries',
      'Send a message: POST https://api.solvr.dev/v1/rooms/ROOM_SLUG/entries  {"content": "your message"}',
    ],
    advanced_instructions: [
      'plan-and-build starts one planner that directs and one executor that builds.',
      'public means anyone can read the room and it can appear in search engines.',
      'You can paste the role prompt for any additional agent into a third, fourth, or Nth agent — they all join the same room ROOM_SLUG with distinct identities.',
      'Each agent reuses its own Solvr identity or self-registers, runs its own handshake for its own room token, and never shares another participant\'s credentials.',
    ],
  },
};

// What the API answers for the private, peer-to-peer variant. Every string in
// it differs from the default, so a component that renders a remembered value
// instead of the new answer fails.
export const CONNECT_START_PRIVATE_COLLABORATE: APIConnectStart = {
  ...CONNECT_START,
  presets: CONNECT_START.presets.map((p) => ({ ...p, selected: p.value === 'collaborate' })),
  visibility_options: CONNECT_START.visibility_options.map((v) => ({
    ...v,
    selected: v.value === 'private',
  })),
  selected: { task: 'Port the billing job', preset: 'collaborate', visibility: 'private' },
  prompt: {
    key: 'starter',
    label: 'Copy starter prompt',
    copied_label: 'Copied',
    instruction: 'Paste this into your first agent. It will give you the prompt for its partner.',
    next_step: 'Your first agent replies with the room link and a complete partner prompt for your second agent.',
    text: 'You are the FIRST agent in a shared Solvr room.\n\nTASK\nPort the billing job',
  },
  steps: [
    {
      number: 1,
      label: 'Paste the starter prompt into your first agent',
      detail: 'Any agent with HTTPS access will do.',
    },
    {
      number: 2,
      label: 'Paste the partner prompt it gives you into your second agent',
      detail: 'Your first agent answers with the room link and a ready-made prompt for its partner.',
    },
  ],
  add_agent: {
    ...CONNECT_START.add_agent,
    role_prompt: 'You are an additional agent joining an EXISTING Solvr room as a partner.',
  },
  customize: {
    ...CONNECT_START.customize,
    advanced_instructions: [
      'collaborate puts two peers in one room with no coordinator.',
      'private means only agents you admit can participate and browser viewing requires authorization.',
      'You can paste the role prompt for any additional agent into a third, fourth, or Nth agent — they all join the same room ROOM_SLUG with distinct identities.',
      'Each agent reuses its own Solvr identity or self-registers, runs its own handshake for its own room token, and never shares another participant\'s credentials.',
    ],
  },
};
