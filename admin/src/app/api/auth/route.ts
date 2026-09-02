import { cookies } from 'next/headers';
import { NextRequest, NextResponse } from 'next/server';

import { API_BASE, appHeaders, clearTokens, readTokens, writeTokens } from '@/lib/session';

/// Kyçja dhe dalja e stafit. Kodi njëpërdorimësh është i njëjti mekanizëm si te aplikacionet;
/// dallimi është se serveri i lejon këto endpoint-e vetëm llogarive me kapacitete stafi (§37, §52).

type Action = 'request' | 'verify' | 'logout' | 'me';

/// Kur serveri nuk arrihet fare, kthehet i njëjti zarf gabimi si kudo tjetër, jo një faqe e Next-it.
const OFFLINE = { error: { code: 'OFFLINE', message_key: 'errors.offline', http_status: 503 } };

async function call(path: string, init: RequestInit): Promise<Response> {
  try {
    return await fetch(`${API_BASE}/api/v1/${path}`, {
      ...init,
      headers: { ...appHeaders, 'Content-Type': 'application/json', ...(init.headers ?? {}) },
      cache: 'no-store',
    });
  } catch {
    return new Response(JSON.stringify(OFFLINE), {
      status: 503,
      headers: { 'content-type': 'application/json' },
    });
  }
}

/// Përgjigjja e serverit kalon e paprekur, që faqja të tregojë arsyen e vërtetë.
async function passthrough(res: Response) {
  const text = await res.text();
  return new NextResponse(text || null, {
    status: res.status,
    headers: { 'content-type': res.headers.get('content-type') ?? 'application/json' },
  });
}

export async function POST(req: NextRequest) {
  const jar = await cookies();
  const { action, phone, code } = (await req.json()) as {
    action: Action;
    phone?: string;
    code?: string;
  };

  if (action === 'request') {
    return passthrough(
      await call('auth/otp/request', { method: 'POST', body: JSON.stringify({ phone }) }),
    );
  }

  if (action === 'verify') {
    const res = await call('auth/otp/verify', {
      method: 'POST',
      body: JSON.stringify({ phone, code, device_name: 'Paneli i Operacioneve' }),
    });
    if (!res.ok) return passthrough(res);

    const pair = (await res.json()) as { access_token: string; refresh_token: string };
    writeTokens(jar, pair.access_token, pair.refresh_token);

    // Profili merret këtu që faqja të mos i shohë token-at as për një thirrje të vetme.
    const me = await call('users/me', {
      method: 'GET',
      headers: { Authorization: `Bearer ${pair.access_token}` },
    });
    return passthrough(me);
  }

  if (action === 'logout') {
    const { refresh: rt } = await readTokens();
    if (rt) {
      // Dalja lokale ndodh edhe nëse serveri nuk përgjigjet.
      await call('auth/logout', { method: 'POST', body: JSON.stringify({ refresh_token: rt }) }).catch(
        () => undefined,
      );
    }
    clearTokens(jar);
    return NextResponse.json({ ok: true });
  }

  return NextResponse.json({ error: { code: 'NOT_FOUND', message_key: 'errors.not_found' } }, { status: 404 });
}
