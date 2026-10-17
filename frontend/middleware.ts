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
// A report is abandoned after this long. It never holds the response either way.
const REPORT_TIMEOUT_MS = 3000
// How much of a request header is handed to the API (it accepts no more).
const REPORT_HEADER_MAX_LENGTH = 200

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev'
// Each report names its sender, so the API host and the edge in front of it can tell it
// from an anonymous client (the runtime would otherwise send its own default).
const SKILL_REPORT_USER_AGENT = 'solvr-web/1.0 (skill-fetch report)'
const PAGE_VIEW_REPORT_USER_AGENT = 'solvr-web/1.0 (pageview report)'

// sendReport posts one report to the API. The request starts at once, beside the response.
// The promise it returns settles when the API answered, failed or the timeout passed, and it
// never rejects: a statistic must not break the page it measures.
function sendReport(endpoint: string, userAgent: string, body: Record<string, unknown>): Promise<void> {
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), REPORT_TIMEOUT_MS)
  let sent: Promise<unknown>
  try {
    sent = fetch(`${API_BASE_URL}${endpoint}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'User-Agent': userAgent },
      body: JSON.stringify(body),
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

// reportSkillFetched tells the API that the skill link was fetched with a flow code, and
// how: the request's Sec-Fetch-Mode and its User-Agent, as they came. The API uses them to
// tell a person's browser, a link preview or crawler, and an agent apart, and decides
// everything else too; nothing is judged here.
function reportSkillFetched(flowId: string, requestMode: string, userAgent: string): Promise<void> {
  return sendReport('/v1/analytics/funnel', SKILL_REPORT_USER_AGENT, {
    event: 'skill_fetched',
    flow_id: flowId,
    request_mode: requestMode,
    user_agent: userAgent,
  })
}

// reportPageView tells the API that a page was served (SPEC.md 27.6, 27.7): its path,
// without the query string (it can carry a secret), and what the request said about itself.
// The API drops a prefetch, reduces the path to the kind of page and classifies the view;
// nothing is judged here, and nothing is stored that names the visitor.
function reportPageView(request: NextRequest): Promise<void> {
  const header = (name: string) => request.headers.get(name) ?? ''
  const purpose = `${header('sec-purpose')} ${header('purpose')}`.toLowerCase()
  return sendReport('/v1/analytics/pageview', PAGE_VIEW_REPORT_USER_AGENT, {
    path: request.nextUrl.pathname,
    user_agent: header('user-agent').slice(0, REPORT_HEADER_MAX_LENGTH),
    request_mode: header('sec-fetch-mode'),
    fetch_dest: header('sec-fetch-dest'),
    sec_ch_ua: header('sec-ch-ua').slice(0, REPORT_HEADER_MAX_LENGTH),
    accept_language: request.headers.has('accept-language'),
    rsc: header('rsc') === '1',
    prefetch: purpose.includes('prefetch'),
  })
}

// isPageView is a GET of a page, not a file (a name with a dot) and not the app's own
// prefetch of a link: Next asks for the next page before anyone opens it.
function isPageView(request: NextRequest): boolean {
  const { pathname } = request.nextUrl
  const last = pathname.split('/').pop() ?? ''
  return (
    request.method === 'GET' &&
    !last.includes('.') &&
    !pathname.startsWith('/_next/') &&
    !pathname.startsWith('/api/') &&
    !request.headers.has('next-router-prefetch') &&
    !request.headers.has('next-router-segment-prefetch')
  )
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
      const report = reportSkillFetched(
        flowId,
        request.headers.get('sec-fetch-mode') ?? '',
        (request.headers.get('user-agent') ?? '').slice(0, REPORT_HEADER_MAX_LENGTH),
      )
      event?.waitUntil(report)
    }
  }

  // A page served: reported beside the response, never awaited.
  if (isPageView(request)) {
    event?.waitUntil(reportPageView(request))
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
    // Every page, for the page-view report: not Next's own files, not a file with a dot in its
    // name, not an /api route.
    '/((?!_next/|api/|.*\\..*).*)',
  ],
}
