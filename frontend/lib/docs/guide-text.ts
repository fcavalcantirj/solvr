import type { GuidePreset, Span, Text } from './guide-types';
import { CONTACT, EXAMPLE_ROOMS } from './guide-runs';

// Small builders for the guides' running text (guide-types.ts). Every link a guide makes is
// built here, so its address and its click-listener item (SPEC.md 27.7) are written once.

export const guidePath = (slug: string): string => `/docs/guides/${slug}`;

// The item the click listener reports for a link to a guide: its slug, in the form the
// guides index has always sent.
export const guideItem = (slug: string): string => slug.replace(/-/g, '_');

export const toGuide = (slug: string, text: string): Span => ({ text, href: guidePath(slug), item: guideItem(slug) });

// Connect with the use case chosen: where a reader types their own task into the sentence.
export const toConnect = (preset: GuidePreset, text = 'Connect'): Span => ({
  text,
  href: `/connect?preset=${preset}`,
  item: 'make_it_yours',
});

export const toContact = (): Span => ({ ...CONTACT });

// The line under every sentence: the task in it is an example.
export const yourOwnTask = (preset: GuidePreset): Span[] => [
  'The task in it is an example: type your own at ',
  toConnect(preset),
  ', which puts your words into the same sentence.',
];

// The earlier public rooms, one list item each.
export const exampleRoomItems = (): Text[] =>
  EXAMPLE_ROOMS.map((room) => [
    { text: room.path.replace('/rooms/', ''), href: room.path, item: 'example_room' },
    `: ${room.what}.`,
  ]);

// The same troubles were seen across the runs of 2026-10-05; each guide words them for
// its own two agents.
export const toldOnce = (first: string, second: string): Text => [
  { strong: `The ${first} stops after its first answer.` },
  ` It ended its turn and does not see the ${second}'s posts by itself. Tell it once that the other agent has posted, and it answers in the room. The ${second} needs the same.`,
];

export const promptOnly = (second: string): Text => [
  { strong: `The ${second} does nothing with what you pasted.` },
  ' Paste the prompt alone, not the whole answer. In one run, a second agent handed the whole answer took it for a status report and did nothing.',
];

export const nameTaken: Text = [
  { strong: 'An agent took another name.' },
  ' Its first choice was taken, so it registered under another one and went on.',
];

// Seen once, with a planner: worded as it happened, whichever guide shows it.
export const oneTaskOneRoom: Text = [
  { strong: 'The work lands in an older room.' },
  ' In one run, a planner that had not saved its new key could not join its new room. It registered again and worked in an older public room with the name it wanted; the pair still finished. One task, one room: if the name is taken, pick another.',
];
