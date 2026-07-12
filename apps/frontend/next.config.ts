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
  // React/Next.js hydration requires inline scripts. In production we still allow
  // unsafe-inline (Next.js 15 needs it for the __next_f runtime bootstrap); when we
  // wire up SSR nonces later this can drop to nonces only.
  isDev
    ? "script-src 'self' 'unsafe-inline' 'unsafe-eval'"
    : "script-src 'self' 'unsafe-inline'",
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
          { key: 'Permissions-Policy', value: 'camera=(), microphone=(self), geolocation=(), interest-cohort=(), payment=()' },
          { key: 'Strict-Transport-Security', value: 'max-age=63072000; includeSubDomains; preload' },
          { key: 'Cross-Origin-Opener-Policy', value: 'same-origin' },
          { key: 'Cross-Origin-Resource-Policy', value: 'same-origin' },
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
