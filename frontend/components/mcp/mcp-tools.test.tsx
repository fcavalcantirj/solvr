import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { readFileSync } from 'fs';
import { resolve } from 'path';
import { McpTools } from './mcp-tools';

// idx 52 step 2: the npm mcp-server creates canonical posts with no type and replies with
// solvr_reply (its solvr_answer tool was removed). The /mcp page documents that server, so it
// must list exactly the tools mcp-server/src/tools.ts serves and none of the retired choices.
function servedToolNames(): string[] {
  const src = readFileSync(resolve(__dirname, '../../../mcp-server/src/tools.ts'), 'utf8');
  return [...src.matchAll(/^\s+name: '(solvr_\w+)',$/gm)].map((m) => m[1]);
}

function renderedToolNames(): string[] {
  return screen.getAllByText(/^solvr_\w+$/).map((el) => el.textContent ?? '');
}

describe('McpTools', () => {
  it('lists exactly the tools the npm mcp-server serves', () => {
    const served = servedToolNames();
    expect(served).toContain('solvr_reply');
    render(<McpTools />);
    expect(renderedToolNames()).toEqual(served);
  });

  it('documents solvr_reply with its body and optional parent reply', () => {
    render(<McpTools />);
    expect(screen.getByText('solvr_reply')).toBeInTheDocument();
    expect(screen.getByText('post_id')).toBeInTheDocument();
    expect(screen.getByText('body')).toBeInTheDocument();
    expect(screen.getByText('parent_reply_id')).toBeInTheDocument();
  });

  it('offers no retired answer tool, approach angle, include option or post type', () => {
    const { container } = render(<McpTools />);
    expect(screen.queryByText('solvr_answer')).not.toBeInTheDocument();
    expect(screen.queryByText('approach_angle')).not.toBeInTheDocument();
    expect(screen.queryByText('include')).not.toBeInTheDocument();
    // Only solvr_search keeps a type filter (search is idx 53); solvr_post takes no type.
    expect(screen.getAllByText('type', { selector: 'code' })).toHaveLength(1);
    expect(container.textContent).not.toMatch(/problem, question, or idea/);
  });

  it('documents the visibility option of solvr_post', () => {
    render(<McpTools />);
    expect(screen.getByText('visibility')).toBeInTheDocument();
  });
});
