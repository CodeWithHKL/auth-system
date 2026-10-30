export type User = { id: number; name: string; email: string; createdAt: string }

export class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
    public fields: Record<string, string> = {},
  ) {
    super(message)
  }
}

async function request<T>(path: string, init?: RequestInit & { json?: unknown }): Promise<T> {
  const { json, ...rest } = init ?? {}
  const res = await fetch(path, {
    ...rest,
    credentials: 'same-origin',
    headers: json === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: json === undefined ? undefined : JSON.stringify(json),
  })
  if (res.status === 204) return undefined as T
  const data = await res.json().catch(() => ({}))
  if (!res.ok) throw new ApiError(data.error ?? 'Something went wrong', res.status, data.fields)
  return data as T
}

export const api = {
  me: () => request<User>('/api/auth/me'),
  signup: (name: string, email: string, password: string) =>
    request<User>('/api/auth/signup', { method: 'POST', json: { name, email, password } }),
  login: (email: string, password: string) =>
    request<User>('/api/auth/login', { method: 'POST', json: { email, password } }),
  logout: () => request<void>('/api/auth/logout', { method: 'POST' }),
}
