import { NextResponse } from 'next/server'
import type { NextRequest } from 'next/server'

const BLOCKED_PATHS = ['/adfa']

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl

  if (BLOCKED_PATHS.some((p) => pathname === p || pathname.startsWith(p + '/'))) {
    return new NextResponse(null, { status: 404 })
  }

  return NextResponse.next()
}

export const config = {
  matcher: ['/adfa', '/adfa/:path*'],
}
