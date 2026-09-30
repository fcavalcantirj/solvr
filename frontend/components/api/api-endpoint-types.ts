export interface Param {
  name: string;
  type: string;
  required: boolean;
  description: string;
}

// A legacy write retired at the knowledge-model cutover (SPEC.md 26.6): it answers every
// caller 410 ENDPOINT_RETIRED and names the canonical route to use instead.
export interface Retirement {
  // Canonical "METHOD /v1/path" to call instead; null when there is no canonical equivalent.
  replacement: string | null;
  // The old request shape and the canonical one, as SPEC.md 26.6 states it.
  migration: string;
}

export interface Endpoint {
  method: "GET" | "POST" | "PATCH" | "DELETE";
  path: string;
  description: string;
  auth?: "jwt" | "api_key" | "both" | "none";
  params?: Param[];
  response: string;
  retired?: Retirement;
}

export interface EndpointGroup {
  name: string;
  description: string;
  endpoints: Endpoint[];
}
