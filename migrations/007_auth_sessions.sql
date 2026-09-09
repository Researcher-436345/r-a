-- 007_auth_sessions.sql
-- Email verification + server-side auth sessions + one-time auth tokens.

ALTER TABLE public.users ADD COLUMN email_verified_at timestamptz;

-- Existing users are grandfathered as verified so nothing breaks on deploy.
UPDATE public.users SET email_verified_at = created_at WHERE email_verified_at IS NULL;

-- Server-side refresh sessions. Only the SHA-256 hash of the token is stored;
-- rotation sets revoked_at + replaced_by; reuse of a revoked token revokes all
-- sessions of that user.
CREATE TABLE public.auth_sessions (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    token_hash   text NOT NULL UNIQUE,
    user_agent   text,
    ip           text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL,
    revoked_at   timestamptz,
    replaced_by  uuid
);

CREATE INDEX auth_sessions_user_id_idx ON public.auth_sessions(user_id);
CREATE INDEX auth_sessions_expires_at_idx ON public.auth_sessions(expires_at);

-- Single-use tokens for verify-email / reset-password emails.
CREATE TABLE public.auth_tokens (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    type       text NOT NULL CHECK (type IN ('verify_email', 'reset_password')),
    token_hash text NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX auth_tokens_user_id_idx ON public.auth_tokens(user_id);
