import type { Metadata } from 'next';
import localFont from 'next/font/local';

import { SessionProvider } from '@/components/session-provider';
import { Shell } from '@/components/shell';

import './globals.css';

// Inter (SIL OFL), i paketuar: paneli nuk varet nga rrjeti për fontin e markës.
const inter = localFont({
  src: './fonts/InterVariable.woff2',
  variable: '--font-inter',
  display: 'swap',
  weight: '100 900',
});

export const metadata: Metadata = {
  title: 'KREJT — Operacionet',
  description: 'Paneli i Operacioneve: dispatch live, shoferët, mbështetja.',
  // Paneli nuk indeksohet: është mjet i brendshëm, jo faqe publike.
  robots: { index: false, follow: false },
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
