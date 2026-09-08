# RC.36 frontend / product conflict audit

## Evidence

Compared `origin/develop` (Molii) with `v1.0.0-rc.36`. Their merge base is
`27ff6a8767e728f879d52770c273d4f73214a430` (`v1.0.0-rc.30`), so this covers
the entire rc.31--rc.36 upstream delta.

```sh
git merge-base origin/develop v1.0.0-rc.36
git diff --name-status v1.0.0-rc.30..v1.0.0-rc.36 -- web docs
git merge-tree --write-tree --messages origin/develop v1.0.0-rc.36
comm -12 <(git diff --name-only BASE..origin/develop | sort) \
  <(git diff --name-only BASE..v1.0.0-rc.36 | sort)
```

The virtual merge found **46 frontend content/modify-delete conflicts** and
68 paths touched by both branches; 22 shared paths auto-merge but need a
semantic review. This audit did not modify source files.

Upstream release landmarks:

| interval | product-facing commits |
| --- | --- |
| rc.30 -> rc.31 | `9df450fe5` task/plugin polling; `057f71c23` privileged log metadata |
| rc.31 -> rc.32 | `7c044d7c5` explicit `@` models / canonical billing identity |
| rc.32 -> rc.33 | no web/docs commits |
| rc.33 -> rc.34 | `0c76e4dae` models/vendors/pricing; `6f2333990`, `0973dc2b8`, `a8729b5c3`, `45c3fbe8`, `d8cb17744` auth/security/audit |
| rc.34 -> rc.35 | `bee45b58a`, `387a40914` pricing grid and drawer/test fixes |
| rc.35 -> rc.36 | `ea7cb0ba4` tables/quota, `71c1fd7ca` models, `0e0ba152b` pricing/logs, `eb76b136b` channels, `984330920` plugins, `75e533209` currency, `8f72ecbbf` group filter |

## Critical

### Models, vendors, and marketplace management

Exact textual conflicts (14):

```text
web/src/features/models/api.ts
web/src/features/models/components/dialogs/sync-wizard-dialog.tsx (Molii deleted / upstream modified)
web/src/features/models/components/dialogs/upstream-conflict-dialog.tsx (Molii deleted / upstream modified)
web/src/features/models/components/dialogs/vendor-mutate-dialog.tsx
web/src/features/models/components/drawers/model-mutate-drawer.tsx
web/src/features/models/components/models-{columns,dialogs,primary-buttons,provider,table}.tsx
web/src/features/models/{constants.ts,index.tsx,types.ts}
web/src/features/models/lib/model-utils.ts
```

Upstream's `0c76e4dae` adds model modalities/capabilities, marketplace state,
vendor-linked models, vendor operations, model deletion, price sync and square
visibility filtering. It retains an upstream metadata-sync flow. Molii instead
removed that flow and made local metadata authoritative: explicit model/vendor
display order, publication fields and local marketplace editors. In `api.ts`,
Molii removes `syncUpstream`/preview/overwrite in favor of
`get/saveModelOrder` and `get/saveVendorOrder`; the opposing `types.ts` diffs
also remove/add incompatible sync fields.

**Adopt/preserve:** keep Molii's local metadata, publication and ordering, and
keep the deleted sync dialogs deleted. Port upstream vendor/listing UX only
after mapping every endpoint to the fork backend. Recompose provider/table/
columns/buttons/dialog registry around that contract; do not take either side
of a file wholesale. Add upstream type fields only when backend responses
actually supply them.

**Risk: Critical.** Taking upstream revives intentionally retired calls;
taking Molii loses new vendor operations and model visibility behavior.

### Auth, account security, and secret disclosure

Upstream adds `web/src/features/security/**`, route
`web/src/routes/_authenticated/security/index.tsx`, and a scoped,
session/action-bound secure-verification contract. It moves sensitive profile
controls, changes `password-encryption.ts` to key-id-bound hybrid encryption
for long passwords, and makes protected `route.tsx` await
`resolveAuthentication()`. It also changes `channels/api.ts#getChannelKey`
from optional `proofToken` to required proof token.

