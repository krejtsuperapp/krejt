'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';

import { useSession } from './session-provider';
import { SignIn } from './sign-in';
import { Empty, Loading } from './ui';
import styles from './shell.module.css';

const ENTRIES = [
  { href: '/', label: 'Porositë' },
  { href: '/menu', label: 'Menuja' },
  { href: '/settings', label: 'Cilësimet' },
];

export function Shell({ children }: { children: React.ReactNode }) {
  const { me, merchants, merchant, loading, select, signIn, signOut } = useSession();
  const pathname = usePathname();

  if (loading) {
    return (
      <main className={styles.center}>
        <Loading />
      </main>
    );
  }

  if (!me) return <SignIn onSignedIn={signIn} />;

  // Kyçja funksionoi, por kjo llogari nuk është staf i asnjë vendi. E themi qartë,
  // në vend që ta lëmë në një panel bosh që duket i prishur.
  if (!merchant) {
    return (
      <main className={styles.center}>
        <div className={styles.notice}>
          <Empty
            title="Kjo llogari nuk është e lidhur me asnjë vend"
            message="Kërko nga pronari i vendit të të shtojë si staf, pastaj kyçu përsëri."
          />
          <button className={styles.signOut} onClick={signOut}>
            Dil
          </button>
        </div>
      </main>
    );
  }

  return (
    <div className={styles.layout}>
      <header className={styles.top}>
        <div className={styles.identity}>
          <span className={styles.wordmark}>KREJT</span>
          {merchants.length > 1 ? (
            <select
              className={styles.picker}
              value={merchant.id}
              onChange={(e) => {
                const next = merchants.find((m) => m.id === e.target.value);
                if (next) select(next);
              }}
            >
              {merchants.map((m) => (
                <option key={m.id} value={m.id}>
                  {m.name}
                </option>
              ))}
            </select>
          ) : (
            <strong className={styles.name}>{merchant.name}</strong>
          )}
        </div>

        <nav className={styles.nav}>
          {ENTRIES.map((e) => (
            <Link
              key={e.href}
              href={e.href}
              className={`${styles.link} ${pathname === e.href ? styles.active : ''}`}
            >
              {e.label}
            </Link>
          ))}
        </nav>

        <button className={styles.signOut} onClick={signOut}>
          Dil
        </button>
      </header>
      <main className={styles.content}>{children}</main>
    </div>
  );
}
