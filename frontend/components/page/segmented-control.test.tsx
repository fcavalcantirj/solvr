import { fireEvent, render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { SegmentedControl } from './segmented-control';

// One square window selector for every statistics section: the options are the
// caller's (the API's), the pressed one is the selected window.

const OPTIONS = [
  { value: '24h', label: '24 hours' },
  { value: '7d', label: '7 days' },
];

describe('SegmentedControl', () => {
  it('is a group named by the label it was given, with the selected option pressed', () => {
    render(<SegmentedControl label="Time window" options={OPTIONS} value="7d" onSelect={() => {}} />);
    expect(screen.getByRole('group', { name: 'Time window' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '7 days' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('button', { name: '24 hours' })).toHaveAttribute('aria-pressed', 'false');
  });

  it('takes its name from a visible heading when the API sent no label', () => {
    render(
      <>
        <h2 id="live-heading">Live Search Activity</h2>
        <SegmentedControl labelledBy="live-heading" options={OPTIONS} value="24h" onSelect={() => {}} />
      </>,
    );
    expect(screen.getByRole('group', { name: 'Live Search Activity' })).toBeInTheDocument();
  });

  it('sends back the value of the option chosen', () => {
    const onSelect = vi.fn();
    render(<SegmentedControl label="Time window" options={OPTIONS} value="24h" onSelect={onSelect} />);
    fireEvent.click(screen.getByRole('button', { name: '7 days' }));
    expect(onSelect).toHaveBeenCalledWith('7d');
  });

  it('cannot be used while a window is being read', () => {
    render(<SegmentedControl label="Time window" options={OPTIONS} value="24h" onSelect={() => {}} disabled />);
    expect(screen.getByRole('button', { name: '7 days' })).toBeDisabled();
  });
});