Molii's `web/src/features/auth/auth-layout.tsx` is a branding/layout change,
not a contract fork; preserve it while porting upstream auth/security behavior.

**Adopt/preserve:** adopt the Security module/route, proof validation, login
verification, OAuth callback and encryption changes. Preserve Molii branding.
Update all secret-disclosure callsites to obtain and pass a valid scoped proof;
remove stale profile-security imports after the module move.

**Risk: Critical.** Optional legacy proof calls can fail disclosure or bypass
the intended proof flow when combined with the upgraded backend.

## High

### API keys

Exact textual conflicts (8):

```text
web/src/features/keys/components/__tests__/api-key-group-cell.test.tsx
web/src/features/keys/components/__tests__/api-key-group-combobox.test.tsx (Molii deleted / upstream modified)
web/src/features/keys/components/__tests__/auto-group-order-editor.test.tsx
web/src/features/keys/components/{api-key-group-cell,api-key-timestamp-cell,api-keys-cells,api-keys-columns,api-keys-table}.tsx
```

Upstream rc.36 extracts a quota cell, uses site currency/masked-value cells,
and refines activity plus desktop/mobile restrictions. Molii adds ordered
automatic routing, group success-rate/icon metadata, provider/model icons,
default keys, IP CIDR validation/editing and key rotation.

**Adopt/preserve:** retain all Molii routing/IP/model-display props; introduce
upstream quota/table primitives through those props. Keep the deleted old
combobox deleted and test its Molii replacement. Recheck selection/bulk actions
because upstream and Molii both change key row selection semantics.

### Usage logs and audit records

Exact unique source conflicts:

```text
web/src/features/usage-logs/components/columns/common-logs-columns.tsx
web/src/features/usage-logs/components/common-logs-filter-bar.tsx
web/src/features/usage-logs/components/dialogs/details-dialog.tsx
web/src/features/usage-logs/components/usage-logs-mobile-card.tsx
```

Upstream adds `usage-logs/audit/**`, `/usage-logs/audit`, quota-operation
rendering, searchable group filter, shared mobile common-log cards and
privileged metadata isolation. Molii adds GPT Image 2, Grok Image/Grok Video/
Seedance billing states, previews/downloads, trusted asset handling and
task-family source routing.

**Adopt/preserve:** add upstream audit UI/filter/mobile structure while keeping
every Molii source-registry route and AIGC preview/billing branch. Details and
columns must compose both formatter families; neither complete version is
safe.

### Pricing and public model directory

Exact textual conflicts:

```text
web/src/features/pricing/components/dynamic-pricing-breakdown.tsx
web/src/features/pricing/components/{model-card-grid,model-card,model-details,pricing-columns,pricing-sidebar,pricing-toolbar}.tsx
web/src/features/pricing/index.tsx
```

Upstream adds site-currency editors, task price display, three-column desktop
cards and visibility/filter controls. Molii has a different directory:
vendor ordering/groups, high-density catalog, continuous details, capability
metadata, Grok/image/video matrices, CNY tiers and custom API samples.

**Adopt/preserve:** preserve the Molii directory and task matrices. Port
upstream currency semantics and visibility/filter data only where supported by
the fork backend. Shared `lib/dynamic-price.ts` and `types.ts` auto-merge but
need manual duplicate-normalization review.

### Routes, sidebar and generated route tree

`web/src/hooks/use-sidebar-data.ts` conflicts. Molii adds
`/temporary-assets`, super-admin `/product-quotation`, and switches generation
records to `GENERATION_LOG_DEFAULT_PATH`; upstream adds `/security` and
`/usage-logs/audit`. `routeTree.gen.ts` auto-merges but is generated output.

**Adopt/preserve:** retain all routes/guards below and regenerate the tree
from route files, rather than hand-resolving generated output.

```text
/product-quotation     Molii, SUPER_ADMIN
/temporary-assets      Molii
/usage-logs/audit      upstream
/security              upstream
```

