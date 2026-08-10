#!/bin/sh

set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
old_migration="$script_dir/20260803_molii_postgres.sql"
new_migration="$script_dir/20260810_async_task_billing_jobs.sql"

for migration in "$old_migration" "$new_migration"; do
  if [ ! -r "$migration" ]; then
    echo "missing migration: $migration" >&2
    exit 1
  fi
done

postgres_image=${POSTGRES_TEST_IMAGE:-postgres:15-alpine@sha256:3d0f7584ed7d04e27fa050d6683a74746608faf21f202be78460d679cc56461f}
container="molii-task-billing-migration-test-$$"

cleanup() {
  docker rm -f "$container" >/dev/null 2>&1 || true
}
trap cleanup EXIT HUP INT TERM

docker run --detach --rm \
  --name "$container" \
  --env POSTGRES_DB=migration_contract \
  --env POSTGRES_PASSWORD=migration_contract \
  --env POSTGRES_USER=migration_contract \
  "$postgres_image" >/dev/null

attempt=0
until docker exec "$container" pg_isready \
  --username=migration_contract \
  --dbname=migration_contract >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 30 ]; then
    echo "PostgreSQL test container did not become ready" >&2
    docker logs "$container" >&2
    exit 1
  fi
  sleep 1
done

run_migration() {
  docker exec --interactive "$container" psql \
    --username=migration_contract \
    --dbname=migration_contract \
    --set=ON_ERROR_STOP=1 <"$1" >/dev/null
}

# Match the deployment order, then prove the new migration is repeatable.
run_migration "$old_migration"
run_migration "$new_migration"
run_migration "$new_migration"

docker exec --interactive "$container" psql \
  --username=migration_contract \
  --dbname=migration_contract \
  --set=ON_ERROR_STOP=1 <<'SQL'
DO $contract$
DECLARE
  differences text;
BEGIN
  WITH expected(
    ordinal_position,
    column_name,
    data_type,
    is_nullable,
    character_maximum_length
  ) AS (
    VALUES
      (1,  'id',              'bigint',            'NO',  NULL::integer),
      (2,  'task_id',         'bigint',            'YES', NULL::integer),
      (3,  'idempotency_key', 'character varying', 'YES', 191),
      (4,  'operation',       'character varying', 'YES', 32),
      (5,  'from_quota',      'bigint',            'YES', NULL::integer),
      (6,  'target_quota',    'bigint',            'YES', NULL::integer),
      (7,  'status',          'character varying', 'YES', 32),
      (8,  'attempts',        'bigint',            'YES', NULL::integer),
      (9,  'next_attempt_at', 'bigint',            'YES', NULL::integer),
      (10, 'locked_by',       'character varying', 'YES', 128),
      (11, 'locked_until',    'bigint',            'YES', NULL::integer),
      (12, 'last_error',      'character varying', 'YES', 1024),
      (13, 'created_at',      'bigint',            'YES', NULL::integer),
      (14, 'updated_at',      'bigint',            'YES', NULL::integer)
  ),
  actual AS (
    SELECT
      ordinal_position,
      column_name::text,
      data_type::text,
      is_nullable::text,
      character_maximum_length::integer
    FROM information_schema.columns
    WHERE table_schema = 'public'
      AND table_name = 'task_billing_jobs'
  ),
  difference AS (
    (SELECT * FROM expected EXCEPT SELECT * FROM actual)
    UNION ALL
    (SELECT * FROM actual EXCEPT SELECT * FROM expected)
  )
  SELECT string_agg(row(difference.*)::text, E'\n' ORDER BY ordinal_position)
  INTO differences
  FROM difference;

  IF differences IS NOT NULL THEN
    RAISE EXCEPTION 'task_billing_jobs column contract mismatch:%', E'\n' || differences;
  END IF;

  IF pg_get_serial_sequence('public.task_billing_jobs', 'id') IS NULL THEN
    RAISE EXCEPTION 'task_billing_jobs.id is not backed by a serial sequence';
  END IF;

  WITH expected(index_name, is_unique, is_primary, columns) AS (
    VALUES
      ('task_billing_jobs_pkey',                  true,  true,  ARRAY['id']::text[]),
      ('idx_task_billing_jobs_task_id',           true,  false, ARRAY['task_id']::text[]),
      ('idx_task_billing_jobs_idempotency_key',   true,  false, ARRAY['idempotency_key']::text[]),
      ('idx_task_billing_jobs_ready',             false, false, ARRAY['status', 'next_attempt_at']::text[]),
      ('idx_task_billing_jobs_expired',           false, false, ARRAY['status', 'locked_until']::text[])
  ),
  actual AS (
    SELECT
      index_class.relname::text AS index_name,
      index_catalog.indisunique AS is_unique,
      index_catalog.indisprimary AS is_primary,
      array_agg(column_catalog.attname::text ORDER BY indexed_column.ordinality) AS columns
    FROM pg_index AS index_catalog
    JOIN pg_class AS table_catalog
      ON table_catalog.oid = index_catalog.indrelid
    JOIN pg_namespace AS table_namespace
      ON table_namespace.oid = table_catalog.relnamespace
    JOIN pg_class AS index_class
      ON index_class.oid = index_catalog.indexrelid
    JOIN LATERAL unnest(index_catalog.indkey)
      WITH ORDINALITY AS indexed_column(attribute_number, ordinality)
      ON true
    JOIN pg_attribute AS column_catalog
      ON column_catalog.attrelid = table_catalog.oid
      AND column_catalog.attnum = indexed_column.attribute_number
    WHERE table_namespace.nspname = 'public'
      AND table_catalog.relname = 'task_billing_jobs'
    GROUP BY index_class.relname, index_catalog.indisunique, index_catalog.indisprimary
  ),
  difference AS (
    (SELECT * FROM expected EXCEPT SELECT * FROM actual)
    UNION ALL
    (SELECT * FROM actual EXCEPT SELECT * FROM expected)
  )
  SELECT string_agg(row(difference.*)::text, E'\n' ORDER BY index_name)
  INTO differences
  FROM difference;

  IF differences IS NOT NULL THEN
    RAISE EXCEPTION 'task_billing_jobs index contract mismatch:%', E'\n' || differences;
  END IF;
END
$contract$;
SQL

echo "async task billing migration contract passed"
