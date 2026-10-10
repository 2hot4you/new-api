export type CatalogSyncRole = 'source' | 'target' | 'disabled'
export type CatalogSyncKind = 'sync' | 'restore'
export type CatalogChangeAction =
  | 'create'
  | 'update'
  | 'delete'
  | 'conflict'
  | 'preserve'
  | 'blocked'
  | 'adopt'
  | 'unchanged'
export type CatalogEntryKind =
  | 'vendor'
  | 'model'
  | 'model_price'
  | 'plugin_price'
  | 'special_price'
  | 'tool_price'

export interface CatalogSyncStatus {
  role: CatalogSyncRole
  source_id?: string
  target_id?: string
  source_ready: boolean
  target_ready: boolean
  management_ready: boolean
  error?: string
}

export interface CatalogEntry {
  kind: CatalogEntryKind
  key: string
  value: string
}

export interface CatalogChange {
  kind: CatalogEntryKind
  key: string
  action: CatalogChangeAction
  reason: string
  before: CatalogEntry | null
  base: CatalogEntry | null
  after: CatalogEntry | null
  /** Original catalogmanifest.ConfirmationUnit, projected by the server. */
  confirmation_unit: string
}

export interface CatalogSyncPlan {
  id: string
  kind: CatalogSyncKind
  restore_operation_id?: string
  source_id: string
  source_exported_at: number
  source_digest: string
  target_id: string
  digest: string
  expires_at: number
  resolution: { overwrite_keys: string[] | null; confirm_deletes: boolean }
  changes: CatalogChange[]
  executable: boolean
}

export interface CatalogSyncResult {
  operation_id: string
  state: string
  revision: number
}

export interface CatalogOperationSummary {
  kind: CatalogSyncKind
  restore_operation_id?: string
  source_id: string
  target_id: string
  actor_user_id: number
  digest: string
  actions: Record<string, number>
}

export interface CatalogOperation {
  id: string
  plan_id: string
  state: string
  revision: number
  created_at: number
  summary: CatalogOperationSummary
}

export interface CatalogHistoryPage {
  items: CatalogOperation[]
  total: number
  offset: number
  limit: number
}

export interface CatalogBoundOperation {
  receipt: CatalogSyncResult
  operation: CatalogOperation | null
}

export interface CatalogOperationBinding {
  operationId: string
  planId: string
  digest: string
}
