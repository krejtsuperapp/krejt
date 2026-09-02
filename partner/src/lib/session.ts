import { cookies } from 'next/headers';

/// Sesioni i stafit të merchant-it rri në cookie httpOnly, njësoj si te paneli i Operacioneve:
/// një tablet kuzhine qëndron i kyçur gjithë ditën dhe token-i nuk duhet të jetë i lexueshëm
/// nga asnjë skript i faqes (§53, §57).

const ACCESS = 'krejt_at';
const REFRESH = 'krejt_rt';

export const API_BASE = process.env.KREJT_API_BASE_URL ?? 'http://localhost:8080';

export const appHeaders: Record<string, string> = {
  'X-App-Id': 'partner',
  'X-App-Platform': 'web',
  'X-App-Version': process.env.KREJT_APP_VERSION ?? '0.1.0',
};

type CookieStore = Awaited<ReturnType<typeof cookies>>;

const secure = process.env.NODE_ENV === 'production';

export async function readTokens(): Promise<{ access?: string; refresh?: string }> {
  const jar = await cookies();
  return { access: jar.get(ACCESS)?.value, refresh: jar.get(REFRESH)?.value };
}

export function writeTokens(jar: CookieStore, access: string, refresh: string) {
  const base = { httpOnly: true, secure, sameSite: 'lax' as const, path: '/' };
  jar.set(ACCESS, access, { ...base, maxAge: 15 * 60 });
  jar.set(REFRESH, refresh, { ...base, maxAge: 30 * 24 * 60 * 60 });
}

export function clearTokens(jar: CookieStore) {
  jar.delete(ACCESS);
  jar.delete(REFRESH);
}

export async function refresh(
  refreshToken: string,
): Promise<{ access: string; refresh: string } | null> {
  let res: Response;
  try {
    res = await fetch(`${API_BASE}/api/v1/auth/token/refresh`, {
      method: 'POST',
      headers: { ...appHeaders, 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: refreshToken }),
      cache: 'no-store',
    });
  } catch {
    // Serveri nuk u arrit: sesioni nuk është i pavlefshëm, thjesht i paverifikueshëm tani.
    return null;
  }
  if (!res.ok) return null;
  const body = (await res.json()) as { access_token?: string; refresh_token?: string };
  if (!body.access_token || !body.refresh_token) return null;
  return { access: body.access_token, refresh: body.refresh_token };
}
