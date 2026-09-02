import { cookies } from 'next/headers';
import { NextRequest, NextResponse } from 'next/server';

import { API_BASE, appHeaders, clearTokens, readTokens, refresh, writeTokens } from '@/lib/session';

/// Proxy i vetëm drejt API-së. Shfletuesi nuk e prek kurrë API-në drejtpërdrejt dhe nuk e sheh
/// kurrë token-in: këtu i vihet bearer-i nga cookie-t dhe këtu rifreskohet kur skadon (§53).
/// Zarfi i gabimit kalon i paprekur, që faqja të tregojë të njëjtën arsye që dha serveri (§55).

const HOP_BY_HOP = new Set(['host', 'connection', 'content-length', 'cookie', 'authorization']);

function unreachable(): NextResponse {
  return NextResponse.json(
    { error: { code: 'OFFLINE', message_key: 'errors.offline', http_status: 503, retryable: true } },
    { status: 503 },
  );
}

async function forward(req: NextRequest, path: string[]): Promise<NextResponse> {
  const jar = await cookies();
  const tokens = await readTokens();

  const url = new URL(`${API_BASE}/api/v1/${path.join('/')}`);
  req.nextUrl.searchParams.forEach((v, k) => url.searchParams.append(k, v));

  const body =
    req.method === 'GET' || req.method === 'HEAD' ? undefined : await req.arrayBuffer();

  const send = async (access?: string) => {
    const headers = new Headers();
    req.headers.forEach((v, k) => {
      if (!HOP_BY_HOP.has(k.toLowerCase())) headers.set(k, v);
    });
    for (const [k, v] of Object.entries(appHeaders)) headers.set(k, v);
    if (access) headers.set('Authorization', `Bearer ${access}`);
    return fetch(url, { method: req.method, headers, body, cache: 'no-store' });
  };

  let upstream: Response;
  try {
    upstream = await send(tokens.access);
  } catch {
    // Serveri nuk u arrit fare. Kthejmë të njëjtin zarf gabimi si kudo tjetër, që faqja të
    // tregojë 'nuk ka lidhje' e jo një faqe gabimi të Next-it (§55).
    return unreachable();
  }

  // Një 401 i vetëm meriton një rifreskim; nëse edhe ai dështon, sesioni ka mbaruar vërtet.
  if (upstream.status === 401 && tokens.refresh) {
    const pair = await refresh(tokens.refresh);
    if (pair) {
      writeTokens(jar, pair.access, pair.refresh);
      try {
        upstream = await send(pair.access);
      } catch {
        return unreachable();
      }
    } else {
      clearTokens(jar);
    }
  }

  const out = new NextResponse(upstream.body, { status: upstream.status });
  const type = upstream.headers.get('content-type');
  if (type) out.headers.set('content-type', type);
  out.headers.set('cache-control', 'no-store');
  return out;
}

type Ctx = { params: Promise<{ path: string[] }> };

export async function GET(req: NextRequest, ctx: Ctx) {
  return forward(req, (await ctx.params).path);
}
export async function POST(req: NextRequest, ctx: Ctx) {
  return forward(req, (await ctx.params).path);
}
export async function PATCH(req: NextRequest, ctx: Ctx) {
  return forward(req, (await ctx.params).path);
}
export async function PUT(req: NextRequest, ctx: Ctx) {
  return forward(req, (await ctx.params).path);
}
export async function DELETE(req: NextRequest, ctx: Ctx) {
  return forward(req, (await ctx.params).path);
}
