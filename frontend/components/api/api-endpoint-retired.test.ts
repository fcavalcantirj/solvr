import { describe, it, expect } from "vitest";
import { readFileSync } from "fs";
import { resolve } from "path";
import { endpointGroups } from "./api-endpoint-data";
import { Endpoint } from "./api-endpoint-types";

// idx 52 step 4: the /api-docs page documents every retired legacy write with its canonical
// replacement. The source is SPEC.md 26.6, whose table the backend test
// TestLegacyWriteRetirements_PublishedInSpecMigrationNotes pins to the table the router mounts.
// idx 73 step 5: the same for the retired legacy reads the page documented, from SPEC.md 26.7
// (pinned by TestLegacyReadRetirements_PublishedInSpecMigrationNotes).

interface SpecRow {
  route: string;
  replacement: string | null;
  migration: string;
}

function specRows(heading = "## 26.6 Retired Legacy Writes"): SpecRow[] {
  const spec = readFileSync(resolve(__dirname, "../../../SPEC.md"), "utf8");
  const start = spec.indexOf(heading);
  expect(start).toBeGreaterThanOrEqual(0);
  const rest = spec.slice(start + 1);
  const section = rest.slice(0, rest.search(/\n(## |---)/));
  const rows: SpecRow[] = [];
  for (const line of section.split("\n")) {
    const m = line.match(/^\| `([A-Z]+ \/v1\/[^`]+)` \| (none|`[^`]+`) \| (.+) \|$/);
    if (!m) continue;
    rows.push({
      route: m[1],
      replacement: m[2] === "none" ? null : m[2].slice(1, -1),
      migration: m[3].replace(/`/g, ""),
    });
  }
  return rows;
}

// "POST /v1/questions/{id}/accept/{aid}" and the page's "/questions/{id}/accept/{answerId}"
// name the same route: compare without the /v1 prefix and without path-variable names.
function routeKey(method: string, path: string): string {
  return `${method} ${path.replace(/^\/v1/, "").replace(/\{[^}]+\}/g, "{}")}`;
}

function allEndpoints(): Endpoint[] {
  return endpointGroups.flatMap((g) => g.endpoints);
}

function pageEndpoint(route: string): Endpoint | undefined {
  const [method, path] = route.split(" ");
  return allEndpoints().find((e) => routeKey(e.method, e.path) === routeKey(method, path));
}

const READS = "## 26.7 Retired Legacy Reads";

describe("retired legacy writes on the API docs page", () => {
  it("reads all 19 retired routes from SPEC.md 26.6", () => {
    expect(specRows()).toHaveLength(19);
  });

  it("marks every retired route as retired with the SPEC replacement and migration", () => {
    for (const row of specRows()) {
      const ep = pageEndpoint(row.route);
      expect(ep, `${row.route} is not on the page`).toBeDefined();
      expect(ep!.retired, `${row.route} is not marked retired`).toEqual({
        replacement: row.replacement,
        migration: row.migration,
      });
      expect(ep!.auth, `${row.route} reads no credential`).toBe("none");
      expect(ep!.response).toContain('"code": "ENDPOINT_RETIRED"');
      expect(ep!.response).toContain(`"retired_route": "${row.route}"`);
      const message = row.replacement
        ? `${row.route} was retired with the canonical knowledge model; use ${row.replacement} instead.`
        : `${row.route} was retired with the canonical knowledge model and has no canonical equivalent.`;
      expect(ep!.response).toContain(message);
    }
  });

  it("marks nothing else as retired", () => {
    const retired = new Set(
      [...specRows(), ...specRows(READS)].map((r) => routeKey(...(r.route.split(" ") as [string, string]))),
    );
    for (const ep of allEndpoints()) {
      if (ep.retired) {
        expect(retired.has(routeKey(ep.method, ep.path)), `${ep.method} ${ep.path}`).toBe(true);
      }
    }
  });

  it("documents every canonical replacement as a live endpoint on the same page", () => {
    for (const row of specRows()) {
      if (!row.replacement) continue;
      const ep = pageEndpoint(row.replacement);
      expect(ep, `${row.replacement} (replacement of ${row.route}) is not documented`).toBeDefined();
      expect(ep!.retired, `${row.replacement} is itself retired`).toBeUndefined();
      expect(ep!.auth).toBe("both");
    }
  });

  it("documents POST /posts without a type and replies with body and parent_reply_id", () => {
    const create = pageEndpoint("POST /v1/posts")!;
    expect(create.params!.map((p) => p.name)).not.toContain("type");
    const reply = pageEndpoint("POST /v1/posts/{id}/replies")!;
    expect(reply.params!.map((p) => p.name)).toEqual(expect.arrayContaining(["body", "parent_reply_id"]));
  });
});

describe("retired legacy reads on the API docs page", () => {
  const feed = ["GET /v1/feed", "GET /v1/feed/stuck", "GET /v1/feed/unanswered"];
  const typedLists = ["GET /v1/problems", "GET /v1/questions", "GET /v1/ideas"];
  const commentLists = [
    "GET /v1/posts/{id}/comments",
    "GET /v1/approaches/{id}/comments",
    "GET /v1/answers/{id}/comments",
    "GET /v1/responses/{id}/comments",
  ];
  const typedReads = [
    "GET /v1/problems/{id}",
    "GET /v1/questions/{id}",
    "GET /v1/ideas/{id}",
    "GET /v1/problems/{id}/approaches",
    "GET /v1/problems/{id}/approaches/{approachId}/history",
    "GET /v1/problems/{id}/export",
    "GET /v1/questions/{id}/answers",
    "GET /v1/ideas/{id}/responses",
  ];
  const contributionLists = ["GET /v1/problems/{id}/approaches", "GET /v1/questions/{id}/answers", "GET /v1/ideas/{id}/responses"];
  const contributionListings = ["GET /v1/users/{id}/contributions", "GET /v1/me/contributions"];

  it("reads all 23 retired reads from SPEC.md 26.7", () => {
    expect(specRows(READS)).toHaveLength(23);
  });

  it("still documents the retired feed routes, typed lists, comment lists, typed reads and GET /me/contributions, so old callers find the migration", () => {
    for (const route of [...feed, ...typedLists, ...commentLists, ...typedReads, "GET /v1/me/contributions"]) {
      expect(pageEndpoint(route), `${route} is not on the page`).toBeDefined();
    }
  });

  it("marks every retired read on the page as retired with the SPEC replacement and instructions", () => {
    const onPage = specRows(READS).filter((row) => pageEndpoint(row.route));
    expect(onPage.map((row) => row.route)).toEqual(
      expect.arrayContaining([...feed, ...typedLists, ...commentLists, ...typedReads, "GET /v1/me/contributions"]),
    );
    for (const row of onPage) {
      const ep = pageEndpoint(row.route)!;
      expect(ep.retired, `${row.route} is not marked retired`).toEqual({
        replacement: row.replacement,
        migration: row.migration,
      });
      expect(ep.auth, `${row.route} reads no credential`).toBe("none");
      expect(ep.params ?? [], `${row.route} offers no parameters it does not read`).toEqual([]);
      expect(ep.response).toContain('"code": "ENDPOINT_RETIRED"');
      expect(ep.response).toContain(`"retired_route": "${row.route}"`);
      expect(ep.response).toContain(
        `${row.route} was retired with the canonical knowledge model; use ${row.replacement} instead.`,
      );
    }
  });

  it("documents GET /posts/{id}/replies, the comment lists' replacement, with the paging the instructions name", () => {
    const list = pageEndpoint("GET /v1/posts/{id}/replies");
    expect(list, "GET /posts/{id}/replies is not documented").toBeDefined();
    expect(list!.retired).toBeUndefined();
    expect(list!.params!.map((p) => p.name)).toEqual(expect.arrayContaining(["id", "cursor", "limit"]));
    for (const row of specRows(READS).filter((r) => commentLists.includes(r.route))) {
      expect(row.replacement).toBe("GET /v1/posts/{id}/replies");
      expect(row.migration).toContain("limit (default 50, at most 100) and cursor");
    }
  });

  it("documents GET /posts/{id} and GET /posts/{id}/replies, the typed reads' replacements, as live endpoints", () => {
    for (const row of specRows(READS).filter((r) => typedReads.includes(r.route))) {
      expect(["GET /v1/posts/{id}", "GET /v1/posts/{id}/replies"]).toContain(row.replacement);
      const ep = pageEndpoint(row.replacement!);
      expect(ep, `${row.replacement} (replacement of ${row.route}) is not documented`).toBeDefined();
      expect(ep!.retired, `${row.replacement} is itself retired`).toBeUndefined();
    }
    for (const row of specRows(READS).filter((r) => contributionLists.includes(r.route))) {
      expect(row.migration).toContain("limit (default 50, at most 100) and cursor");
    }
  });

  it("documents GET /posts, the feed and typed list replacement, with the filters the instructions name", () => {
    const list = pageEndpoint("GET /v1/posts");
    expect(list, "GET /posts is not documented").toBeDefined();
    expect(list!.retired).toBeUndefined();
    expect(list!.params!.map((p) => p.name)).toEqual(
      expect.arrayContaining(["type", "status", "needs_help", "has_answer", "sort", "tags", "page", "per_page"]),
    );
  });

  it("documents GET /replies, the contribution listings' replacement, with the author filter and paging the instructions name", () => {
    const list = pageEndpoint("GET /v1/replies");
    expect(list, "GET /replies is not documented").toBeDefined();
    expect(list!.retired).toBeUndefined();
    expect(list!.params!.map((p) => p.name)).toEqual(
      expect.arrayContaining(["author_type", "author_id", "cursor", "limit"]),
    );
    const rows = specRows(READS).filter((r) => contributionListings.includes(r.route));
    expect(rows).toHaveLength(2);
    for (const row of rows) {
      expect(row.replacement).toBe("GET /v1/replies");
      expect(row.migration).toContain("GET /v1/replies?author_type=");
      expect(row.migration).toContain("limit (default 50, at most 100) and cursor");
    }
  });
});
