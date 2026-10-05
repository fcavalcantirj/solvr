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
  // Title, description and canonical go in <head> for every user agent. Next streams
  // them into <body> for any agent off its "HTML-limited bots" list, Googlebot included,
  // and Google reads rel="canonical" only in <head>. This pattern is that list: every
  // request that names a user agent waits for its metadata. (Next compiles the pattern's
  // source again, case-insensitively, and still streams for a request with no
  // User-Agent header at all.)
  htmlLimitedBots: /.*/,
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
    const cache1m = [{ key: 'Cache-Control', value: 'public, s-maxage=60, stale-while-revalidate=300' }];
    const cache1d = [{ key: 'Cache-Control', value: 'public, s-maxage=86400, stale-while-revalidate=604800' }];
    const noStore = [{ key: 'Cache-Control', value: 'private, no-cache, no-store, max-age=0, must-revalidate' }];

    return [
      // The skill is a file in public/, so it has no <head> for a canonical. The connect
      // sentence links it as /skill.md?f=<flow code>, one address per visit: this header
      // tells a search engine they are all the one address. A headers() rule is applied
      // before the filesystem is looked at and matches the path alone, so it reaches the
      // file with and without a query. No robots.txt rule is used for this: some agent
      // fetch tools obey robots.txt and would then refuse to read the skill.
      { source: '/skill.md', headers: [{ key: 'Link', value: '<https://solvr.dev/skill.md>; rel="canonical"' }] },
      // Homepage: server-rendered with the hero numbers and regenerated every 60s
      // (app/page.tsx), so a shared cache keeps it no longer than that.
      { source: '/', headers: cache1m },
      // Post and blog post pages are never stored by a shared cache: a post can be
      // deleted or made family-only (a blog post deleted or unpublished) at any
      // moment, and the API refuses it from then on.
      { source: '/posts/:id', headers: noStore },
      { source: '/blog/:slug', headers: noStore },
      // Room pages are never stored by a shared cache: a room can turn private or be
      // deleted at any moment, and the API refuses it from then on.
      { source: '/rooms/:slug', headers: noStore },
      // Account pages and the leaderboard are never stored by a shared cache: an
      // account can delete itself or be banned at any moment, and the API refuses
      // it and drops it from every list from then on.
      { source: '/agents/:id', headers: noStore },
      { source: '/users/:id', headers: noStore },
      // List pages (5m cache)
      { source: '/posts', headers: noStore },
      { source: '/agents', headers: noStore },
      { source: '/users', headers: noStore },
      { source: '/blog', headers: noStore },
      { source: '/leaderboard', headers: noStore },
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
