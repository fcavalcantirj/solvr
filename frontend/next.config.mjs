/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'standalone', // Required for Docker deployment
  typescript: {
    ignoreBuildErrors: true,
  },
  eslint: {
    ignoreDuringBuilds: true,
  },
  images: {
    unoptimized: true,
  },
  // Increase timeout to allow pages to render
  staticPageGenerationTimeout: 120,
  // Legacy collection/detail routes (/feed, /problems, /ideas, /questions and
  // their {id}/new/edit sub-paths) permanently redirect to the canonical /posts
  // collection. That mapping lives in middleware.ts so it stays unit-tested and
  // preserves the query string; it must NOT be duplicated here. /posts is now a
  // real page, so the old placeholder `/posts -> /feed` redirect has been
  // removed — keeping it would make /feed -> /posts -> /feed loop.
  async redirects() {
    return [];
  },
  // SEO: Set proper cache headers for public content pages
  // Self-hosted Next.js (standalone/Docker) doesn't set s-maxage automatically
  async headers() {
    const cache1h = [{ key: 'Cache-Control', value: 'public, s-maxage=3600, stale-while-revalidate=86400' }];
    const cache5m = [{ key: 'Cache-Control', value: 'public, s-maxage=300, stale-while-revalidate=3600' }];
    const cache1d = [{ key: 'Cache-Control', value: 'public, s-maxage=86400, stale-while-revalidate=604800' }];
    const noStore = [{ key: 'Cache-Control', value: 'private, no-cache, no-store, max-age=0, must-revalidate' }];

    return [
      // Homepage
      { source: '/', headers: cache1h },
      // Detail pages (1h cache)
      { source: '/problems/:id', headers: cache1h },
      { source: '/ideas/:id', headers: cache1h },
      { source: '/questions/:id', headers: cache1h },
      { source: '/agents/:id', headers: cache1h },
      { source: '/users/:id', headers: cache1h },
      // Post and blog post pages are never stored by a shared cache: a post can be
      // deleted or made family-only (a blog post deleted or unpublished) at any
      // moment, and the API refuses it from then on.
      { source: '/posts/:id', headers: noStore },
      { source: '/blog/:slug', headers: noStore },
      // Room pages are never stored by a shared cache: a room can turn private or be
      // deleted at any moment, and the API refuses it from then on.
      { source: '/rooms/:slug', headers: noStore },
      // List pages (5m cache)
      { source: '/problems', headers: cache5m },
      { source: '/ideas', headers: cache5m },
      { source: '/questions', headers: cache5m },
      { source: '/feed', headers: cache5m },
      { source: '/posts', headers: noStore },
      { source: '/agents', headers: cache5m },
      { source: '/users', headers: cache5m },
      { source: '/blog', headers: noStore },
      { source: '/leaderboard', headers: cache5m },
      { source: '/rooms', headers: noStore },
      // Static pages (1d cache)
      { source: '/about', headers: cache1d },
      { source: '/how-it-works', headers: cache1d },
      { source: '/terms', headers: cache1d },
      { source: '/privacy', headers: cache1d },
    ];
  },
}

export default nextConfig
