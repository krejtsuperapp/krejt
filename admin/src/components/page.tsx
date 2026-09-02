'use client';

import { ReactNode } from 'react';

import styles from './page.module.css';

/// Kreu i njëjtë për çdo faqe: titulli majtas, veprimet djathtas, filtrat poshtë.
export function PageHeader({
  title,
  right,
  children,
}: {
  title: string;
  right?: ReactNode;
  children?: ReactNode;
}) {
  return (
    <header className={styles.head}>
      <div className={styles.row}>
        <h1>{title}</h1>
        {right}
      </div>
      {children && <div className={styles.filters}>{children}</div>}
    </header>
  );
}

export function Chips<T extends string>({
  options,
  value,
  onChange,
}: {
  options: { value: T; label: string }[];
  value: T;
  onChange: (v: T) => void;
}) {
  return (
    <div className={styles.chips} role="group">
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          className={`${styles.chip} ${value === o.value ? styles.chipOn : ''}`}
          aria-pressed={value === o.value}
          onClick={() => onChange(o.value)}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

export function Search({
  value,
  onChange,
  placeholder,
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder: string;
}) {
  return (
    <input
      className={styles.search}
      value={value}
      placeholder={placeholder}
      onChange={(e) => onChange(e.target.value)}
      type="search"
    />
  );
}

/// Fletë anësore për detajet. Lista mbetet e dukshme prapa saj, që operatori të mos e humbë vendin.
export function Drawer({
  title,
  onClose,
  children,
}: {
  title: string;
  onClose: () => void;
  children: ReactNode;
}) {
  return (
    <div className={styles.scrim} onClick={onClose} role="presentation">
      <aside
        className={styles.drawer}
        role="dialog"
        aria-label={title}
        onClick={(e) => e.stopPropagation()}
      >
        <header className={styles.drawerHead}>
          <h2>{title}</h2>
          <button className={styles.close} onClick={onClose} aria-label="Mbyll">
            ✕
          </button>
        </header>
        <div className={styles.drawerBody}>{children}</div>
      </aside>
    </div>
  );
}

export function Row({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className={styles.kv}>
      <span className={styles.kvLabel}>{label}</span>
      <span className={styles.kvValue}>{value}</span>
    </div>
  );
}
