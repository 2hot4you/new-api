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
import type { LogCleanupTaskResult } from '../types'

function toCount(value: number | undefined): number {
  return typeof value === 'number' && Number.isFinite(value) && value > 0
    ? Math.floor(value)
    : 0
}

export function getLogCleanupResultCounts(result: LogCleanupTaskResult) {
  const categorized =
    result.deleted_log_count !== undefined ||
    result.deleted_generation_count !== undefined
  const total = toCount(result.deleted_count)

  return {
    total,
    logs: categorized ? toCount(result.deleted_log_count) : total,
    generations: categorized ? toCount(result.deleted_generation_count) : 0,
    categorized,
  }
}
