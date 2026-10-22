#!/usr/bin/env bash
# Read-only metadata completeness diagnostic. Never prints price values or secrets.
set -Eeuo pipefail
[[ $(hostname) == molii && $(id -un) == molii-deploy ]]
docker run -i --rm --pull never --network molii-development_default \
  --volumes-from molii-development:ro --env-file /opt/molii/development/.env.runtime \
  --entrypoint sh postgres:15-alpine -c '
    export PGCONNECT_TIMEOUT=10 PGOPTIONS="-c default_transaction_read_only=on -c statement_timeout=15000"
    exec psql "$SQL_DSN" -X -qAt -v ON_ERROR_STOP=1
  ' <<'SQL'
BEGIN READ ONLY;
WITH stored_prices AS (
 SELECT key AS option_key, jsonb_object_keys(value::jsonb) AS model_name
 FROM options
 WHERE key IN ('AudioCompletionRatio','AudioRatio','CacheRatio','CompletionRatio','CreateCacheRatio','ImageRatio','ModelPrice','ModelRatio','billing_setting.billing_expr','billing_setting.billing_mode')
), incomplete AS (
 SELECT p.option_key,p.model_name
 FROM stored_prices p
 WHERE NOT EXISTS (
  SELECT 1 FROM models m WHERE m.model_name=p.model_name AND m.name_rule=0
   AND m.billing_currency IN ('USD','CNY') AND m.deleted_at IS NULL
 )
)
SELECT json_build_object('orphan_price_entries',count(*),'examples',(
 SELECT coalesce(json_agg(example),'[]'::json) FROM (
  SELECT option_key,model_name FROM incomplete ORDER BY option_key,model_name LIMIT 20
 ) example
)) FROM incomplete;
ROLLBACK;
SQL
