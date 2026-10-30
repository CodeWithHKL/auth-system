# auth-system

A small, complete login / signup system: a Go API backed by PostgreSQL and a Svelte single-page UI. Accounts are stored with bcrypt-hashed passwords, and sessions are signed JWTs kept in an HttpOnly cookie, so the server holds no session state and can run as many replicas (or serverless instances) as you like.

**Live demo:** https://hkl-auth-system.vercel.app

## Features

- Sign up, log in, log out, and a "who am I" endpoint
- Passwords hashed with bcrypt; emails unique case-insensitively
- Stateless sessions: JWT (HS256) in an `HttpOnly`, `SameSite=Lax` cookie, `Secure` over HTTPS
- Login takes the same time for unknown and known emails, so response time does not reveal which emails are registered
- Login rate limiting: 10 failed attempts per email and 20 per client IP in 15 minutes, then `429` with a `Retry-After` header
- Password strength meter (zxcvbn-ts) and confirm-password field on signup
- Embedded SQL migrations, safe to run from several instances at once
- Runs as a normal server or as a Vercel serverless function from the same code

## Stack

| Layer    | Technology                                                   |
| -------- | ------------------------------------------------------------ |
| API      | Go (standard library `net/http` mux), `pgx`, `bcrypt`, `golang-jwt` |
| Database | PostgreSQL 17 (Docker locally, Supabase in production)       |
| UI       | Svelte 5, Vite, TypeScript, zxcvbn-ts                        |
| Hosting  | Vercel (UI + Go function), Supabase (Postgres)               |

## Project layout

```
api/index.go           Vercel function entry point (all /api/* requests)
cmd/server/            standalone HTTP server
cmd/migrate/           applies database migrations
serverless/            non-internal facade the Vercel function imports (see Deployment)
internal/app/          builds the HTTP handler shared by the server and the function
internal/auth/         signup / login / logout / me handlers and JWT tokens (+ tests)
internal/config/       environment-based configuration
internal/store/        Postgres access, login-failure counters and embedded migrations
web/                   Svelte UI (Vite)
vercel.json            Vercel build + routing config
docker-compose.yml     local PostgreSQL
```

## Getting started

**Prerequisites:** Go 1.27+, Node.js and npm, Docker.

```sh
# 1. Start PostgreSQL (host port 5433, to avoid clashing with a local Postgres on 5432)
docker compose up -d --wait

# 2. Configure. The server reads real environment variables; it does not load .env itself.
cp .env.example .env        # then export the variables below in your shell
```

Bash:

```sh
export DATABASE_URL='postgres://myapp:myapp@localhost:5433/myapp?sslmode=disable'
export JWT_SECRET="$(openssl rand -base64 48)"
```

PowerShell:

```powershell
$env:DATABASE_URL = "postgres://myapp:myapp@localhost:5433/myapp?sslmode=disable"
$b = New-Object byte[] 48
[Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($b)
$env:JWT_SECRET = [Convert]::ToBase64String($b)
```

```sh
# 3. Run the API (:8080). It applies migrations on startup.
go run ./cmd/server

# 4. Run the UI (:5173). Vite proxies /api to the Go server, so the browser sees one origin.
cd web && npm install && npm run dev
```

Open http://localhost:5173.

## Configuration

| Variable               | Required | Default  | Description                                                                 |
| ---------------------- | -------- | -------- | --------------------------------------------------------------------------- |
| `DATABASE_URL`         | yes      |          | PostgreSQL connection string                                                |
| `JWT_SECRET`           | yes      |          | Signing key, at least 32 bytes. Keep it private; changing it logs everyone out |
| `COOKIE_SECURE`        | no       | `false`  | Set `true` when served over HTTPS                                           |
| `TOKEN_TTL`            | no       | `24h`    | Session lifetime, a Go duration such as `12h`                               |
| `TRUST_PROXY`          | no       | `false`  | Read the client IP from `X-Real-IP` / `X-Forwarded-For` (on automatically on Vercel). See Rate limiting |
| `ADDR`                 | no       | `:8080`  | Listen address (standalone server only)                                     |
| `MIGRATE_DATABASE_URL` | no       |          | Connection used by `cmd/migrate`; falls back to `DATABASE_URL`              |

## API

All bodies are JSON. A successful signup or login sets the `session` cookie.

