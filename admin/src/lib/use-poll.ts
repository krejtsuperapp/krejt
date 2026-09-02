'use client';

import { useCallback, useEffect, useRef, useState } from 'react';

export type Poll<T> = {
  data: T | null;
  error: unknown;
  loading: boolean;
  refresh: () => void;
};

/// Ngarkim me rifreskim periodik. Të dhënat e vjetra mbeten në ekran gjatë rifreskimit,
/// që paneli të mos pulsojë çdo disa sekonda dhe operatori të mos e humbë rreshtin që po lexon.
/// Një gabim i rastit nuk e fshin pamjen: ruhet veç dhe shfaqet vetëm kur s'ka çfarë të tregohet.
export function usePoll<T>(load: () => Promise<T>, everyMs = 0): Poll<T> {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [loading, setLoading] = useState(true);
  const [tick, setTick] = useState(0);

  const loadRef = useRef(load);
  loadRef.current = load;

  const refresh = useCallback(() => setTick((t) => t + 1), []);

  useEffect(() => {
    let alive = true;

    const run = async () => {
      try {
        const next = await loadRef.current();
        if (!alive) return;
        setData(next);
        setError(null);
      } catch (e) {
        if (alive) setError(e);
      } finally {
        if (alive) setLoading(false);
      }
    };

    void run();
    if (everyMs <= 0) return () => { alive = false; };

    const timer = setInterval(run, everyMs);
    return () => {
      alive = false;
      clearInterval(timer);
    };
  }, [everyMs, tick]);

  return { data, error, loading, refresh };
}
