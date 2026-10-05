import { render, screen, fireEvent, within, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

import { CONNECT_EXAMPLES, CONNECT_FLOW_ID, CONNECT_START, CONNECT_START_NO_FLOW } from '@/components/connect/connect-fixture';
import { Prompt } from './prompt';
import { PromptStack } from './prompt-stack';
import { PromptSentence } from './prompt-sentence';
import { reserveAcross, sharedText } from './prompt-align';

// The Prompt renders the API's segments by kind and copies the API's text. It never
// writes a word of the sentence.

beforeEach(() => {
  Object.assign(navigator, { clipboard: { writeText: vi.fn().mockResolvedValue(undefined) } });
});

const plan = CONNECT_EXAMPLES[0];

describe('PromptSentence', () => {
  it('renders every segment, in order, and nothing else', () => {
    const { container } = render(<p><PromptSentence segments={plan.prompt.segments} /></p>);
    expect(container.textContent).toBe(plan.prompt.text);
  });

  it('is read-only unless editable: no text box and no switch', () => {
    render(<p><PromptSentence segments={plan.prompt.segments} /></p>);
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
    expect(screen.queryByRole('switch')).not.toBeInTheDocument();
  });

  it('reports what the visitor types into the intent, and restores it on Escape', () => {
    const onIntent = vi.fn();
    render(<p><PromptSentence segments={plan.prompt.segments} editable onIntentChange={onIntent} /></p>);
    const slot = screen.getByRole('textbox');
    fireEvent.focus(slot);
    slot.textContent = 'ship it';
    fireEvent.input(slot);
    expect(onIntent).toHaveBeenLastCalledWith('ship it');
    fireEvent.keyDown(slot, { key: 'Escape' });
    expect(slot.textContent).toBe('ship the signup page');
    expect(onIntent).toHaveBeenLastCalledWith('ship the signup page');
  });

  it('keeps a paste within the limit the API set', () => {
    const onIntent = vi.fn();
    render(<p><PromptSentence segments={plan.prompt.segments} editable onIntentChange={onIntent} intentMaxChars={10} /></p>);
    const slot = screen.getByRole('textbox');
    slot.textContent = 'a much longer intent than allowed';
    fireEvent.input(slot);
    expect(onIntent).toHaveBeenLastCalledWith('a much lon');
  });

  // The skill link may carry the visit's flow code (?f=<code>). It is one link to the
  // whole address the API served; only the look of the part after "?" steps back.
  it('renders a skill link with a flow code as one link to the whole address', () => {
    const withCode = CONNECT_START.prompt;
    const address = withCode.segments.find((s) => s.kind === 'link')!.text;
    expect(address).toMatch(/^https:\/\/solvr\.dev\/skill\.md\?f=[a-hjkmnp-z2-9]{8}$/);
    const { container } = render(<p><PromptSentence segments={withCode.segments} /></p>);

    expect(container.textContent).toBe(withCode.text);
    const link = screen.getByRole('link', { name: address });
    expect(link).toHaveAttribute('href', address);
    expect(link.textContent).toBe(address);
    const quiet = link.querySelector('span');
    expect(quiet).toHaveTextContent(address.slice(address.indexOf('?')));
    expect(quiet).toHaveClass('text-muted-foreground');
  });

  it('renders a plain skill link with nothing stepping back', () => {
    render(<p><PromptSentence segments={plan.prompt.segments} /></p>);
    const link = screen.getByRole('link', { name: 'https://solvr.dev/skill.md' });
    expect(link).toHaveAttribute('href', 'https://solvr.dev/skill.md');
    expect(link.querySelector('span')).toBeNull();
  });

  // /connect first shows the sentence its server read, whose link has no flow code yet; the
  // browser's answer adds "?f=" and the code. Until then (reserveFlowCode) the link holds that
  // width, empty, so the code arriving moves no word. Nothing is in the placeholder: the text a
  // crawler, a screen reader or a copy gets is still exactly the API's.
  describe('reserving the width of the flow code', () => {
    const plainLink = (container: HTMLElement) => container.querySelector<HTMLAnchorElement>('a[data-kind="link"]')!;

    it('holds an empty, hidden placeholder right after the link text, inside the link', () => {
      const { container } = render(<p><PromptSentence segments={plan.prompt.segments} reserveFlowCode /></p>);
      const link = plainLink(container);
      const held = link.querySelectorAll('span');
      expect(held).toHaveLength(1);
      const placeholder = held[0];
      expect(placeholder).toHaveAttribute('aria-hidden', 'true');
      expect(placeholder.childNodes).toHaveLength(0);
      expect(link.lastChild).toBe(placeholder);
      expect(link.textContent).toBe('https://solvr.dev/skill.md');
      expect(container.textContent).toBe(plan.prompt.text);
    });

    // One mono cell per character the code part will have, in the size the code is set in.
    it('is as wide as "?f=" and a code, in the face and size of the code it stands for', () => {
      const { container } = render(<p><PromptSentence segments={plan.prompt.segments} reserveFlowCode /></p>);
      const placeholder = plainLink(container).querySelector('span')!;
      expect([...placeholder.classList].sort()).toEqual(['inline-block', 'text-[0.78em]', 'w-[11ch]']);
      expect(`?f=${CONNECT_FLOW_ID}`).toHaveLength(11);

      const coded = render(<p><PromptSentence segments={CONNECT_START.prompt.segments} /></p>);
      const rest = plainLink(coded.container).querySelector('span')!;
      const fontClasses = (el: Element) => [...el.classList].filter((c) => c.startsWith('text-[') || c.startsWith('font-'));
      expect(fontClasses(placeholder)).toEqual(fontClasses(rest));
    });

    it('holds nothing without the prop', () => {
      const { container } = render(<p><PromptSentence segments={plan.prompt.segments} /></p>);
      expect(plainLink(container).children).toHaveLength(0);
    });

    it('holds nothing once the link carries its code: the code takes the place', () => {
      const { container } = render(<p><PromptSentence segments={CONNECT_START.prompt.segments} reserveFlowCode /></p>);
      const spans = plainLink(container).querySelectorAll('span');
      expect(spans).toHaveLength(1);
      expect(spans[0]).toHaveTextContent(`?f=${CONNECT_FLOW_ID}`);
      expect(spans[0]).not.toHaveAttribute('aria-hidden');
    });
  });

  // The link is for the agent that receives the sentence. A crawler that renders the page
  // is told not to walk it: a coded link (?f=) is one visit's, not a page of the site.
  it.each([
    ['with a flow code', CONNECT_START.prompt.segments],
    ['plain', plan.prompt.segments],
  ])('tells crawlers not to follow the skill link (%s)', (_name, segments) => {
    render(<p><PromptSentence segments={segments} /></p>);
    const link = screen.getByRole('link');
    expect(link).toHaveAttribute('rel', 'nofollow noreferrer');
    expect(link).toHaveAttribute('target', '_blank');
  });

  it('flips the visibility only when editable, as a switch', () => {
    const onFlip = vi.fn();
    render(<p><PromptSentence segments={plan.prompt.segments} editable onVisibilityToggle={onFlip} /></p>);
    const sw = screen.getByRole('switch', { name: 'Private room' });
    expect(sw).toHaveAttribute('aria-checked', 'false');
    fireEvent.click(sw);
    expect(onFlip).toHaveBeenCalledTimes(1);
  });
});

describe('Prompt variants', () => {
  it('guide: the sentence big and read-only, the line on what happens next, one Copy', async () => {
    render(<Prompt variant="guide" preset={plan} />);
    expect(screen.getByTestId('prompt-sentence').textContent).toBe(plan.prompt.text);
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
    expect(screen.getByText(plan.next)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /copy prompt/i }));
    await waitFor(() => expect(navigator.clipboard.writeText).toHaveBeenCalledWith(plan.prompt.text));
  });

  it('card: compact, titled, copies its own sentence', async () => {
    const onCopied = vi.fn();
    render(<Prompt variant="card" title={plan.label} preset={plan} onCopied={onCopied} footer={<a href="/connect">Make it yours</a>} />);
    expect(screen.getByRole('heading', { level: 3, name: plan.label })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /copy prompt/i }));
    await waitFor(() => expect(onCopied).toHaveBeenCalled());
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(plan.prompt.text);
    expect(screen.getByRole('link', { name: 'Make it yours' })).toBeInTheDocument();
  });

  it('connect: the active sentence is editable; the others are held in place, hidden', () => {
    const [active, ...others] = CONNECT_START.presets;
    const { container } = render(<Prompt variant="connect" preset={active} others={others} />);
    expect(within(screen.getByTestId('prompt-sentence')).getByRole('textbox')).toBeInTheDocument();
    const held = container.querySelectorAll('p.prompt-sentence[aria-hidden="true"]');
    expect(held).toHaveLength(2);
    held.forEach((p) => expect(p.querySelector('[role="textbox"]')).toBeNull());
  });

  // The held sentences fill the same place as the one shown (a wide screen keeps all three),
  // so each of them holds the code's width too: none grows when the codes arrive.
  it('connect: reserves the flow code in the sentence shown and in every one held', () => {
    const [active, ...others] = CONNECT_START_NO_FLOW.presets;
    const { container, rerender } = render(<Prompt variant="connect" preset={active} others={others} reserveFlowCode />);
    const placeholders = () => container.querySelectorAll('p.prompt-sentence a[data-kind="link"] > span[aria-hidden="true"]');
    expect(placeholders()).toHaveLength(3);

    rerender(<Prompt variant="connect" preset={active} others={others} />);
    expect(placeholders()).toHaveLength(0);
  });
});

