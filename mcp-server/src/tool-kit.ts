/**
 * What every tool shares: the tool definition and result shapes, argument readers, and how a
 * failure is reported.
 */

import { ApiError } from './errors.js';

export interface ToolDefinition {
  name: string;
  description: string;
  inputSchema: {
    type: 'object';
    properties: Record<string, {
      type: string;
      description: string;
      enum?: string[];
      items?: { type: string };
      default?: unknown;
    }>;
    required?: string[];
  };
}

export interface ToolManifest {
  tools: ToolDefinition[];
}

export interface ToolResult {
  content: Array<{
    type: 'text';
    text: string;
  }>;
  isError?: boolean;
}

/** A call the tool refuses before sending any request (a missing argument, no room token). */
export class ToolInputError extends Error {}

export function textResult(lines: string[], isError?: boolean): ToolResult {
  const result: ToolResult = { content: [{ type: 'text', text: lines.join('\n') }] };
  if (isError) result.isError = true;
  return result;
}

/** The text of a failed call: the error, and the request id the API gave it. */
export function failureText(tool: string, error: unknown): string {
  const message = error instanceof Error ? error.message : 'Unknown error';
  const lines = [`Error executing ${tool}: ${message}`];
  if (error instanceof ApiError && error.requestId) {
    lines.push(`request id: ${error.requestId}`);
  }
  return lines.join('\n');
}

export function requireString(args: Record<string, unknown>, name: string): string {
  const value = args[name];
  if (typeof value !== 'string' || value === '') {
    throw new ToolInputError(`${name} is required`);
  }
  return value;
}

/** The argument as a string; absent when it was not given or is empty. */
export function optionalString(args: Record<string, unknown>, name: string): string | undefined {
  const value = args[name];
  if (value === undefined || value === null || value === '') return undefined;
  return String(value);
}

/** The argument as a number; absent when it was not given. */
export function optionalNumber(args: Record<string, unknown>, name: string): number | undefined {
  const value = args[name];
  if (value === undefined || value === null || value === '') return undefined;
  const number = Number(value);
  if (!Number.isFinite(number)) {
    throw new ToolInputError(`${name} must be a number`);
  }
  return number;
}

/** The argument as a list of strings (an array, or a comma-separated string); absent when not given. */
export function optionalList(args: Record<string, unknown>, name: string): string[] | undefined {
  const value = args[name];
  if (value === undefined || value === null) return undefined;
  const items = Array.isArray(value) ? value.map(String) : String(value).split(',');
  return items.map((item) => item.trim()).filter((item) => item !== '');
}
