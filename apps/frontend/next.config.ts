import type { NextConfig } from 'next'

const isDev = process.env.NODE_ENV === 'development'

const csp = [
  "default-src 'self'",
  "connect-src 'self' http://127.0.0.1:* http://localhost:* ws://127.0.0.1:* ws://localhost:* https://*.stun.l.google.com",
  "img-src 'self' data: blob:",
  "media-src 'self' blob:",
  "worker-src 'self' blob:",
  "style-src 'self' 'unsafe-inline'",
  "font-src 'self' data:",
  "frame-ancestors 'none'",
  // React hydration in development requires inline scripts and eval.
  isDev
    ? "script-src 'self' 'unsafe-inline' 'unsafe-eval'"
    : "script-src 'self'",
].join('; ')

const nextConfig: NextConfig = {
  output: 'standalone',

  poweredByHeader: false,
  reactStrictMode: true,
  compress: true,
  productionBrowserSourceMaps: false,
  httpAgentOptions: {
    keepAlive: true,
  },

  experimental: {
    optimizePackageImports: ['lucide-react'],
    scrollRestoration: true,
    serverActions: {
      bodySizeLimit: '4mb',
    },
    staleTimes: {
      dynamic: 30,
      static: 180,
    },
  },

  images: {
    formats: ['image/avif', 'image/webp'],
    minimumCacheTTL: 86400,
    deviceSizes: [640, 768, 1024, 1280, 1536],
  },

  async headers() {
    const baseHeaders: { key: string; value: string }[] = [
      { key: 'Content-Security-Policy', value: csp },
    ]

    // Harden headers are applied only in production. In development the CSP is
    // intentionally relaxed (unsafe-inline + unsafe-eval) so React hydration and
    // HMR work without triggering CSP violations.
    const hardenHeaders: { key: string; value: string }[] = isDev
      ? []
      : [
          { key: 'X-Content-Type-Options', value: 'nosniff' },
          { key: 'X-Frame-Options', value: 'DENY' },
          { key: 'X-XSS-Protection', value: '1; mode=block' },
          { key: 'Referrer-Policy', value: 'strict-origin-when-cross-origin' },
          { key: 'Permissions-Policy', value: 'camera=(), microphone=(), geolocation=(), interest-cohort=()' },
        ]

    return [
      {
        source: '/:path*',
        headers: [...baseHeaders, ...hardenHeaders],
      },
      {
        source: '/proxy/:path*',
        headers: [
          { key: 'Cache-Control', value: 'no-store, no-cache, must-revalidate' },
        ],
      },
      {
        source: '/sw.js',
        headers: [
          { key: 'Cache-Control', value: 'public, max-age=0, must-revalidate' },
          { key: 'Service-Worker-Allowed', value: '/' },
        ],
      },
      {
        source: '/manifest.json',
        headers: [
          { key: 'Cache-Control', value: 'public, max-age=3600' },
        ],
      },
    ]
  },
}

export default nextConfig
