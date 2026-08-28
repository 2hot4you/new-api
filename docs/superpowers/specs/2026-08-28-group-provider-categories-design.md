# Group Provider Categories Design

## Goal

Allow administrators to associate each billing group with one or more existing model vendors, then organize the API-key group picker by those vendors without changing the token routing payload or fallback semantics.

## Data model

`group_ratio_setting.group_metadata` remains the source of group display order and icon configuration. Each entry gains an optional `vendor_ids` array containing stable vendor database IDs:

```json
{"name":"anthropic-primary","icon":"Claude.Color","vendor_ids":[3,7]}
```

Missing `vendor_ids` remains valid and means uncategorized. IDs must be positive unique integers. Vendor names, icons, and ordering are resolved from the vendor table so renames and icon changes are reflected automatically. Deleting a vendor does not mutate group metadata; unresolved IDs remain visible to administrators as invalid associations and are omitted from the user-facing category payload.

## API contract

`GET /api/user/self/groups` keeps its existing map shape and adds an optional `providers` array per group. Each provider contains `id`, `name`, `icon`, and `display_order`. Providers are returned in the administrator-managed vendor order. No vendor description or other administrative data is exposed.

## Administrator interface

The visual group-pricing table adds a model-vendor multi-select column. Options come from the existing vendor management API and show the configured vendor icon and name. A group may select multiple vendors. Missing vendor IDs render as removable invalid associations rather than being silently discarded. Changes serialize only to `GroupMetadata`, preserving the stable row identity and current focus behavior.

## API key interface

The group picker renders provider sections in vendor display order. Within each section, groups follow the existing group-pricing order. A group associated with multiple providers appears in each relevant section, but its checkbox state is shared by group name and it can occur only once in the flat fallback order. Groups with no resolved providers appear in a final Uncategorized section. Search matches provider name, group name, and group description. The selected/reorder list remains flat and unchanged.

## Compatibility

Existing group metadata without `vendor_ids` is accepted unchanged. Token create/update requests still contain the same ordered group-name array; no routing, billing, pricing, or channel behavior changes.
