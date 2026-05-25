export class ApiError extends Error {
  code?: string;
  status: number;

  constructor(message: string, status: number, code?: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }
}

export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const isFormData = init?.body instanceof FormData;
  const res = await fetch(path, {
    ...init,
    headers: isFormData ? init?.headers : { 'Content-Type': 'application/json', ...(init?.headers ?? {}) }
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    const envelope = body.error && typeof body.error === 'object' ? body.error : body;
    const message = typeof body.error === 'string'
      ? body.error
      : typeof envelope.message === 'string'
        ? envelope.message
        : `${res.status} ${res.statusText}`;
    const code = typeof envelope.code === 'string' ? envelope.code : undefined;
    throw new ApiError(message, res.status, code);
  }
  return res.json();
}
