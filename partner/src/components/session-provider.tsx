'use client';

import { createContext, useCallback, useContext, useEffect, useState } from 'react';

import { api, ApiError, auth } from '@/lib/api';
import { Items, Me, Merchant } from '@/lib/types';

type State = {
  me: Me | null;
  merchants: Merchant[];
  merchant: Merchant | null;
  loading: boolean;
  select: (m: Merchant) => void;
  reload: () => Promise<void>;
  signIn: () => Promise<void>;
  signOut: () => Promise<void>;
};

const Ctx = createContext<State>({
  me: null,
  merchants: [],
  merchant: null,
  loading: true,
  select: () => undefined,
  reload: async () => undefined,
  signIn: async () => undefined,
  signOut: async () => undefined,
});

export function useSession() {
  return useContext(Ctx);
}

const PICKED = 'krejt.partner.merchant';

/// Sesioni i partnerit. Një llogari mund të jetë staf i disa vendeve; i zgjedhuri ruhet lokalisht
/// që tableti i kuzhinës të hapet gjithmonë te i njëjti vend pas rindezjes.
export function SessionProvider({ children }: { children: React.ReactNode }) {
  const [me, setMe] = useState<Me | null>(null);
  const [merchants, setMerchants] = useState<Merchant[]>([]);
  const [merchant, setMerchant] = useState<Merchant | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    const user = await api.get<Me>('users/me');
    const mine = await api.get<Items<Merchant>>('merchant/mine');
    const items = mine.items ?? [];
    setMe(user);
    setMerchants(items);

    let picked: string | null = null;
    try {
      picked = window.localStorage.getItem(PICKED);
    } catch {
      // Ruajtja lokale mund të jetë e mbyllur; atëherë zgjidhet i pari.
    }
    setMerchant(items.find((m) => m.id === picked) ?? items[0] ?? null);
  }, []);

  useEffect(() => {
    let alive = true;
    load()
      .catch((e: unknown) => {
        if (alive && !(e instanceof ApiError)) setMe(null);
        if (alive && e instanceof ApiError) setMe(null);
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, [load]);

  const select = useCallback((m: Merchant) => {
    setMerchant(m);
    try {
      window.localStorage.setItem(PICKED, m.id);
    } catch {
      // Zgjedhja mbetet për këtë sesion edhe nëse nuk ruhet dot.
    }
  }, []);

  const signIn = useCallback(async () => {
    setLoading(true);
    try {
      await load();
    } finally {
      setLoading(false);
    }
  }, [load]);

  const signOut = useCallback(async () => {
    await auth('logout').catch(() => undefined);
    setMe(null);
    setMerchants([]);
    setMerchant(null);
  }, []);

  const reload = useCallback(async () => {
    await load().catch(() => undefined);
  }, [load]);

  return (
    <Ctx.Provider
      value={{ me, merchants, merchant, loading, select, reload, signIn, signOut }}
    >
      {children}
    </Ctx.Provider>
  );
}
