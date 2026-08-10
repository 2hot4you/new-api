-- Async task billing jobs PostgreSQL migration
-- Target: PostgreSQL 15+
-- Safe to run repeatedly before application startup or after GORM AutoMigrate.

BEGIN;

-- Serialize this migration when multiple deployment jobs start together.
SELECT pg_advisory_xact_lock(hashtext('molii-new-api-20260810-async-task-billing-jobs'));

-- Keep this definition aligned with model.TaskBillingJob. GORM intentionally
-- leaves scalar fields nullable unless a notNull tag is present.
CREATE TABLE IF NOT EXISTS public.task_billing_jobs (
  id bigserial,
  task_id bigint,
  idempotency_key varchar(191),
  operation varchar(32),
  from_quota bigint,
  target_quota bigint,
  status varchar(32),
  attempts bigint,
  next_attempt_at bigint,
  locked_by varchar(128),
  locked_until bigint,
  last_error varchar(1024),
  created_at bigint,
  updated_at bigint,
  PRIMARY KEY (id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_task_billing_jobs_task_id
  ON public.task_billing_jobs (task_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_task_billing_jobs_idempotency_key
  ON public.task_billing_jobs (idempotency_key);

CREATE INDEX IF NOT EXISTS idx_task_billing_jobs_ready
  ON public.task_billing_jobs (status, next_attempt_at);

CREATE INDEX IF NOT EXISTS idx_task_billing_jobs_expired
  ON public.task_billing_jobs (status, locked_until);

COMMIT;
