'use client';

import { ReactNode } from 'react';

import styles from './ui.module.css';

export type Tone = 'muted' | 'brand' | 'ok' | 'warn' | 'danger' | 'info';

export function Badge({ children, tone = 'muted' }: { children: ReactNode; tone?: Tone }) {
  return <span className={`${styles.badge} ${styles[tone]}`}>{children}</span>;
}

export function Card({ children, tone }: { children: ReactNode; tone?: Tone }) {
  return <section className={`${styles.card} ${tone ? styles[`edge${tone}`] : ''}`}>{children}</section>;
}

export function Button({
  children,
  onClick,
  busy = false,
  disabled = false,
  variant = 'primary',
  size = 'md',
}: {
  children: ReactNode;
  onClick?: () => void;
  busy?: boolean;
  disabled?: boolean;
  variant?: 'primary' | 'ghost' | 'danger';
  size?: 'md' | 'lg';
}) {
  return (
    <button
      type="button"
      className={`${styles.button} ${styles[variant]} ${size === 'lg' ? styles.lg : ''}`}
      onClick={onClick}
      disabled={disabled || busy}
    >
      {busy ? '…' : children}
    </button>
  );
}

export function Toggle({
  label,
  checked,
  onChange,
  busy = false,
}: {
  label: string;
  checked: boolean;
  onChange: (v: boolean) => void;
  busy?: boolean;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      className={`${styles.toggle} ${checked ? styles.toggleOn : ''}`}
      onClick={() => onChange(!checked)}
      disabled={busy}
    >
      <span className={styles.toggleDot} />
      <span>{label}</span>
    </button>
  );
}

export function Field({
  label,
  value,
  onChange,
  placeholder,
  error,
  autoFocus = false,
  maxLength,
  inputMode,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  error?: string | null;
  autoFocus?: boolean;
  maxLength?: number;
  inputMode?: 'text' | 'numeric' | 'tel';
}) {
  return (
    <label className={styles.field}>
      <span className={styles.fieldLabel}>{label}</span>
      <input
        className={`${styles.input} ${error ? styles.inputError : ''}`}
        value={value}
        placeholder={placeholder}
        onChange={(e) => onChange(e.target.value)}
        autoFocus={autoFocus}
        maxLength={maxLength}
        inputMode={inputMode}
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

export function ErrorState({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <div className={styles.state} role="alert">
      <strong className={styles.stateTitle}>Diçka shkoi keq</strong>
      <p className={styles.stateMessage}>{message}</p>
      {onRetry && (
        <Button variant="ghost" onClick={onRetry}>
          Provo përsëri
        </Button>
      )}
    </div>
  );
}