| Method | Path               | Body                       | Responses                                              |
| ------ | ------------------ | -------------------------- | ------------------------------------------------------ |
| POST   | `/api/auth/signup` | `{name, email, password}`  | `201` user, `409` email taken, `422` validation errors |
| POST   | `/api/auth/login`  | `{email, password}`        | `200` user, `401` invalid email or password, `429` too many failed attempts |
| POST   | `/api/auth/logout` |                            | `204`, clears the cookie                               |
| GET    | `/api/auth/me`     |                            | `200` current user, `401` not signed in                |
| GET    | `/api/healthz`     |                            | `200` ready, `503` database unreachable                |

Validation: name 1-80 characters, a valid email address, password 8-72 bytes (bcrypt ignores anything longer). A `422` response includes a per-field `fields` object.

```sh
curl -i -c jar.txt -H 'Content-Type: application/json' \
  -d '{"name":"Ada","email":"ada@example.com","password":"correct horse"}' \
  http://localhost:8080/api/auth/signup

curl -b jar.txt http://localhost:8080/api/auth/me
```

## Rate limiting

Failed logins are counted per client IP and per email over a 15-minute window: 10 failures for one email or 20 for one IP, then `429 Too Many Requests` with a `Retry-After` header. A locked email is refused even with the correct password, and the check runs before bcrypt, so blocked guesses cost no CPU. A successful login resets that email's counter (not the IP's).

- Counters live in the `login_failures` table (migration `003`), not in memory, so every server or serverless instance shares them. Emails are stored only as SHA-256 hashes, and expired rows are purged automatically.
- The client IP is the socket address unless `TRUST_PROXY=true` (set automatically when `VERCEL=1`). Only enable it behind a proxy that overwrites `X-Real-IP` / `X-Forwarded-For`; otherwise clients could choose their own IP and dodge the limit.
- Limits to be aware of: each IPv6 address is counted separately, signup is not rate limited, and an attacker can lock a victim's email for 15 minutes by failing on purpose.

## Tests

```sh
go test ./...                 # handler tests use an in-memory store, so no database is needed
TEST_DATABASE_URL='postgres://myapp:myapp@localhost:5433/myapp?sslmode=disable' go test ./internal/store   # limiter SQL against a real database
cd web && npm run check       # svelte-check + TypeScript
```

## Deployment: Vercel + Supabase

One Vercel project serves the UI (`web/dist`) and the API (`api/index.go`, a Go function that `vercel.json` rewrites `/api/*` to), so the session cookie stays same-origin.

1. **Supabase:** create a project (pick a region close to your users, and run the Vercel function in the same region). From **Connect**, copy two connection strings:
   - Session pooler (port 5432) for `MIGRATE_DATABASE_URL`
   - Transaction pooler (port 6543) for `DATABASE_URL`, with `?default_query_exec_mode=simple_protocol` appended. The transaction pooler does not support prepared statements, and without this suffix signup and login fail with a 500 while `/api/healthz` still looks healthy.
2. **Migrate** once, and again whenever you add a migration. This also enables Row Level Security on `users` and `login_failures`, so Supabase's public API cannot read them:
   ```sh
   MIGRATE_DATABASE_URL='<session pooler string>' go run ./cmd/migrate
   ```
   Run it **before** deploying code that needs a new table: login reads `login_failures`, and returns 500 if it does not exist yet.
3. **Vercel:** import the repo (root directory `./`, preset "Other") and set the environment variables `DATABASE_URL`, `JWT_SECRET` (32+ random bytes) and `COOKIE_SECURE=true`. Build settings and the function region (`sin1`) come from `vercel.json`; the Go version is read from `go.mod`. Redeploy after changing any variable.
4. **Check:** open `/api/healthz` (expect 200), then sign up and log in.

Things worth knowing:

- Vercel compiles `api/index.go` as its own package, outside this module's path, so it cannot import `internal/...`. The function imports only `serverless/`, which wraps the internal packages. Keep it that way.
- Keep the function region next to the database. A function in another region adds a round trip to every query.
- Each function instance keeps its own small connection pool; the Supabase pooler is what protects Postgres. Cold starts add latency to the first request.

## Security notes

- Passwords are bcrypt-hashed and never returned by the API.
- The session cookie is `HttpOnly` (not readable by page scripts) and `SameSite=Lax`; set `COOKIE_SECURE=true` behind HTTPS.
- Anyone who knows `JWT_SECRET` can forge a session for any user. Store it only in your host's environment settings, never in git, and rotate it if it leaks (all sessions then end).
- Login attempts are rate limited per IP and per email (see Rate limiting).
- Row Level Security is enabled on `users` and `login_failures` for hosted Postgres such as Supabase. The Go server connects as the table owner, which bypasses it.
