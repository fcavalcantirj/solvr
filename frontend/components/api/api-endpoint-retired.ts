import { Endpoint } from "./api-endpoint-types";

// retiredEndpoint documents a legacy route retired at the knowledge-model cutover with the
// answer the API gives every caller (backend/internal/api/legacy_write_retirement.go and
// legacy_read_retirement.go): 410 ENDPOINT_RETIRED, the same message, and the canonical
// replacement. `route` is the "METHOD /v1/path" of SPEC.md 26.6 (writes) or 26.7 (reads);
// `path` is how this page writes it (without /v1).
export function retiredEndpoint(
  method: Endpoint["method"],
  path: string,
  route: string,
  replacement: string | null,
  migration: string,
): Endpoint {
  const message = replacement
    ? `${route} was retired with the canonical knowledge model; use ${replacement} instead.`
    : `${route} was retired with the canonical knowledge model and has no canonical equivalent.`;
  return {
    method,
    path,
    description: replacement ? `Retired: use ${replacement} instead` : "Retired: no canonical equivalent",
    auth: "none",
    response: `// 410 Gone, for every caller; nothing is read or written
{
  "error": {
    "code": "ENDPOINT_RETIRED",
    "message": "${message}",
    "details": {
      "retired_route": "${route}",
      "replacement": ${replacement ? `"${replacement}"` : "null"},
      "instructions": "..."
    }
  }
}`,
    retired: { replacement, migration },
  };
}
