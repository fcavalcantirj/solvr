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
  // SEO: Set proper cache headers for public content pages
  // Self-hosted Next.js (standalone/Docker) doesn't set s-maxage automatically
  async headers() {
    const cache1h = [{ key: 'Cache-Control', value: 'public, s-maxage=3600, stale-while-revalidate=86400' }];
    const cache5m = [{ key: 'Cache-Control', value: 'public, s-maxage=300, stale-while-revalidate=3600' }];
    const cache1d = [{ key: 'Cache-Control', value: 'public, s-maxage=86400, stale-while-revalidate=604800' }];

    return [
      // Homepage
      { source: '/', headers: cache1h },
      // Detail pages (1h cache)
      { source: '/problems/:id', headers: cache1h },
      { source: '/ideas/:id', headers: cache1h },
      { source: '/questions/:id', headers: cache1h },
      { source: '/agents/:id', headers: cache1h },
      { source: '/users/:id', headers: cache1h },
      { source: '/blog/:slug', headers: cache1h },
      { source: '/rooms/:slug', headers: cache1h },
      // List pages (5m cache)
      { source: '/problems', headers: cache5m },
      { source: '/ideas', headers: cache5m },
      { source: '/questions', headers: cache5m },
      { source: '/feed', headers: cache5m },
      { source: '/agents', headers: cache5m },
      { source: '/users', headers: cache5m },
      { source: '/blog', headers: cache5m },
      { source: '/leaderboard', headers: cache5m },
      { source: '/rooms', headers: cache5m },
      // Static pages (1d cache)
      { source: '/about', headers: cache1d },
      { source: '/how-it-works', headers: cache1d },
      { source: '/terms', headers: cache1d },
      { source: '/privacy', headers: cache1d },
    ];
  },
}

export default nextConfig
