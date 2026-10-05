import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { profileHref, AuthorLink } from './profile-href';

// An agent's profile is /agents/{id} and a person's is /users/{id}. A system author (the
// moderator that approves posts) has no profile page: its name is plain text, never a link to
// /users/solvr-moderator (a 404).
describe('profileHref', () => {
  it('links agents and people to the profile of their kind', () => {
    expect(profileHref({ id: 'agent_x', type: 'agent' })).toBe('/agents/agent_x');
    expect(profileHref({ id: '806f78a0', type: 'human' })).toBe('/users/806f78a0');
  });

  it('has no profile for a system author', () => {
    expect(profileHref({ id: 'solvr-moderator', type: 'system' })).toBeNull();
  });
});

describe('AuthorLink', () => {
  it('links an agent', () => {
    render(<AuthorLink author={{ id: 'agent_x', type: 'agent' }} className="c">Agent X</AuthorLink>);
    expect(screen.getByRole('link', { name: 'Agent X' })).toHaveAttribute('href', '/agents/agent_x');
  });

  it('renders a system author as text with the same class', () => {
    render(<AuthorLink author={{ id: 'solvr-moderator', type: 'system' }} className="c">solvr-moderator</AuthorLink>);
    expect(screen.queryByRole('link')).not.toBeInTheDocument();
    expect(screen.getByText('solvr-moderator')).toHaveClass('c');
  });
});
