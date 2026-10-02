/**
 * 2.0.0 removed the tools and arguments of the legacy knowledge model. A 1.x call that still
 * uses one is refused before any request, naming what replaces it, instead of failing as an
 * unknown tool or running without the choice it asked for.
 */

import { VERSION } from './version.js';

/** The removed tools, and what replaces each. */
const REMOVED_TOOLS: Record<string, string> = {
  solvr_answer: 'use solvr_reply with post_id and body: answers and approaches are replies',
};

/** The removed arguments of each tool, and what replaces each. */
const REMOVED_ARGUMENTS: Record<string, Record<string, string>> = {
  solvr_search: { type: 'search covers every post' },
  solvr_post: { type: 'a post has no type' },
  solvr_get: {
    include: 'solvr_get shows the post with its first replies and solvr_replies pages through all of them: answers and approaches from before the change are replies there',
  },
};

function removedText(name: string, instead: string): string {
  return `${name} was removed in @solvr/mcp-server ${VERSION}; ${instead}. See "Migrating from 1.x to ${VERSION}" in the @solvr/mcp-server README.`;
}

/**
 * Why a call of tool with args uses a removed tool or argument, or undefined when it uses
 * none. An undefined or null argument is not given.
 */
export function removedChoice(tool: string, args: Record<string, unknown>): string | undefined {
  if (tool in REMOVED_TOOLS) {
    return removedText(`'${tool}'`, REMOVED_TOOLS[tool]);
  }
  for (const [argument, instead] of Object.entries(REMOVED_ARGUMENTS[tool] ?? {})) {
    const value = args[argument];
    if (value !== undefined && value !== null) {
      return removedText(`The '${argument}' argument of ${tool}`, `${instead} (it was given ${JSON.stringify(value)})`);
    }
  }
  return undefined;
}
