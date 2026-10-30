# myapp: login / signup

Go API + PostgreSQL + Svelte glassmorphism UI (Rajdhani headings, Barlow body).

## Run

```sh
docker compose up -d --wait          # PostgreSQL
cp .env.example .env                 # then export these vars in your shell
export DATABASE_URL=... JWT_SECRET=...
go run ./cmd/server                  # API on :8080, creates the schema on start
cd web && npm install && npm run dev # UI on :5173, proxies /api to :8080
```

## API

| Method | Path               | Notes                                       |
| ------ | ------------------ | ------------------------------------------- |
| POST   | `/api/auth/signup` | `{name,email,password}` → 201, sets session |
| POST   | `/api/auth/login`  | `{email,password}` → 200, sets session      |
| POST   | `/api/auth/logout` | clears the session cookie                   |
| GET    | `/api/auth/me`     | current user, or 401                        |
| GET    | `/healthz`         | readiness (pings the database)              |

## Scaling notes

- Sessions are signed JWTs in an HttpOnly, SameSite=Lax cookie, so the server keeps no
  session state: run any number of replicas with the same `JWT_SECRET`.
- Postgres access goes through a `pgxpool` connection pool; migrations are embedded and
  guarded by an advisory lock so concurrent starts are safe.
- Passwords are bcrypt-hashed; emails are unique case-insensitively.
- Not included yet: rate limiting / lockout on login, email verification, password reset,
  token revocation. Set `COOKIE_SECURE=true` behind HTTPS.
