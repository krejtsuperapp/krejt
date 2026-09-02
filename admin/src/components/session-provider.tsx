'use client';

import { createContext, useCallback, useContext, useEffect, useState } from 'react';

import { api, ApiError, auth } from '@/lib/api';
import { isStaff, Me } from '@/lib/types';

type State = {
  me: Me | null;
  loading: boolean;
  signIn: (me: Me) => void;
  signOut: () => Promise<void>;
};

const Ctx = createContext<State>({
  me: null,
  loading: true,
  signIn: () => undefined,
  signOut: async () => undefined,
});

export function useSession() {
  return useContext(Ctx);
}

/// Sesioni provohet një herë në nisje: nëse cookie-t janë ende të vlefshme, stafi hyn drejt.
/// Një llogari pa kapacitete stafi trajtohet si e pakyçur, sepse paneli nuk ka çfarë t'i tregojë.
export function SessionProvider({ children }: { children: React.ReactNode }) {
  const [me, setMe] = useState<Me | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let alive = true;
    api
      .get<Me>('users/me')
      .then((user) => {
        if (alive) setMe(isStaff(user) ? user : null);
      })
      .catch((e: unknown) => {
        if (alive && !(e instanceof ApiError && e.isUnauthorized)) setMe(null);
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, []);

  const signOut = useCallback(async () => {
    await auth('logout').catch(() => undefined);
    setMe(null);
  }, []);

  return (
    <Ctx.Provider value={{ me, loading, signIn: setMe, signOut }}>{children}</Ctx.Provider>
  );
}
