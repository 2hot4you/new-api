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
import { TASK_PLATFORMS, TASK_STATUS } from '../constants'
import type { TaskLog } from '../types'

const VIDEO_TASK_PLATFORMS: ReadonlySet<string> = new Set([
  TASK_PLATFORMS.STARAI,
  TASK_PLATFORMS.MOLII_GROK,
])

export function shouldShowGrokVideoTemporaryLinkWarning(
  task: Pick<TaskLog, 'platform'>
): boolean {
  return task.platform === TASK_PLATFORMS.MOLII_GROK
}

export function isGeneratedVideoTask(
  task: Pick<TaskLog, 'platform' | 'status'>
): boolean {
  return (
    VIDEO_TASK_PLATFORMS.has(task.platform) &&
    task.status === TASK_STATUS.SUCCESS
  )
}

export function canPreviewVideoTask(
  task: Pick<TaskLog, 'platform' | 'status' | 'result_url'>
): boolean {
  return (
    isGeneratedVideoTask(task) &&
    typeof task.result_url === 'string' &&
    task.result_url.trim().length > 0
  )
}
