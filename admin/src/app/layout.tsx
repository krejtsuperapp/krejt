import type { Metadata } from 'next';

import { SessionProvider } from '@/components/session-provider';
import { Shell } from '@/components/shell';

import './globals.css';

export const metadata: Metadata = {
  title: 'KREJT — Operacionet',
  description: 'Paneli i Operacioneve: dispatch live, shoferët, mbështetja.',
  // Paneli nuk indeksohet: është mjet i brendshëm, jo faqe publike.
  robots: { index: false, follow: false },
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