describe('PromptStack lines the use cases up word for word', () => {
  it('reserves every token slot that differs, and only those', () => {
    const reserve = reserveAcross(CONNECT_EXAMPLES.map((p) => p.prompt));
    const kinds = Object.keys(reserve).map((i) => plan.prompt.segments[Number(i)].kind);
    expect(kinds.sort()).toEqual(['intent', 'role', 'role', 'visibility']);
  });

  it('steps back the words every use case shares, the hand-off included', () => {
    const quiet = sharedText(CONNECT_EXAMPLES.map((p) => p.prompt));
    const words = [...quiet].map((i) => plan.prompt.segments[i].text);
    expect(words).toContain('Learn Solvr from ');
    expect(words).toContain('answer me with a prompt for the ');
    expect(words).not.toContain(plan.prompt.segments.find((s) => s.kind === 'intent')!.text);
  });

  it('renders the three sentences exactly, one per row, with their labels', () => {
    render(<PromptStack presets={CONNECT_EXAMPLES} />);
    const rows = within(screen.getByTestId('prompt-stack')).getAllByRole('listitem');
    expect(rows).toHaveLength(3);
    rows.forEach((row, i) => {
      expect(row).toHaveTextContent(CONNECT_EXAMPLES[i].label);
      const visible = [...row.querySelectorAll('p.prompt-sentence > span')]
        .map((slot) => (slot.classList.contains('inline-grid') ? slot.firstElementChild!.textContent : slot.textContent))
        .join('');
      expect(visible).toBe(CONNECT_EXAMPLES[i].prompt.text);
    });
  });
});
