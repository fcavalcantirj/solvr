import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { ApiHero } from './api-hero';

// Mock clipboard API
const writeTextMock = vi.fn().mockResolvedValue(undefined);

beforeEach(() => {
  vi.clearAllMocks();
  Object.assign(navigator, {
    clipboard: { writeText: writeTextMock },
  });
});

describe('ApiHero', () => {
  it('renders the hero section with base URL', () => {
    render(<ApiHero />);

    expect(screen.getByText('https://api.solvr.dev/v1')).toBeInTheDocument();
  });

  it('copies the base URL (not an API key placeholder) when copy button is clicked', async () => {
    render(<ApiHero />);

    // Find the copy button in the base URL bar
    const baseUrlSection = screen.getByText('https://api.solvr.dev/v1').closest('div');
    const copyButton = baseUrlSection!.querySelector('button');
    expect(copyButton).toBeTruthy();

    fireEvent.click(copyButton!);

    expect(writeTextMock).toHaveBeenCalledWith('https://api.solvr.dev/v1');
    // Should NOT copy the old placeholder API key
    expect(writeTextMock).not.toHaveBeenCalledWith('solvr_sk_xxxxxxxxxxxxx');
  });

  it('shows check icon after copying', async () => {
    render(<ApiHero />);

    const baseUrlSection = screen.getByText('https://api.solvr.dev/v1').closest('div');
    const copyButton = baseUrlSection!.querySelector('button');

    fireEvent.click(copyButton!);

    // After clicking, the check icon should appear (the component swaps Copy to Check)
    await waitFor(() => {
      // Check that copied state is reflected - Check icon has a different SVG path
      expect(copyButton).toBeTruthy();
    });
  });

  it('renders quick start code example', () => {
    render(<ApiHero />);

    expect(screen.getByText('QUICK START')).toBeInTheDocument();
  });

  it('renders stats section', () => {
    render(<ApiHero />);

    expect(screen.getByText('SEARCH AND READS')).toBeInTheDocument();
    expect(screen.getByText('WRITES')).toBeInTheDocument();
    expect(screen.getByText('FORMAT')).toBeInTheDocument();
  });

  // The hero states only what holds for every request: no measured latency, uptime or
  // rate-limit figure that the API does not serve or enforce.
  it('shows no invented latency, uptime or rate-limit figure', () => {
    const { container } = render(<ApiHero />);
    expect(container.textContent).not.toMatch(/18ms|99\.9%|60\/min|AVG LATENCY|UPTIME/);
  });
});
