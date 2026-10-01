#!/usr/bin/env node
/**
 * Solvr MCP Server
 *
 * Model Context Protocol server for integrating Solvr with AI coding tools
 * like Claude Code, Cursor, and others.
 *
 * Usage:
 *   SOLVR_API_KEY=your_key solvr-mcp-server
 *
 * Or configure in your MCP settings:
 *   {
 *     "mcpServers": {
 *       "solvr": {
 *         "command": "solvr-mcp-server",
 *         "env": { "SOLVR_API_KEY": "your_key" }
 *       }
 *     }
 *   }
 */

import { loadConfig } from './config.js';
import { SolvrTools } from './tools.js';
import { handleRequest } from './server.js';
import type { MCPRequest, MCPResponse } from './server.js';

async function main() {
  let config: ReturnType<typeof loadConfig>;
  let tools: SolvrTools;

  try {
    config = loadConfig();
    tools = new SolvrTools(config.apiKey, config.apiUrl);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'Configuration error';
    console.error(`Error: ${message}`);
    process.exit(1);
  }

  // Set up stdio transport
  process.stdin.setEncoding('utf8');

  let buffer = '';

  process.stdin.on('data', async (chunk: string) => {
    buffer += chunk;

    // Process complete JSON-RPC messages
    const lines = buffer.split('\n');
    buffer = lines.pop() || '';

    for (const line of lines) {
      if (!line.trim()) continue;

      try {
        const request: MCPRequest = JSON.parse(line);
        const response = await handleRequest(request, tools);
        sendResponse(response);
      } catch (error) {
        const message = error instanceof Error ? error.message : 'Parse error';
        sendResponse({
          jsonrpc: '2.0',
          id: null as unknown as number,
          error: { code: -32700, message: `Parse error: ${message}` },
        });
      }
    }
  });

  process.stdin.on('end', () => {
    process.exit(0);
  });

  // Handle uncaught errors gracefully
  process.on('uncaughtException', (error) => {
    console.error('Uncaught exception:', error);
  });

  process.on('unhandledRejection', (error) => {
    console.error('Unhandled rejection:', error);
  });
}

function sendResponse(response: MCPResponse): void {
  process.stdout.write(JSON.stringify(response) + '\n');
}

main().catch((error) => {
  console.error('Fatal error:', error);
  process.exit(1);
});
