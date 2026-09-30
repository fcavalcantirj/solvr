import { NextResponse } from 'next/server'
import type { NextRequest } from 'next/server'

const BLOCKED_PATHS = ['/adfa']

// Legacy first path segments that now live under the unified /posts collection.
// The pre-redesign product split knowledge into /problems, /ideas and
// /questions collections (plus the /feed aggregate). All of that is now one
// canonical /posts collection, so those routes redirect permanently.
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

export function middleware(request: NextRequest) {
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
  ],
}
