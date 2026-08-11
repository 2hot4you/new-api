#!/bin/sh

set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
migration="$script_dir/20260811_molii_files.sql"
postgres_image=${POSTGRES_TEST_IMAGE:-postgres:15-alpine@sha256:3d0f7584ed7d04e27fa050d6683a74746608faf21f202be78460d679cc56461f}
container="molii-files-migration-test-$$"

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
until docker exec "$container" pg_isready --username=migration_contract --dbname=migration_contract >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 30 ]; then
    docker logs "$container" >&2
    exit 1
  fi
  sleep 1
done

run_migration() {
  docker exec --interactive "$container" psql \
    --username=migration_contract \
    --dbname=migration_contract \
    --set=ON_ERROR_STOP=1 <"$migration" >/dev/null
}

run_migration
run_migration

docker exec --interactive "$container" psql \
  --username=migration_contract \
  --dbname=migration_contract \
  --set=ON_ERROR_STOP=1 <<'SQL'
DO $contract$
DECLARE
  differences text;
BEGIN
  WITH expected(column_name, data_type, maximum_length) AS (
    VALUES
      ('id', 'bigint', NULL::integer),
      ('file_id', 'character varying', 191),
      ('user_id', 'bigint', NULL::integer),
      ('object_key', 'character varying', 512),
      ('filename', 'character varying', 255),
      ('purpose', 'character varying', 64),
      ('bytes', 'bigint', NULL::integer),
      ('mime_type', 'character varying', 127),
      ('media_type', 'character varying', 16),
      ('status', 'character varying', 20),
      ('created_at', 'bigint', NULL::integer),
      ('updated_at', 'bigint', NULL::integer),
      ('expires_at', 'bigint', NULL::integer)
  ), actual AS (
    SELECT column_name::text, data_type::text, character_maximum_length::integer
    FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'molii_files'
  ), difference AS (
    (SELECT * FROM expected EXCEPT SELECT * FROM actual)
    UNION ALL
    (SELECT * FROM actual EXCEPT SELECT * FROM expected)
  )
  SELECT string_agg(row(difference.*)::text, E'\n') INTO differences FROM difference;
  IF differences IS NOT NULL THEN
    RAISE EXCEPTION 'molii_files column contract mismatch:%', E'\n' || differences;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_indexes
    WHERE schemaname = 'public' AND tablename = 'molii_files'
      AND indexname = 'idx_molii_files_user_status_expiry'
  ) THEN
    RAISE EXCEPTION 'molii_files composite index is missing';
  END IF;
END
$contract$;
SQL

echo "Molii files migration contract passed"
