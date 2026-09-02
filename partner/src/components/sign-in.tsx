'use client';

import { useState } from 'react';

import { auth } from '@/lib/api';
import { errorText } from '@/lib/errors';

import { Button, Field } from './ui';
import styles from './sign-in.module.css';

/// Kyçja me kod njëpërdorimësh. Token-at nuk kalojnë nga kjo faqe: shkojnë drejt në cookie
/// httpOnly te rruga /api/auth. Nëse llogaria nuk është staf i asnjë vendi, kjo shihet
/// pas kyçjes, kur serveri kthen listë bosh.
export function SignIn({ onSignedIn }: { onSignedIn: () => void }) {
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
      await auth('verify', { phone: e164(), code });
      onSignedIn();
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
        <h1 className={styles.title}>Paneli i partnerit</h1>
        <p className={styles.subtitle}>
          {sent ? `Kodin e dërguam te ${e164()}.` : 'Kyçu me numrin që ke regjistruar te KREJT.'}
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
              inputMode="numeric"
            />
            <div className={styles.actions}>
              <Button onClick={verify} busy={busy} disabled={code.length !== 6} size="lg">
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
              inputMode="tel"
            />
            <div className={styles.actions}>
              <Button onClick={request} busy={busy} disabled={!valid} size="lg">
                Dërgo kodin
              </Button>
            </div>
          </>
        )}
      </div>
    </main>
  );
}
