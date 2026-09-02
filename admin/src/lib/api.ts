/// Klienti i shfletuesit. Flet vetëm me proxy-n e vet, kurrë me API-në drejtpërdrejt,
/// ndaj nuk njeh as adresë serveri as token (§53).

export class ApiError extends Error {
  constructor(
    readonly code: string,
    readonly messageKey: string,
    readonly status: number,
    readonly fields: Record<string, string> = {},
  ) {
    super(code);
    this.name = 'ApiError';
  }

  get isUnauthorized() {
    return this.status === 401 || this.code === 'UNAUTHORIZED' || this.code === 'SESSION_INVALID';
  }

  get isForbidden() {
    return this.status === 403;
  }
}

/// Zarfi i gabimit është një i vetëm në gjithë API-në; këtu lexohet dhe kudo tjetër përkthehet.
export async function toApiError(res: Response): Promise<ApiError> {
  let code = 'INTERNAL';
  let key = 'errors.internal';
  let fields: Record<string, string> = {};
  try {
    const body = (await res.json()) as {
      error?: { code?: string; message_key?: string; fields?: Record<string, string> };
    };
    if (body.error) {
      code = body.error.code ?? code;
      key = body.error.message_key ?? key;
      fields = body.error.fields ?? {};
    }
  } catch {
    // Përgjigje pa JSON: mbetet gabimi i përgjithshëm, pa nxjerrë tekst të papërpunuar.
  }
  return new ApiError(code, key, res.status, fields);
}

async function send<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(`/api/krejt/${path}`, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...(init.headers ?? {}) },
  });
  if (!res.ok) throw await toApiError(res);
  if (res.status === 204) return undefined as T;
  const text = await res.text();
  return (text ? JSON.parse(text) : undefined) as T;
}

export const api = {
  get: <T>(path: string, query?: Record<string, string | number | undefined>) => {
    const q = new URLSearchParams();
    for (const [k, v] of Object.entries(query ?? {})) {
      if (v !== undefined && v !== '') q.set(k, String(v));
    }
    const suffix = q.toString();
    return send<T>(suffix ? `${path}?${suffix}` : path);
  },
  post: <T>(path: string, body?: unknown) =>
    send<T>(path, { method: 'POST', body: body === undefined ? undefined : JSON.stringify(body) }),
  patch: <T>(path: string, body: unknown) =>
    send<T>(path, { method: 'PATCH', body: JSON.stringify(body) }),
  put: <T>(path: string, body: unknown) =>
    send<T>(path, { method: 'PUT', body: JSON.stringify(body) }),
  del: <T>(path: string) => send<T>(path, { method: 'DELETE' }),
};

type AuthAction = 'request' | 'verify' | 'logout';

export async function auth<T>(action: AuthAction, payload: Record<string, string> = {}): Promise<T> {
  const res = await fetch('/api/auth', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ action, ...payload }),
  });
  if (!res.ok) throw await toApiError(res);
  return (await res.json()) as T;
}
