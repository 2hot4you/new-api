#!/usr/bin/env bash
# Isolated local PostgreSQL only; no host ports, DSN, or application env files.
set -Eeuo pipefail
ops=$(cd -- "$(dirname -- "$0")" && pwd)
fixture="molii-legacy-price-fixture-$(date +%s)-$$"
docker run -d --rm --name "$fixture" --network none --pull never \
  -e POSTGRES_HOST_AUTH_METHOD=trust postgres:15-alpine >/dev/null
trap 'docker stop --time 1 "$fixture" >/dev/null' EXIT
for ((i=0; i<30; i++)); do
  docker exec "$fixture" pg_isready -U postgres >/dev/null 2>&1 && break
  sleep 1
done
docker exec "$fixture" mkdir /repair
docker cp "$ops/cleanup-legacy-prices.sql" "$fixture:/repair/cleanup.sql" >/dev/null
docker cp "$ops/check-legacy-prices-original.sql" "$fixture:/repair/check-original.sql" >/dev/null
sql() { docker exec -i "$fixture" psql -U postgres -X -qAt -v ON_ERROR_STOP=1 "$@"; }
sql <<'SQL'
CREATE TABLE options (key text PRIMARY KEY, value text NOT NULL);
CREATE TABLE models (model_name text, name_rule int, billing_currency text, deleted_at timestamptz);
CREATE TABLE abilities (model text, enabled bool);
CREATE TABLE channels (models text, status int);
CREATE TABLE tasks (status text);
CREATE TABLE marketplace_order_locks (name text PRIMARY KEY, version bigint);
CREATE TABLE catalog_sync_states (id int PRIMARY KEY, publication_state text, pending_operation_id text, revision bigint, runtime_revision bigint, current_digest text);
CREATE TABLE approved_options (key text,value text);
CREATE TABLE untouched_options (key text,value text);
INSERT INTO approved_options VALUES ('permanent','must not change');
INSERT INTO untouched_options VALUES ('permanent','must not change');
SQL
reset_fixture() {
  sql <<'SQL'
TRUNCATE options,models,abilities,channels,tasks,marketplace_order_locks,catalog_sync_states;
INSERT INTO options VALUES
 ('billing_setting.billing_expr','{"claude-opus-4-6":"tier(\"old\", p * 2)","keep":"tier(\"safe\", p * 3)"}'),
 ('billing_setting.billing_mode','{"claude-opus-4-6":"tiered_expr","keep":"tiered_expr"}'),
 ('ModelRatio',' { "keep" : 2.50 } '), ('UnrelatedOption','unchanged raw text');
INSERT INTO models VALUES ('keep',0,'USD',NULL);
INSERT INTO marketplace_order_locks VALUES ('marketplace_display_order',7);
INSERT INTO catalog_sync_states VALUES (1,'ready','',7,7,'original-digest');
SQL
}
backup() {
  sql <<'SQL'
\copy (SELECT key,value FROM options WHERE key IN ('billing_setting.billing_expr','billing_setting.billing_mode') ORDER BY key) TO '/repair/options.csv' WITH (FORMAT csv,HEADER true)
SQL
}
apply() { sql -v restore=false -f /repair/cleanup.sql; }
reset_fixture
backup
sql -f /repair/check-original.sql | grep -qx t
apply
sql -f /repair/check-original.sql | grep -qx f
sql <<'SQL'
DO $$ BEGIN
 IF (SELECT value::jsonb FROM options WHERE key='billing_setting.billing_expr') <> '{"keep":"tier(\"safe\", p * 3)"}'::jsonb
 OR (SELECT value::jsonb FROM options WHERE key='billing_setting.billing_mode') <> '{"keep":"tiered_expr"}'::jsonb
 OR (SELECT value FROM options WHERE key='ModelRatio') <> ' { "keep" : 2.50 } '
 OR (SELECT value FROM options WHERE key='UnrelatedOption') <> 'unchanged raw text'
 OR (SELECT version FROM marketplace_order_locks) <> 7
 OR (SELECT current_digest FROM catalog_sync_states) <> 'original-digest'
 OR (SELECT count(*) FROM public.approved_options) <> 1
 OR (SELECT count(*) FROM public.untouched_options) <> 1
 THEN RAISE EXCEPTION 'only the two approved leaves must be removed'; END IF;
END $$;
SQL
sql -v restore=true -f /repair/cleanup.sql
sql -v restore=true -f /repair/cleanup.sql
sql -f /repair/check-original.sql | grep -qx t
sql -c "SELECT count(*)=2 FROM (SELECT value FROM options WHERE key LIKE 'billing_setting.%') o WHERE value::jsonb ? 'claude-opus-4-6'" | grep -qx t
printf 'PASS: exact cleanup, unrelated raw values, unchanged anchors, CAS restore and no-op restore\n'
for scenario in changed missing duplicate pending reference orphan; do
  reset_fixture
  case "$scenario" in
    duplicate) sql -c "UPDATE options SET value='{\"claude-opus-4-6\":\"a\",\"claude-opus-4-6\":\"b\"}' WHERE key='billing_setting.billing_expr'" ;;
    missing) sql -c "DELETE FROM options WHERE key='billing_setting.billing_expr'" ;;
    pending) sql -c "UPDATE catalog_sync_states SET publication_state='committed_pending_publish'" ;;
    reference) sql -c "INSERT INTO abilities VALUES ('claude-opus-4-6',true)" ;;
    orphan) sql -c "INSERT INTO options VALUES ('ModelPrice','{\"other-orphan\":2}')" ;;
  esac
  backup
  sql -f /repair/check-original.sql | grep -qx t
  if [[ "$scenario" == changed ]]; then sql -c "UPDATE options SET value='{}' WHERE key='billing_setting.billing_expr'"; fi
  before=$(sql -c 'SELECT md5(json_agg(o ORDER BY key)::text) FROM options o')
  if apply >/dev/null 2>&1; then printf 'FAIL: accepted unsafe scenario %s\n' "$scenario" >&2; exit 1; fi
  after=$(sql -c 'SELECT md5(json_agg(o ORDER BY key)::text) FROM options o')
  [[ "$before" == "$after" ]]
  printf 'PASS: %s rejected without option changes\n' "$scenario"
done
