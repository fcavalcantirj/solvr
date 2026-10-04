import { render, screen, within } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { readFileSync } from 'fs';
import { resolve } from 'path';
import { McpTools } from './mcp-tools';

// idx 52 step 2: the hosted MCP server creates canonical posts with no type and replies with
// solvr_reply (its solvr_answer tool was removed). The /mcp page documents that server
// (POST /v1/mcp), so it must list exactly the tools backend/internal/api/handlers/mcp.go
// serves and none of the retired choices.
function servedSource(): string {
  return readFileSync(resolve(__dirname, '../../../backend/internal/api/handlers/mcp.go'), 'utf8');
}

const TOOL_NAME = /^\t\t"name":\s+"(solvr_\w+)",$/m;

function servedToolNames(): string[] {
  return [...servedSource().matchAll(new RegExp(TOOL_NAME.source, 'gm'))].map((m) => m[1]);
}

// idx 78 step 5: the arguments each served tool declares (the keys of its inputSchema properties).
function servedArguments(): Record<string, string[]> {
  const parts = servedSource().split(TOOL_NAME);
  const args: Record<string, string[]> = {};
  for (let i = 1; i < parts.length; i += 2) {
    const schema = parts[i + 1].split('mcpSchema(')[1]?.split('\n\t\t}')[0] ?? '';
    args[parts[i]] = [...schema.matchAll(/^\t\t\t"(\w+)":/gm)].map((m) => m[1]);
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
  it('lists exactly the tools the hosted MCP server serves', () => {
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
    // idx 78: /v1/mcp 2.0.0 removed solvr_search's type filter too; no tool takes a type.
    expect(screen.queryAllByText('type', { selector: 'code' })).toHaveLength(0);
    expect(container.textContent).not.toMatch(/problem, question, or idea/);
  });

  it('documents each tool with exactly the arguments the hosted MCP server serves', () => {
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

  it('documents solvr_post with a title, a description and tags, and no claim tool', () => {
    render(<McpTools />);
    expect(screen.queryByText('solvr_claim')).not.toBeInTheDocument();
    expect(screen.queryByText('visibility')).not.toBeInTheDocument();
    for (const param of ['title', 'description', 'tags']) {
      expect(toolCard('solvr_post').getByText(param)).toBeInTheDocument();
    }
  });
});
