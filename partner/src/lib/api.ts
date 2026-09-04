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

export type MediaKind = 'merchant_logo' | 'merchant_cover' | 'product_image';

/// Ngarkimi i një imazhi publik në tri hapa: serveri nënshkruan URL-në, skedari shkon drejt
/// në bucket (jo nëpër proxy, jo nëpër API), pastaj serveri e konfirmon dhe e lidh me vendin
/// ose produktin. Kthen URL-në e re publike.
export async function uploadMedia(kind: MediaKind, targetId: string, file: File): Promise<string | null> {
  const contentType = file.type || 'image/jpeg';
  const signed = await api.post<{
    object_key: string;
    upload: { url: string; method: string; headers: Record<string, string> };
  }>('media/upload-url', {
    kind,
    target_id: targetId,
    content_type: contentType,
    size_bytes: file.size,
  });
  const put = await fetch(signed.upload.url, {
    method: signed.upload.method || 'PUT',
    headers: { ...signed.upload.headers, 'Content-Type': contentType },
    body: file,
  });
  if (!put.ok) throw new ApiError('UPLOAD_FAILED', 'errors.unavailable', put.status);
  const confirmed = await api.post<{ url: string | null }>('media', { object_key: signed.object_key });
  return confirmed.url;
}

export function removeMedia(kind: MediaKind, targetId: string): Promise<void> {
  return api.del<void>(`media/${kind}?target_id=${encodeURIComponent(targetId)}`);
}

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
