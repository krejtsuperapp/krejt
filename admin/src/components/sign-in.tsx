'use client';

import { useState } from 'react';

import { auth } from '@/lib/api';
import { errorText } from '@/lib/errors';
import { isStaff, Me } from '@/lib/types';

import { Button, Field } from './ui';
import styles from './sign-in.module.css';

/// Kyçja e stafit me kod njëpërdorimësh, i njëjti mekanizëm si te aplikacionet.
/// Token-at nuk kalojnë kurrë nga kjo faqe: shkojnë drejt në cookie httpOnly te rruga /api/auth.
export function SignIn({ onSignedIn }: { onSignedIn: (me: Me) => void }) {
  const [phone, setPhone] = useState('');
  const [code, setCode] = useState('');
  const [sent, setSent] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const e164 = () => {
    const digits = phone.replace(/\D/g, '');
    if (digits.startsWith('383')) return `+${digits}`;
    if (digits.startsWith('0')) return `+383${digits.slice(1)}`;
    return `+383${digits}`;
  };

  const valid = phone.replace(/\D/g, '').length >= 8;

  async function request() {
    if (!valid) {
      setError('Numri nuk duket i saktë');
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await auth('request', { phone: e164() });
      setSent(true);
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  }

  async function verify() {
    setBusy(true);
    setError(null);
    try {
      const me = await auth<Me>('verify', { phone: e164(), code });
      if (!isStaff(me)) {
        // Kyçja u krye, por kjo llogari nuk ka punë këtu; e themi qartë e nuk e lëmë në një faqe bosh.
        setError('Kjo llogari nuk ka qasje në panelin e Operacioneve.');
        await auth('logout').catch(() => undefined);
        return;
      }
      onSignedIn(me);
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className={styles.wrap}>
      <div className={styles.panel}>
        <span className={styles.wordmark}>KREJT</span>
        <h1 className={styles.title}>Paneli i Operacioneve</h1>
        <p className={styles.subtitle}>
          {sent ? `Kodin e dërguam te ${e164()}.` : 'Kyçu me numrin e stafit.'}
        </p>

        {sent ? (
          <>
            <Field
              label="Kodi"
              value={code}
              onChange={(v) => setCode(v.replace(/\D/g, '').slice(0, 6))}
              placeholder="123456"
              error={error}
              autoFocus
              maxLength={6}
            />
            <div className={styles.actions}>
              <Button onClick={verify} busy={busy} disabled={code.length !== 6}>
                Hyr
              </Button>
              <Button variant="ghost" onClick={() => setSent(false)} disabled={busy}>
                Ndrysho numrin
              </Button>
            </div>
          </>
        ) : (
          <>
            <Field
              label="Numri i telefonit"
              value={phone}
              onChange={(v) => {
                setPhone(v.replace(/\D/g, '').slice(0, 12));
                setError(null);
              }}
              placeholder="44 123 456"
              error={error}
              autoFocus
            />
            <div className={styles.actions}>
              <Button onClick={request} busy={busy} disabled={!valid}>
                Dërgo kodin
              </Button>
            </div>
          </>
        )}
      </div>
    </main>
  );
}
