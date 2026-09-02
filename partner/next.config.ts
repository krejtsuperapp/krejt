import type { NextConfig } from 'next';

const config: NextConfig = {
  reactStrictMode: true,
  // Paneli i Operacioneve nuk shërben imazhe të jashtme dhe nuk ka nevojë për optimizim figurash.
  images: { unoptimized: true },
  async headers() {
    return [
      {
        source: '/:path*',
        headers: [
          { key: 'X-Frame-Options', value: 'DENY' },
          { key: 'X-Content-Type-Options', value: 'nosniff' },
          { key: 'Referrer-Policy', value: 'no-referrer' },
          { key: 'Permissions-Policy', value: 'geolocation=(), camera=(), microphone=()' },
        ],
      },
    ];
  },
};

export default config;
