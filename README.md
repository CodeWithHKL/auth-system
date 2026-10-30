# myapp: login / signup

Go API + PostgreSQL + Svelte glassmorphism UI (Rajdhani headings, Barlow body).

## Run

```sh
docker compose up -d --wait          # PostgreSQL
cp .env.example .env                 # then export these vars in your shell
export DATABASE_URL=... JWT_SECRET=...
go run ./cmd/server                  # API on :8080, applies migrations on start
cd web && npm install && npm run dev # UI on :5173, proxies /api to :8080
```

## API

| Method | Path               | Notes                                       |
| ------ | ------------------ | ------------------------------------------- |
| POST   | `/api/auth/signup` | `{name,email,password}` → 201, sets session |
| POST   | `/api/auth/login`  | `{email,password}` → 200, sets session      |
| POST   | `/api/auth/logout` | clears the session cookie                   |
| GET    | `/api/auth/me`     | current user, or 401                        |
| GET    | `/api/healthz`     | readiness (pings the database)              |

## Scaling notes

- Sessions are signed JWTs in an HttpOnly, SameSite=Lax cookie, so the server keeps no
  session state: run any number of replicas with the same `JWT_SECRET`.
- Postgres access goes through a `pgxpool` connection pool; migrations are embedded and
  guarded by an advisory lock so concurrent starts are safe.
- Passwords are bcrypt-hashed; emails are unique case-insensitively.
- Not included yet: rate limiting / lockout on login, email verification, password reset,
  token revocation. Set `COOKIE_SECURE=true` behind HTTPS.

## Deploy: Vercel + Supabase

One Vercel project serves the UI (`web/dist`) and the API (`api/index.go`, a Go
function that `vercel.json` rewrites `/api/*` to), so the session cookie stays same-origin.

1. **Supabase:** create a project. From *Connect*, copy two connection strings:
   - *Direct* or *Session pooler* (port 5432) -> `MIGRATE_DATABASE_URL`
   - *Transaction pooler* (port 6543) -> `DATABASE_URL`, with
     `?default_query_exec_mode=simple_protocol` appended (the transaction pooler
     does not support prepared statements).
2. **Migrate** once, and again whenever you add a migration:
   `MIGRATE_DATABASE_URL=... go run ./cmd/migrate`
   This also enables Row Level Security on `users`, so Supabase's public API
   cannot read password hashes.
3. **Vercel:** import the repo and set these environment variables:
   `DATABASE_URL`, `JWT_SECRET` (32+ bytes), `COOKIE_SECURE=true`.
   The framework preset is configured in `vercel.json`; the Go version is read
   from `go.mod`.

Serverless caveats: each function instance opens its own small pool (the
pooler is what protects Postgres), and cold starts add latency to the first request.
