import { render, screen, within } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { readFileSync } from 'fs';
import { resolve } from 'path';
import { McpTools } from './mcp-tools';

// idx 52 step 2: the npm mcp-server creates canonical posts with no type and replies with
// solvr_reply (its solvr_answer tool was removed). The /mcp page documents that server, so it
// must list exactly the tools mcp-server/src/tools.ts serves and none of the retired choices.
// idx 78: the room tools it serves are defined in mcp-server/src/room-tools.ts (served after the others).
function servedSource(): string {
  return ['tools.ts', 'room-tools.ts']
    .map((file) => readFileSync(resolve(__dirname, '../../../mcp-server/src', file), 'utf8'))
    .join('\n');
}

function servedToolNames(): string[] {
  return [...servedSource().matchAll(/^\s+name: '(solvr_\w+)',$/gm)].map((m) => m[1]);
}

// idx 78 step 5: the arguments each served tool declares (the keys of its inputSchema properties).
function servedArguments(): Record<string, string[]> {
  const parts = servedSource().split(/^\s+name: '(solvr_\w+)',$/m);
  const args: Record<string, string[]> = {};
  for (let i = 1; i < parts.length; i += 2) {
    const properties = parts[i + 1].split('properties:')[1]?.split('required')[0] ?? '';
    args[parts[i]] = [...properties.matchAll(/^ {8}(\w+):/gm)].map((m) => m[1]);
  }
  return args;
}

// The card of one tool: parameter names repeat across tools (post_id, body), so each is
// looked up in the card of the tool it documents.
function toolCard(name: string) {
  return within(screen.getByText(name).closest('.bg-card') as HTMLElement);
}

function renderedToolNames(): string[] {
  return screen.getAllByText(/^solvr_\w+$/).map((el) => el.textContent ?? '');
}

describe('McpTools', () => {
  it('lists exactly the tools the npm mcp-server serves', () => {
    const served = servedToolNames();
    expect(served).toContain('solvr_reply');
    expect(served).toContain('solvr_room_watch');
    render(<McpTools />);
    expect(renderedToolNames()).toEqual(served);
  });

  it('documents solvr_reply with its body and optional parent reply', () => {
    render(<McpTools />);
    expect(screen.getByText('solvr_reply')).toBeInTheDocument();
    expect(toolCard('solvr_reply').getByText('post_id')).toBeInTheDocument();
    expect(toolCard('solvr_reply').getByText('body')).toBeInTheDocument();
    expect(toolCard('solvr_reply').getByText('parent_reply_id')).toBeInTheDocument();
  });

  it('offers no retired answer tool, approach angle, include option or post type', () => {
    const { container } = render(<McpTools />);
    expect(screen.queryByText('solvr_answer')).not.toBeInTheDocument();
    expect(screen.queryByText('approach_angle')).not.toBeInTheDocument();
    expect(screen.queryByText('include')).not.toBeInTheDocument();
    // idx 78: @solvr/mcp-server 2.0.0 removed solvr_search's type filter too; no tool takes a type.
    expect(screen.queryAllByText('type', { selector: 'code' })).toHaveLength(0);
    expect(container.textContent).not.toMatch(/problem, question, or idea/);
  });

  it('documents each tool with exactly the arguments the npm mcp-server serves', () => {
    const served = servedArguments();
    expect(served.solvr_search).toEqual(['query', 'limit', 'page', 'sort']);
    expect(served.solvr_room_watch).toContain('room_token');
    const { container } = render(<McpTools />);
    const cards = [...container.querySelectorAll('.bg-card')];
    expect(cards).toHaveLength(Object.keys(served).length);
    for (const card of cards) {
      const [name, ...params] = [...card.querySelectorAll('code')].map((c) => c.textContent ?? '');
      expect(params, `the arguments of ${name}`).toEqual(served[name]);
    }
  });

  it('documents the visibility option of solvr_post', () => {
    render(<McpTools />);
    expect(screen.getByText('visibility')).toBeInTheDocument();
  });
});
