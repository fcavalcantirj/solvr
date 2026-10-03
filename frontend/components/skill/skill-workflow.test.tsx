import { render } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { SkillHero } from './skill-hero';
import { SkillPreview } from './skill-preview';

// The /skill page tells agents what to do with the skill. Skill 4.0.0 removed the approach and
// answer commands, approach status and typed search: the page offers none of them, and its
// preview shows the workflow skill/SKILL.md actually ships.
const REMOVED_CHOICE =
  /(start|starts|post|posts|contribute)( an| the| your)? approach|approach(es)? status|update (the |your |stale )?approach|type=(problem|question|idea)/i;

const skillMd = readFileSync(join(process.cwd(), '..', 'skill', 'SKILL.md'), 'utf8');
const workflow = (text: string) => text.match(/```\nHit a problem\n[\s\S]*?\n```/)?.[0] ?? null;

describe('/skill page workflow', () => {
  it('the hero offers no removed choice and names the reply workflow', () => {
    const { container, getByText } = render(<SkillHero />);

    expect(container.textContent).not.toMatch(REMOVED_CHOICE);
    for (const step of ['Search first', 'Reply with what you will try', 'Track progress', 'Reply with the outcome']) {
      expect(getByText(step)).toBeInTheDocument();
    }
  });

  it('the preview offers no removed choice', () => {
    const { container } = render(<SkillPreview />);

    expect(container.textContent).not.toMatch(REMOVED_CHOICE);
  });

  it('the preview shows the workflow SKILL.md ships', () => {
    const { container } = render(<SkillPreview />);
    const shipped = workflow(skillMd);

    expect(shipped).not.toBeNull();
    expect(workflow(container.querySelector('pre')?.textContent ?? '')).toBe(shipped);
  });
});
