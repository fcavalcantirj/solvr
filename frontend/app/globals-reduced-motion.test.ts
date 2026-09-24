import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

// The simplified experience must respect a visitor's reduced-motion preference
// (idx 64 step 5). The sheet ships a 20s infinite `.animate-carousel` and 170+
// `animate-*`/`transition-*` utilities plus `scroll-behavior: smooth`; none of
// that may keep moving for someone who asked the OS to reduce motion. This is a
// stylesheet-content guard because jsdom has no layout or media-query engine —
// the actual rendered stillness is confirmed by the owner (see UAT).
const css = readFileSync(join(process.cwd(), 'app/globals.css'), 'utf8');

const reducedMotionBlock = () => {
  const start = css.search(/@media\s*\(\s*prefers-reduced-motion:\s*reduce\s*\)/);
  expect(start).toBeGreaterThanOrEqual(0);
  return css.slice(start);
};

describe('globals.css respects prefers-reduced-motion', () => {
  it('declares a reduced-motion media query', () => {
    expect(css).toMatch(/@media\s*\(\s*prefers-reduced-motion:\s*reduce\s*\)/);
  });

  it('neutralizes every animation and transition under reduced motion', () => {
    const block = reducedMotionBlock();
    // Near-zero duration and a single iteration so the infinite carousel and any
    // animate-*/transition-* utility settle instantly instead of looping.
    expect(block).toMatch(/animation-duration:\s*0\.01ms\s*!important/);
    expect(block).toMatch(/animation-iteration-count:\s*1\s*!important/);
    expect(block).toMatch(/transition-duration:\s*0\.01ms\s*!important/);
  });

  it('disables smooth scrolling under reduced motion', () => {
    expect(reducedMotionBlock()).toMatch(/scroll-behavior:\s*auto\s*!important/);
  });
});

describe('globals.css guards against horizontal overflow', () => {
  it('keeps the body clipped on the x axis so no page-level horizontal scroll appears', () => {
    expect(css).toMatch(/overflow-x-hidden/);
  });
});
