import type { Metadata, Viewport } from 'next';
import localFont from 'next/font/local';

import { SessionProvider } from '@/components/session-provider';
import { Shell } from '@/components/shell';

import './globals.css';

// Inter (SIL OFL), i paketuar: tableti i kuzhinës nuk varet nga rrjeti për fontin e markës.
const inter = localFont({
  src: './fonts/InterVariable.woff2',
  variable: '--font-inter',
  display: 'swap',
  weight: '100 900',
});

export const metadata: Metadata = {
  title: 'KREJT — Partneri',
  description: 'Radha e porosive, menuja dhe cilësimet e vendit.',
  robots: { index: false, follow: false },
};

// Tableti i kuzhinës rri i ndezur gjatë; ekrani nuk duhet të zvogëlohet me prekje të rastit.
export const viewport: Viewport = {
  width: 'device-width',
  initialScale: 1,
  maximumScale: 1,
  themeColor: '#0d0d0d',
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="sq" className={inter.variable}>
      <body>
        <SessionProvider>
          <Shell>{children}</Shell>
        </SessionProvider>
      </body>
    </html>
  );
}
