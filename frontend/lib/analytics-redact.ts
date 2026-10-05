// What may leave the browser inside an event parameter (SPEC.md 27.7). Pure functions.

// The names an event parameter may have. A closed set: a typo is a type error, and a name
// outside it is dropped at run time.
export const TRACK_PARAM_NAMES = [
  'method',
  'surface',
  'preset',
  'role',
  'item',
  'location',
  'list',
  'sort',
  'page',
  'results',
  'search_term',
  'direction',
  'visibility',
  'status',
] as const;

export type TrackParamName = (typeof TRACK_PARAM_NAMES)[number];
export type TrackParams = Partial<Record<TrackParamName, string | number | boolean | undefined>>;
export type CleanParams = Record<string, string | number | boolean>;

export const MAX_PARAM_LENGTH = 100;
const REDACTED = '[redacted]';

// Order matters: a bearer token goes as a whole, whatever it is made of.
const SECRETS: RegExp[] = [
  // "Bearer <token>"
  /\bBearer\s+[A-Za-z0-9._~+/=-]+/gi,
  // A JWT, signed or not: base64url parts joined by dots, the first one starting "eyJ".
  /\beyJ[A-Za-z0-9_-]{8,}(?:\.[A-Za-z0-9_-]*){1,2}/g,
  // A Solvr key: agent (solvr_…) or user (solvr_sk_…).
  /solvr_[A-Za-z0-9_-]+/g,
  // An e-mail address.
  /[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}/g,
];

/** Strip e-mail addresses, Solvr keys, bearer tokens and JWTs, then cut to 100 characters. */
export function redact(value: string): string {
  let out = value;
  for (const secret of SECRETS) out = out.replace(secret, REDACTED);
  // Stripped first, cut second: cutting first could leave half a secret that no longer
  // looks like one. Cut by character, so none is split in two.
  const characters = Array.from(out);
  return characters.length > MAX_PARAM_LENGTH ? characters.slice(0, MAX_PARAM_LENGTH).join('') : out;
}

/** The parameters as they may be sent: known names only, no undefined, strings stripped and cut. */
export function cleanParams(params: TrackParams | undefined): CleanParams {
  const clean: CleanParams = {};
  if (!params) return clean;
  for (const name of TRACK_PARAM_NAMES) {
    const value = params[name];
    if (typeof value === 'string') clean[name] = redact(value);
    else if (typeof value === 'number' || typeof value === 'boolean') clean[name] = value;
  }
  return clean;
}
