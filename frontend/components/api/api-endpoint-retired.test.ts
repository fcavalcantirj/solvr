import { describe, it, expect } from "vitest";
import { readFileSync } from "fs";
import { resolve } from "path";
import { endpointGroups } from "./api-endpoint-data";
import { Endpoint } from "./api-endpoint-types";

// idx 52 step 4: the /api-docs page documents every retired legacy write with its canonical
// replacement. The source is SPEC.md 26.6, whose table the backend test
// TestLegacyWriteRetirements_PublishedInSpecMigrationNotes pins to the table the router mounts.

interface SpecRow {
  route: string;
  replacement: string | null;
  migration: string;
}

function specRows(): SpecRow[] {
  const spec = readFileSync(resolve(__dirname, "../../../SPEC.md"), "utf8");
  const start = spec.indexOf("## 26.6 Retired Legacy Writes");
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
    const retired = new Set(specRows().map((r) => routeKey(...(r.route.split(" ") as [string, string]))));
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
