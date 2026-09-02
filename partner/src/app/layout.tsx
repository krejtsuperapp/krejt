import type { Metadata, Viewport } from 'next';

import { SessionProvider } from '@/components/session-provider';
import { Shell } from '@/components/shell';

import './globals.css';

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
  themeColor: '#070b18',
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="sq">
      <body>
        <SessionProvider>
          <Shell>{children}</Shell>
        </SessionProvider>
      </body>
    </html>
  );
}
