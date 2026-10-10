import { api } from '@/lib/api'
import { requireServerSuccess } from '@/lib/server-error-message'

import type {
  CatalogBoundOperation,
  CatalogHistoryPage,
  CatalogOperation,
  CatalogOperationBinding,
  CatalogSyncPlan,
  CatalogSyncResult,
  CatalogSyncStatus,
} from './types'

type Response<T> = {
  success: boolean
  code?: string
  message?: string
  data: T
}
const base = '/api/catalog_sync'

export async function getCatalogSyncStatus(): Promise<CatalogSyncStatus> {
  const response = await api.get<Response<CatalogSyncStatus>>(`${base}/status`)
  return requireServerSuccess(response.data).data
}

export async function previewCatalogSync(): Promise<CatalogSyncPlan> {
  const response = await api.post<Response<CatalogSyncPlan>>(
    `${base}/preview`,
    {}
  )
  return requireServerSuccess(response.data).data
}

export async function resolveCatalogSyncPlan(
  planId: string,
  digest: string,
  overwriteKeys: string[],
  confirmDeletes: boolean
): Promise<CatalogSyncPlan> {
  const response = await api.post<Response<CatalogSyncPlan>>(
    `${base}/plans/${encodeURIComponent(planId)}/resolve`,
    { digest, overwrite_keys: overwriteKeys, confirm_deletes: confirmDeletes }
  )
  return requireServerSuccess(response.data).data
}

export async function applyCatalogSyncPlan(
  binding: CatalogOperationBinding,
  proofToken: string
): Promise<CatalogSyncResult> {
  const response = await api.post<Response<CatalogSyncResult>>(
    `${base}/plans/${encodeURIComponent(binding.planId)}/apply`,
    {
      plan_id: binding.planId,
      digest: binding.digest,
      operation_id: binding.operationId,
    },
    {
      headers: { 'X-Security-Proof': proofToken },
      singleUseAuthorization: true,
    }
  )
  return requireServerSuccess(response.data).data
}

export async function listCatalogSyncHistory(
  offset = 0,
  limit = 20
): Promise<CatalogHistoryPage> {
  const response = await api.get<Response<CatalogHistoryPage>>(
    `${base}/history`,
    {
      params: { offset, limit: Math.min(limit, 100) },
    }
  )
  return requireServerSuccess(response.data).data
}

export function getCatalogSyncOperation(
  operationId: string,
  binding: CatalogOperationBinding
): Promise<CatalogBoundOperation>
export function getCatalogSyncOperation(
  operationId: string
): Promise<CatalogOperation>
export async function getCatalogSyncOperation(
  operationId: string,
  binding?: CatalogOperationBinding
): Promise<CatalogOperation | CatalogBoundOperation> {
  const response = await api.get<
    Response<CatalogOperation | CatalogBoundOperation>
  >(
    `${base}/operations/${encodeURIComponent(operationId)}`,
    binding
      ? { params: { plan_id: binding.planId, digest: binding.digest } }
      : undefined
  )
  return requireServerSuccess(response.data).data
}

export async function previewCatalogSyncRestore(
  operationId: string
): Promise<CatalogSyncPlan> {
  const response = await api.post<Response<CatalogSyncPlan>>(
    `${base}/operations/${encodeURIComponent(operationId)}/restore-preview`,
    {}
  )
  return requireServerSuccess(response.data).data
}
