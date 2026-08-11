-- Molii user-owned Files API PostgreSQL migration
-- Target: PostgreSQL 15+. Safe to run repeatedly.

BEGIN;

SELECT pg_advisory_xact_lock(hashtext('molii-new-api-20260811-molii-files'));

CREATE TABLE IF NOT EXISTS public.molii_files (
  id bigserial,
  file_id varchar(191),
  user_id bigint,
  object_key varchar(512),
  filename varchar(255),
  purpose varchar(64),
  bytes bigint,
  mime_type varchar(127),
  media_type varchar(16),
  width bigint,
  height bigint,
  duration_seconds double precision,
  status varchar(20),
  created_at bigint,
  updated_at bigint,
  expires_at bigint,
  PRIMARY KEY (id)
);

ALTER TABLE public.molii_files ADD COLUMN IF NOT EXISTS width bigint;
ALTER TABLE public.molii_files ADD COLUMN IF NOT EXISTS height bigint;
ALTER TABLE public.molii_files ADD COLUMN IF NOT EXISTS duration_seconds double precision;

CREATE UNIQUE INDEX IF NOT EXISTS idx_molii_files_file_id
  ON public.molii_files (file_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_molii_files_object_key
  ON public.molii_files (object_key);

CREATE INDEX IF NOT EXISTS idx_molii_files_user_status_expiry
  ON public.molii_files (user_id, status, expires_at);

COMMIT;
