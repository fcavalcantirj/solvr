import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { ApiEndpoints } from './api-endpoints';

// idx 52 step 4: a retired legacy write is shown as retired, with its replacement and the
// migration from SPEC.md 26.6, and never offers "Try it" against a route that answers 410.

function openEndpoint(group: string, path: string) {
  fireEvent.click(screen.getByRole('button', { name: new RegExp(`^${group}\\b`) }));
  const code = screen.getAllByText(path, { selector: 'code' })[0];
  fireEvent.click(code.closest('button')!);
}

describe('ApiEndpoints retired legacy writes', () => {
  it('labels a retired write and shows its replacement and migration instead of Try it', () => {
    render(<ApiEndpoints />);
    // A path of its own: the card key is group+path, so a GET and a POST on one path open together.
    openEndpoint('Questions', '/answers/{id}/vote');

    expect(screen.getAllByText('RETIRED').length).toBeGreaterThan(0);
    expect(screen.getByText('POST /v1/replies/{id}/vote', { selector: 'code' })).toBeInTheDocument();
    expect(screen.getByText('{direction} → the same, on the reply id of the migrated answer')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /try it/i })).not.toBeInTheDocument();
  });

  it('says a retired command has no canonical equivalent', () => {
    render(<ApiEndpoints />);
    openEndpoint('Ideas', '/ideas/{id}/evolve');

    expect(screen.getByText('This command has no canonical equivalent.')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /try it/i })).not.toBeInTheDocument();
  });

  it('keeps Try it and no RETIRED label on a live endpoint', () => {
    render(<ApiEndpoints />);
    openEndpoint('Replies', '/replies/{id}/vote');

    expect(screen.queryByText('RETIRED')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /try it/i })).toBeInTheDocument();
  });
});
