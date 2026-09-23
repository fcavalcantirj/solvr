import { describe, it, expect } from "vitest";
import {
  mergeMessages,
  isNearBottom,
  isLongMessage,
  LONG_MESSAGE_CHARS,
} from "./message-view";
import type { APIRoomMessage } from "@/lib/api-types";

function msg(id: number, content = `m${id}`): APIRoomMessage {
  return {
    id,
    room_id: "room-1",
    author_type: "agent",
    agent_name: `agent-${id}`,
    content,
    content_type: "text",
    metadata: {},
    created_at: new Date("2026-04-15T20:00:00Z").toISOString(),
  };
}

describe("mergeMessages", () => {
  it("orders messages oldest-first by server id regardless of input order", () => {
    const out = mergeMessages([msg(3), msg(1), msg(2)]);
    expect(out.map((m) => m.id)).toEqual([1, 2, 3]);
  });

  it("de-duplicates by persistent id, keeping the first occurrence", () => {
    const first = { ...msg(1), content: "original" };
    const replay = { ...msg(1), content: "replayed copy" };
    const out = mergeMessages([first, replay, msg(2)]);
    expect(out.map((m) => m.id)).toEqual([1, 2]);
    expect(out[0].content).toBe("original");
  });

  it("merges several batches (older history + live + deep-linked) with no gaps or duplicates", () => {
    const older = [msg(1), msg(2)];
    const live = [msg(4), msg(3)];
    const deepLinked = [msg(2), msg(5)]; // msg(2) overlaps older
    const out = mergeMessages(older, live, deepLinked);
    expect(out.map((m) => m.id)).toEqual([1, 2, 3, 4, 5]);
  });

  it("returns an empty array for no input", () => {
    expect(mergeMessages([])).toEqual([]);
  });
});

describe("isNearBottom", () => {
  it("is true when the viewport bottom is within the threshold of the content bottom", () => {
    expect(isNearBottom({ scrollTop: 700, scrollHeight: 1000, clientHeight: 300 })).toBe(true);
  });

  it("is false when the reader has scrolled up into earlier history", () => {
    expect(isNearBottom({ scrollTop: 0, scrollHeight: 2000, clientHeight: 300 })).toBe(false);
  });

  it("honors a custom threshold", () => {
    expect(isNearBottom({ scrollTop: 600, scrollHeight: 1000, clientHeight: 300 }, 200)).toBe(true);
    expect(isNearBottom({ scrollTop: 600, scrollHeight: 1000, clientHeight: 300 }, 50)).toBe(false);
  });
});

describe("isLongMessage", () => {
  it("flags content longer than the collapse threshold", () => {
    expect(isLongMessage("x".repeat(LONG_MESSAGE_CHARS + 1))).toBe(true);
  });

  it("does not flag short content", () => {
    expect(isLongMessage("short")).toBe(false);
  });
});
