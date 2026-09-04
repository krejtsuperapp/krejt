"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

import { CAP, can } from "@/lib/types";

import { useSession } from "./session-provider";
import { SignIn } from "./sign-in";
import { Loading } from "./ui";
import styles from "./shell.module.css";

type Entry = { href: string; label: string; caps: string[] };

/// Menyja ndjek kapacitetet: secili sheh vetëm atë që bën dot. Serveri i zbaton gjithsesi (§37).
const ENTRIES: Entry[] = [
  { href: "/", label: "Dispatch", caps: [CAP.operations, CAP.support] },
  { href: "/rides", label: "Udhëtimet", caps: [CAP.operations, CAP.support] },
  { href: "/drivers", label: "Shoferët", caps: [CAP.operations] },
  { href: "/support", label: "Mbështetja", caps: [CAP.support] },
  { href: "/users", label: "Përdoruesit", caps: [CAP.operations, CAP.support] },
  { href: "/risk", label: "Risku", caps: [CAP.operations] },
  { href: "/providers", label: "Mjeshtrit", caps: [CAP.operations] },
  { href: "/coupons", label: "Kupona", caps: [CAP.operations] },
  { href: "/flags", label: "Flag-et", caps: [CAP.operations] },
];

export function Shell({ children }: { children: React.ReactNode }) {
  const { me, loading, signIn, signOut } = useSession();
  const pathname = usePathname();

  if (loading) {
    return (
      <main className={styles.center}>
        <Loading />
      </main>
    );
  }

  if (!me) return <SignIn onSignedIn={signIn} />;

  const entries = ENTRIES.filter((e) => can(me, ...e.caps));

  return (
    <div className={styles.layout}>
      <aside className={styles.sidebar}>
        <span className={styles.wordmark}>KREJT</span>
        <nav className={styles.nav}>
          {entries.map((e) => (
            <Link
              key={e.href}
              href={e.href}
              className={`${styles.link} ${pathname === e.href ? styles.active : ""}`}
            >
              {e.label}
            </Link>
          ))}
        </nav>
        <div className={styles.user}>
          <span className={styles.userName}>
            {me.full_name ?? me.phone ?? "—"}
          </span>
          <span className={styles.userCaps}>
            {me.capabilities.join(" · ") || "—"}
          </span>
          <button className={styles.signOut} onClick={signOut}>
            Dil
          </button>
        </div>
      </aside>
      <main className={styles.content}>{children}</main>
    </div>
  );
}
