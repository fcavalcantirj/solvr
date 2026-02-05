/** @type {import('next').NextConfig} */
const nextConfig = {
  typescript: {
    ignoreBuildErrors: true,
  },
  images: {
    unoptimized: true,
  },
  // Disable static generation for all pages
  experimental: {
    // Skip prerendering
  },
}

export default nextConfig
