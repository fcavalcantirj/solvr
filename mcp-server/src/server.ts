/**
 * The MCP JSON-RPC handler: answers one request with the given tools.
 */

import { SolvrTools } from './tools.js';
import { VERSION } from './version.js';

export interface MCPRequest {
  jsonrpc: '2.0';
  id: number | string;
  method: string;
  params?: Record<string, unknown>;
}

export interface MCPResponse {
  jsonrpc: '2.0';
  id: number | string;
  result?: unknown;
  error?: {
    code: number;
    message: string;
  };
}

export const SERVER_INFO = {
  name: 'solvr',
  version: VERSION,
  protocolVersion: '2024-11-05',
};

export async function handleRequest(request: MCPRequest, tools: SolvrTools): Promise<MCPResponse> {
  const { id, method, params } = request;

  switch (method) {
    case 'initialize':
      return {
        jsonrpc: '2.0',
        id,
        result: {
          ...SERVER_INFO,
          capabilities: {
            tools: {},
          },
        },
      };

    case 'initialized':
      // Client notification, no response needed but we return empty result
      return {
        jsonrpc: '2.0',
        id,
        result: {},
      };

    case 'tools/list':
      return {
        jsonrpc: '2.0',
        id,
        result: tools.getManifest(),
      };

    case 'tools/call': {
      const toolName = params?.name as string;
      const toolArgs = (params?.arguments || {}) as Record<string, unknown>;

      if (!toolName) {
        return {
          jsonrpc: '2.0',
          id,
          error: { code: -32602, message: 'Missing tool name' },
        };
      }

      const result = await tools.executeTool(toolName, toolArgs);
      return {
        jsonrpc: '2.0',
        id,
        result,
      };
    }

    case 'shutdown':
      return {
        jsonrpc: '2.0',
        id,
        result: null,
      };

    default:
      return {
        jsonrpc: '2.0',
        id,
        error: { code: -32601, message: `Method not found: ${method}` },
      };
  }
}
