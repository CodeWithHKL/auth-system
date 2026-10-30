-- Hosted Postgres such as Supabase exposes tables in the public schema through
-- an HTTP API. With RLS on and no policies, that API can read nothing from
-- users (it holds password hashes). The Go server connects as the table owner
-- (e.g. the postgres role), which bypasses RLS, so it is unaffected.
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
