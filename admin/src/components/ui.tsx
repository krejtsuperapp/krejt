'use client';

import { ReactNode } from 'react';

import styles from './ui.module.css';

/// Pjesët e përbashkëta të panelit. Të njëjtat gjendje si te aplikacionet: ngarkim, bosh, gabim (§55).

export type Tone = 'muted' | 'brand' | 'ok' | 'warn' | 'danger' | 'info';

export function Badge({ children, tone = 'muted' }: { children: ReactNode; tone?: Tone }) {
  return <span className={`${styles.badge} ${styles[tone]}`}>{children}</span>;
}

export function Card({
  children,
  title,
  action,
}: {
  children: ReactNode;
  title?: string;
  action?: ReactNode;
}) {
  return (
    <section className={styles.card}>
      {(title || action) && (
        <header className={styles.cardHead}>
          {title && <h2>{title}</h2>}
          {action}
        </header>
      )}
      {children}
    </section>
  );
}

export function Stat({ label, value, tone }: { label: string; value: ReactNode; tone?: Tone }) {
  return (
    <div className={styles.stat}>
      <span className={styles.statLabel}>{label}</span>
      <span className={`${styles.statValue} num ${tone ? styles[`${tone}Text`] : ''}`}>{value}</span>
    </div>
  );
}

export function Button({
  children,
  onClick,
  busy = false,
  disabled = false,
  variant = 'primary',
  type = 'button',
}: {
  children: ReactNode;
  onClick?: () => void;
  busy?: boolean;
  disabled?: boolean;
  variant?: 'primary' | 'ghost' | 'danger';
  type?: 'button' | 'submit';
}) {
  return (
    <button
      type={type}
      className={`${styles.button} ${styles[variant]}`}
      onClick={onClick}
      disabled={disabled || busy}
    >
      {busy ? '…' : children}
    </button>
  );
}

export function Field({
  label,
  value,
  onChange,
  placeholder,
  error,
  type = 'text',
  autoFocus = false,
  maxLength,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  error?: string | null;
  type?: string;
  autoFocus?: boolean;
  maxLength?: number;
}) {
  return (
    <label className={styles.field}>
      <span className={styles.fieldLabel}>{label}</span>
      <input
        className={`${styles.input} ${error ? styles.inputError : ''}`}
        value={value}
        type={type}
        placeholder={placeholder}
        onChange={(e) => onChange(e.target.value)}
        autoFocus={autoFocus}
        maxLength={maxLength}
      />
      {error && <span className={styles.fieldError}>{error}</span>}
    </label>
  );
}

export function Loading({ label = 'Po ngarkohet…' }: { label?: string }) {
  return (
    <p className={styles.state} role="status">
      {label}
    </p>
  );
}

export function Empty({ title, message }: { title: string; message?: string }) {
  return (
    <div className={styles.state}>
      <strong className={styles.stateTitle}>{title}</strong>
      {message && <p className={styles.stateMessage}>{message}</p>}
    </div>
  );
}

export function ErrorState({
  message,
  onRetry,
}: {
  message: string;
  onRetry?: () => void;
}) {
  return (
    <div className={styles.state} role="alert">
      <strong className={`${styles.stateTitle} ${styles.dangerText}`}>Diçka shkoi keq</strong>
      <p className={styles.stateMessage}>{message}</p>
      {onRetry && <Button variant="ghost" onClick={onRetry}>Provo përsëri</Button>}
    </div>
  );
}
