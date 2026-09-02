/// Formatimet e panelit. Paraja mbetet numër i plotë në cent, si kudo tjetër në KREJT (§5).

export function money(minor: number, currency = 'EUR'): string {
  const neg = minor < 0;
  const v = Math.abs(minor);
  const symbol = currency === 'EUR' ? '€' : currency;
  const body = `${Math.floor(v / 100)},${String(v % 100).padStart(2, '0')} ${symbol}`;
  return neg ? `-${body}` : body;
}

export function clock(iso: string | null | undefined): string {
  if (!iso) return '—';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '—';
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
}

const states: Record<string, string> = {
  pending_merchant: 'E re',
  accepted: 'Pranuar',
  preparing: 'Po përgatitet',
  ready: 'Gati',
  courier_assigned: 'Korrieri po vjen',
  picked_up: 'Marrë nga korrieri',
  delivered: 'Dorëzuar',
  cancelled: 'Anuluar',
  rejected: 'Refuzuar',
};

export function orderState(state: string): string {
  return states[state] ?? state;
}

const tones: Record<string, string> = {
  pending_merchant: 'danger',
  accepted: 'warn',
  preparing: 'brand',
  ready: 'ok',
  courier_assigned: 'info',
  picked_up: 'info',
  delivered: 'muted',
  cancelled: 'muted',
  rejected: 'muted',
};

export function orderTone(state: string): string {
  return tones[state] ?? 'muted';
}

/// Sa gjatë ka pritur, e shkruar shkurt sa të lexohet nga larg në një tablet kuzhine.
export function waited(minutes: number): string {
  if (minutes < 1) return 'tani';
  if (minutes < 60) return `${minutes} min`;
  const h = Math.floor(minutes / 60);
  const rem = minutes % 60;
  return rem === 0 ? `${h} h` : `${h} h ${rem} min`;
}
