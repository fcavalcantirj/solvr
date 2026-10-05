import { describe, it, expect } from "vitest";
import { readFileSync } from "fs";
import { resolve } from "path";
import { endpointGroups } from "./api-endpoint-data";
import { roomEndpointGroups } from "./api-endpoint-data-rooms";

// Rooms are what Solvr is for: /api-docs documents the room API, held to the shared contract
// (contract/openapi-examples.json, the recorded requests and answers every client is tested
// against) so the page cannot drift from what the API serves.

interface ContractOperation {
  operation_id: string;
  method: string;
  path: string;
  request_body: Record<string, unknown> | null;
}

const contract: { operations: ContractOperation[] } = JSON.parse(
  readFileSync(resolve(__dirname, "../../../contract/openapi-examples.json"), "utf8"),
);
const roomOperations = contract.operations.filter((op) => op.path.startsWith("/v1/rooms"));
const rooms = roomEndpointGroups.find((g) => g.name === "Rooms");
const find = (method: string, path: string) => rooms?.endpoints.find((e) => e.method === method && e.path === path);

describe("/api-docs Rooms group", () => {
  it("is part of the page's endpoint groups, right after search", () => {
    expect(rooms).toBeDefined();
    const names = endpointGroups.map((g) => g.name);
    expect(names).toContain("Rooms");
    expect(names.indexOf("Rooms")).toBeLessThan(names.indexOf("Posts"));
  });

  it("documents every room operation of the contract", () => {
    expect(roomOperations.map((op) => op.operation_id).sort()).toEqual([
      "addRoomMember", "createRoom", "createRoomEntry", "createRoomStreamTicket",
      "handshakeRoom", "listRoomEntries", "listRoomMembers", "streamRoom",
    ]);
    for (const op of roomOperations) {
      expect(find(op.method, op.path.replace(/^\/v1/, "")), `${op.method} ${op.path}`).toBeDefined();
    }
  });

  it("names every field the contract's requests send", () => {
    for (const op of roomOperations) {
      const endpoint = find(op.method, op.path.replace(/^\/v1/, ""))!;
      const params = (endpoint.params ?? []).map((p) => p.name);
      for (const field of Object.keys(op.request_body ?? {})) {
        expect(params, `${op.operation_id}: ${field}`).toContain(field);
      }
    }
  });

  it("says which credential each room call takes", () => {
    // The room is created and joined with the agent API key; reading, sending and watching
    // present the room token the handshake returned.
    expect(find("POST", "/rooms")!.auth).toBe("both");
    expect(find("POST", "/rooms/{slug}/handshake")!.auth).toBe("api_key");
    for (const [method, path] of [["POST", "/rooms/{slug}/entries"], ["POST", "/rooms/{slug}/stream-ticket"]]) {
      expect(find(method, path)!.description, path).toMatch(/room token/i);
    }
    expect(find("GET", "/rooms/{slug}/entries")!.description).toMatch(/public room.*without/i);
  });
});
