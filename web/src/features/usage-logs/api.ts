/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { api } from '@/lib/api'

import {
  getVideoPlatformForSource,
  type UsageLogSource,
  type VideoLogSource,
} from './source-registry'
import type {
  GetLogsParams,
  GetLogsResponse,
  GetLogStatsParams,
  GetLogStatsResponse,
  GetMidjourneyLogsParams,
  GetTaskLogsParams,
  UserInfo,
} from './types'

function buildQueryParams(params: Record<string, unknown>): URLSearchParams {
  const queryParams = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== '') {
      queryParams.append(key, String(value))
    }
  }
  return queryParams
}

// ============================================================================
// Generic API Helpers
// ============================================================================

function buildApiPath(endpoint: string, isAdmin: boolean): string {
  return isAdmin ? endpoint : `${endpoint}/self`
}

interface LogApiRequest<T> {
  path: string
  params: T
}

export function buildGrokImageLogRequest(
  params: GetLogsParams,
  isAdmin: boolean
): LogApiRequest<GetLogsParams> {
  return {
    path: buildApiPath('/api/log', isAdmin),
    params: { ...params, log_category: 'grok_image' },
  }
}

export function buildMidjourneyLogRequest(
  params: GetMidjourneyLogsParams,
  isAdmin: boolean
): LogApiRequest<GetMidjourneyLogsParams> {
  return { path: buildApiPath('/api/mj', isAdmin), params }
}

export function buildVideoTaskLogRequest(
  params: GetTaskLogsParams,
  isAdmin: boolean,
  source: UsageLogSource
): LogApiRequest<GetTaskLogsParams> {
  const platform = getVideoPlatformForSource(source)
  if (!platform) {
    throw new Error(`Unsupported video log source: ${source}`)
  }
  return {
    path: buildApiPath('/api/task', isAdmin),
    params: { ...params, platform },
  }
}

async function fetchLogs<T>(
  endpoint: string,
  params: T,
  isAdmin: boolean
): Promise<GetLogsResponse> {
  const paramRecord = params as unknown as Record<string, unknown>
  const queryParams = buildQueryParams({
    p: paramRecord.p || 1,
    page_size: paramRecord.page_size || 20,
    ...params,
  })
  const path = buildApiPath(endpoint, isAdmin)
  const res = await api.get(`${path}?${queryParams}`)
  return res.data
}

async function fetchLogsRequest<T>(
  request: LogApiRequest<T>
): Promise<GetLogsResponse> {
  const paramRecord = request.params as unknown as Record<string, unknown>
  const queryParams = buildQueryParams({
    p: paramRecord.p || 1,
    page_size: paramRecord.page_size || 20,
    ...request.params,
  })
  const res = await api.get(`${request.path}?${queryParams}`)
  return res.data
}

async function fetchLogStats<T>(
  endpoint: string,
  params: T,
  isAdmin: boolean
): Promise<GetLogStatsResponse> {
  const queryParams = buildQueryParams(
    params as unknown as Record<string, unknown>
  )
  const path = buildApiPath(endpoint, isAdmin)
  const res = await api.get(`${path}/stat?${queryParams}`)
  return res.data
}

// ============================================================================
// Common Log APIs
// ============================================================================

export const getAllLogs = (params: GetLogsParams = {}) =>
  fetchLogs('/api/log', params, true)

export const getUserLogs = (
  params: Omit<GetLogsParams, 'username' | 'channel'> = {}
) => fetchLogs('/api/log', params, false)

export const getAllGrokImageLogs = (params: GetLogsParams = {}) =>
  fetchLogsRequest(buildGrokImageLogRequest(params, true))

export const getUserGrokImageLogs = (
  params: Omit<GetLogsParams, 'username' | 'channel'> = {}
) => fetchLogsRequest(buildGrokImageLogRequest(params, false))

export const getLogStats = (params: GetLogStatsParams = {}) =>
  fetchLogStats('/api/log', params, true)

export const getUserLogStats = (
  params: Omit<GetLogStatsParams, 'username' | 'channel'> = {}
) => fetchLogStats('/api/log', params, false)

export async function getUserInfo(
  userId: number
): Promise<{ success: boolean; message?: string; data?: UserInfo }> {
  const res = await api.get(`/api/user/${userId}`)
  return res.data
}

// ============================================================================
// MjProxy (Drawing) Logs API
// ============================================================================

export const getAllMidjourneyLogs = (params: GetMidjourneyLogsParams) =>
  fetchLogsRequest(buildMidjourneyLogRequest(params, true))

export const getUserMidjourneyLogs = (params: GetMidjourneyLogsParams) =>
  fetchLogsRequest(buildMidjourneyLogRequest(params, false))

// ============================================================================
// Task Logs API
// ============================================================================

export const getAllTaskLogs = (params: GetTaskLogsParams) =>
  fetchLogs('/api/task', params, true)

export const getUserTaskLogs = (params: GetTaskLogsParams) =>
  fetchLogs('/api/task', params, false)

export const getAllVideoTaskLogs = (
  params: GetTaskLogsParams,
  source: VideoLogSource
) => fetchLogsRequest(buildVideoTaskLogRequest(params, true, source))

export const getUserVideoTaskLogs = (
  params: GetTaskLogsParams,
  source: VideoLogSource
) => fetchLogsRequest(buildVideoTaskLogRequest(params, false, source))