Test that `/usage-logs/audit` wins over the dynamic `/$section` route.

## Medium

### Channels and task plugins

The direct conflict is
`web/src/features/channels/components/drawers/channel-mutate-drawer.tsx`.
Auto-merged `channels/api.ts` and `types.ts` now add `task_plugin_key` and
plugin metadata (`sortPriority`, website, icon, `baseUrl`, `hasIcon`). Adopt
upstream rc.36 task-plugin marketplace/icon/website files first, then bridge
the drawer to Molii Grok/StarAI configuration and fetched-model categories.
Preserve Molii `molii-grok-channel` and `starai-channel` tests and add a
plugin-channel/Molii-field integration test.

### All locales require object-level merge

Exact textual conflicts:

```text
web/src/i18n/locales/{en,fr,ja,ru,vi,zh-TW,zh}.json
```

Molii adds quotation, AIGC/Grok, catalogue, key-routing and branded-auth copy;
upstream adds Security, audit, vendor operations, plugin metadata, quota and
filtering copy. Keep both key sets and Molii localized quotation exports, then
run `i18n:check`. Auto-merged `static-keys.ts` still needs the same check.

### Layout and test harness

Direct conflicts: `components/layout/components/mobile-drawer.tsx`,
`components/profile-dropdown.tsx`, `test-setup.ts`. Preserve Molii wordmark,
favicon, public/auth design and charcoal styling; adopt upstream drawer portal
interactivity/storage test shim. Check that Security works from mobile drawer
and profile dropdown.

## Low: shared auto-merges needing review

```text
docs/authentication.md
web/src/features/channels/{api.ts,types.ts}
web/src/features/keys/components/data-table-row-actions.tsx
web/src/features/performance-metrics/types.ts
web/src/features/pricing/{lib/dynamic-price.ts,types.ts}
web/src/features/profile/components/sidebar-modules-card.tsx
web/src/features/system-settings/{models/group-ratio-visual-editor.tsx,models/ratio-settings-card.tsx,types.ts}
web/src/features/usage-logs/{components/timing-metrics-cell.tsx,components/usage-logs-table.tsx,constants.ts,index.tsx,lib/format.ts,types.ts}
web/src/i18n/static-keys.ts
web/src/{lib/api.ts,routeTree.gen.ts,styles/index.css}
web/vitest.config.ts
```

Update `docs/authentication.md` for required proofs/OAuth/encryption without
dropping Molii access guidance. Do visual smoke testing for `styles/index.css`.

## Molii product surface to preserve

1. Wordmark/favicon/header/home/footer/auth branding.
2. Super-admin quotation/calculator, drafts and HTML export.
3. Temporary assets plus GPT Image 2/Grok media preview/download flows.
4. Local marketplace metadata/order/publication and retired external-sync flow.
5. Molii AIGC/Grok/StarAI price/task behavior, CNY units, API-key provider
   routing, ordered groups and CIDR restrictions.

## Integration order and staged verification

1. Resolve backend-compatible model/vendor APIs and security proof contract.
2. Add Security/Audit routes, preserve quotation/temp-assets, regenerate router
   tree, and smoke-test anonymous/authenticated/super-admin/mobile navigation.
3. Recompose models/pricing, then keys, then logs; never resolve entire
   directories with `ours` or `theirs`.
4. Object-merge locales and run visual/browser checks.

Commands from `web/package.json`:

```sh
bun --cwd web run typecheck
bun --cwd web run lint
bun --cwd web run i18n:check
bun --cwd web run test -- features/auth features/security
bun --cwd web run test -- features/models features/pricing
bun --cwd web run test -- features/keys features/usage-logs
bun --cwd web run test -- features/task-plugins features/channels
bun --cwd web run build:check
```

Include upstream security/audit/model/vendor/plugin/API-key/log tests and the
Molii quotation, temporary-assets, Grok/StarAI, marketplace-ordering and i18n
tests. Finish with a clean merge-tree run and a browser smoke pass of all
protected routes.
