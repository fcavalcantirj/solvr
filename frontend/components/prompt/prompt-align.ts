import type { PromptData, PromptSegment } from "./prompt-types";

// Layout helpers only: they compare segments the API sent, so a slot can hold the
// width of its widest filling and shared words can step back. They never change
// or compose a word.

// reserveAcross maps each token slot (any kind but plain text) whose filling
// differs between the prompts to every filling it takes, so the slot reserves
// the widest one and the words after it stay put.
export function reserveAcross(prompts: PromptData[]): Record<number, PromptSegment[]> {
  const reserve: Record<number, PromptSegment[]> = {};
  if (prompts.length < 2) return reserve;
  const length = Math.min(...prompts.map((p) => p.segments.length));
  for (let i = 0; i < length; i++) {
    const slot = prompts.map((p) => p.segments[i]);
    const kind = slot[0].kind;
    if (kind === "text" || slot.some((s) => s.kind !== kind)) continue;
    if (slot.every((s) => s.text === slot[0].text)) continue;
    reserve[i] = slot;
  }
  return reserve;
}

// sharedText lists the plain-text (and hand-off) segment indexes whose words are
// identical in every prompt: the sentence the use cases share.
export function sharedText(prompts: PromptData[]): Set<number> {
  const shared = new Set<number>();
  if (prompts.length < 2) return shared;
  const length = Math.min(...prompts.map((p) => p.segments.length));
  for (let i = 0; i < length; i++) {
    const slot = prompts.map((p) => p.segments[i]);
    const kind = slot[0].kind;
    const same = slot.every((s) => s.kind === kind && s.text === slot[0].text);
    if (same && (kind === "text" || kind === "handoff")) shared.add(i);
  }
  return shared;
}

// roles reads the pair a prompt names, for the switch's caption.
export function roles(prompt: PromptData): { a?: string; b?: string } {
  const a = prompt.segments.find((s) => s.kind === "role" && s.side === "a")?.text;
  const b = prompt.segments.find((s) => s.kind === "role" && s.side === "b")?.text;
  return { a, b };
}
