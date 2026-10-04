// The Prompt component renders the API's sentence exactly as served (lib/connect-types.ts):
// the segments' texts concatenate to `text`, and Copy copies `text`. The page never
// composes a word of the sentence.
import type {
  APIConnectPreset,
  APIPrompt,
  APIPromptSegment,
  APIPromptSegmentKind,
} from '@/lib/connect-types';

export type PromptSegmentKind = APIPromptSegmentKind;
export type PromptSegment = APIPromptSegment;
export type PromptData = APIPrompt;
// One use case and its filled sentence, as listed by the API.
export type PromptPreset = APIConnectPreset;
