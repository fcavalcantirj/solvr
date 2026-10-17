// serverFetch is fetch for the site's own server reads of the API: it names the sender, so the
// API's request log counts them as the web server's (ua_class web_server, SPEC.md 27.6) and not
// as an anonymous client's. Everything else about the request is the caller's: cache, signal,
// headers. In a browser it sets nothing, since the browser sends its own User-Agent.

export const SERVER_USER_AGENT = 'solvr-web/1.0 (ssr)';

export function serverFetch(input: string, init?: RequestInit): Promise<Response> {
  if (typeof window !== 'undefined') return fetch(input, init);
  const headers = new Headers(init?.headers);
  headers.set('User-Agent', SERVER_USER_AGENT);
  return fetch(input, { ...init, headers });
}
