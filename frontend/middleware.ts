import { NextResponse } from 'next/server'
import type { NextFetchEvent, NextRequest } from 'next/server'

const BLOCKED_PATHS = ['/adfa']

// The skill, as the sentence of GET /v1/connect links it: /skill.md?f=<flow code>
// (SPEC.md 25.6). A GET of that link is reported to the API as the funnel step
// skill_fetched (25.7), so a website visit can be tied to the room it produced.
const SKILL_PATH = '/skill.md'
const SKILL_FLOW_PARAM = 'f'
// Longer than any flow id the API stores; a longer value is not worth a request.
const SKILL_FLOW_MAX_LENGTH = 64
// The report is abandoned after this long. It never holds the response either way.
const SKILL_REPORT_TIMEOUT_MS = 3000

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev'
// The report names its sender, so the API host and the edge in front of it can tell it
// from an anonymous client (the runtime would otherwise send its own default).
const SKILL_REPORT_USER_AGENT = 'solvr-web/1.0 (skill-fetch report)'

// reportSkillFetched tells the API that the skill link was fetched with a flow code, and
// how (the request's Sec-Fetch-Mode: the API uses it to tell a person's browser from an
// agent, and decides everything else too). The request starts at once, beside the
// response. The promise it returns settles when the API answered, failed or the timeout
// passed, and it never rejects: a statistic must not break the page it measures.
function reportSkillFetched(flowId: string, requestMode: string): Promise<void> {
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), SKILL_REPORT_TIMEOUT_MS)
  let sent: Promise<unknown>
  try {
    sent = fetch(`${API_BASE_URL}/v1/analytics/funnel`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'User-Agent': SKILL_REPORT_USER_AGENT },
      body: JSON.stringify({ event: 'skill_fetched', flow_id: flowId, request_mode: requestMode }),
      signal: controller.signal,
    })
  } catch {
    sent = Promise.resolve()
  }
  // The answer is not read; its body is released so the connection is free again.
  return sent
    .then((answer) => (answer as Response | undefined)?.body?.cancel())
    .then(
      () => undefined,
      () => undefined,
    )
    .finally(() => clearTimeout(timer))
}

// Legacy first path segments that now live under the unified /posts collection.
// The pre-redesign product split knowledge into /problems, /ideas and
// /questions collections (plus the /feed aggregate). All of that is now one
// canonical /posts collection, so those routes redirect permanently.
// These redirects are permanent and outlive the API's compatibility sunset (task idx 83):
// they are decided from the path alone and never ask the API.
const LEGACY_SEGMENTS = ['problems', 'ideas', 'questions']

// canonicalPath returns the canonical /posts destination for a legacy route, or
// null when the path is already canonical or unrelated. It replaces only the
// first path segment, so the id / new / edit sub-path is preserved verbatim:
//   /problems              -> /posts
//   /problems/{id}         -> /posts/{id}
//   /problems/{id}/edit    -> /posts/{id}/edit
//   /problems/new          -> /posts/new
//   /feed                  -> /posts   (the aggregate has no sub-routes)
//   /new                   -> /posts/new (the old typed composer; the canonical
//                                         one asks for no type)
// It never rewrites a /posts path, so a legacy -> canonical redirect can never
// bounce back into a second redirect (no loops or chains).
export function canonicalPath(pathname: string): string | null {
  if (pathname === '/feed' || pathname.startsWith('/feed/')) {
    return '/posts'
  }
  if (pathname === '/new') {
    return '/posts/new'
  }
  const segment = pathname.split('/')[1]
  if (LEGACY_SEGMENTS.includes(segment)) {
    return '/posts' + pathname.slice(segment.length + 1)
  }
  return null
}

export function middleware(request: NextRequest, event?: NextFetchEvent) {
  const { pathname } = request.nextUrl

  if (BLOCKED_PATHS.some((p) => pathname === p || pathname.startsWith(p + '/'))) {
    return new NextResponse(null, { status: 404 })
  }

  const canonical = canonicalPath(pathname)
  if (canonical) {
    // Preserve the query string (search, tag, pagination, sort). Obsolete
    // type-specific filters that the canonical /posts page does not read are
    // simply ignored there, giving deterministic defaults.
    const url = request.nextUrl.clone()
    url.pathname = canonical
    // 308 = permanent redirect that preserves the request method.
    return NextResponse.redirect(url, 308)
  }

  // The skill link with a flow code: report the fetch and let the request go on to the
  // file. Nothing is awaited here; waitUntil only lets the report outlive the response.
  // The code is not judged: the API validates it.
  if (pathname === SKILL_PATH && request.method === 'GET') {
    const flowId = request.nextUrl.searchParams.get(SKILL_FLOW_PARAM)
    if (flowId && flowId.length <= SKILL_FLOW_MAX_LENGTH) {
      const report = reportSkillFetched(flowId, request.headers.get('sec-fetch-mode') ?? '')
      event?.waitUntil(report)
    }
  }

  return NextResponse.next()
}

export const config = {
  matcher: [
    '/adfa',
    '/adfa/:path*',
    '/feed',
    '/feed/:path*',
    '/new',
    '/problems',
    '/problems/:path*',
    '/ideas',
    '/ideas/:path*',
    '/questions',
    '/questions/:path*',
    '/skill.md',
  ],
}
