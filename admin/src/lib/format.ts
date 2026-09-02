/// Formatimet e përbashkëta. Paraja mbetet numër i plotë në cent kudo, siç është në server (§5, §23).

export function money(minor: number, currency = 'EUR'): string {
  const neg = minor < 0;
  const v = Math.abs(minor);
  const symbol = currency === 'EUR' ? '€' : currency;
  const body = `${Math.floor(v / 100)},${String(v % 100).padStart(2, '0')} ${symbol}`;
  return neg ? `-${body}` : body;
}

export function distance(meters: number): string {
  if (meters < 950) return `${meters} m`;
  const km = Math.round(meters / 100) / 10;
  return `${String(km >= 10 ? Math.round(km) : km).replace('.', ',')} km`;
}

export function duration(seconds: number): string {
  if (seconds < 60) return '< 1 min';
  const m = Math.round(seconds / 60);
  if (m < 60) return `${m} min`;
  const h = Math.floor(m / 60);
  const rem = m % 60;
  return rem === 0 ? `${h} h` : `${h} h ${rem} min`;
}

export function clock(iso: string | null | undefined): string {
  if (!iso) return '—';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '—';
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
}

export function dateTime(iso: string | null | undefined): string {
  if (!iso) return '—';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '—';
  return `${d.getDate()}.${d.getMonth() + 1}.${d.getFullYear()} ${clock(iso)}`;
}

/// Sa kohë ka kaluar, e shkurtër sa të lexohet me një shikim në një panel që rifreskohet vetë.
export function ago(iso: string | null | undefined, now: Date = new Date()): string {
  if (!iso) return '—';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '—';
  const s = Math.floor((now.getTime() - d.getTime()) / 1000);
  if (s < 10) return 'tani';
  if (s < 60) return `${s} s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m} min`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h} h`;
  return `${Math.floor(h / 24)} d`;
}

/// Identifikuesit shfaqen të shkurtuar: një panel nuk lexohet dot me UUID të plotë në çdo rresht.
export function shortId(id: string): string {
  return id.length <= 8 ? id : id.slice(0, 8);
}

const rideStates: Record<string, string> = {
  matching: 'Po kërkohet shofer',
  assigned: 'Shoferi po vjen',
  arrived: 'Shoferi mbërriti',
  in_progress: 'Në rrugë',
  completed: 'Përfunduar',
  cancelled: 'Anuluar',
  no_driver: 'Pa shofer',
};

export function rideState(state: string): string {
  return rideStates[state] ?? state;
}

const rideTones: Record<string, string> = {
  matching: 'warn',
  assigned: 'info',
  arrived: 'info',
  in_progress: 'brand',
  completed: 'ok',
  cancelled: 'muted',
  no_driver: 'danger',
};

export function rideTone(state: string): string {
  return rideTones[state] ?? 'muted';
}
