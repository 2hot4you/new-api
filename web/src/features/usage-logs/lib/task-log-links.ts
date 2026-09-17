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
const TASK_LOG_TIME_MARGIN_SECONDS = 5 * 60

export function buildUsageLogRequestLink(
  requestId: string,
  submitTime?: number,
  finishTime?: number
): string {
  const search = new URLSearchParams({ requestId })
  if (submitTime && submitTime > 0) {
    search.set(
      'startTime',
      String(Math.max(0, submitTime - TASK_LOG_TIME_MARGIN_SECONDS) * 1000)
    )
    const boundedFinish =
      finishTime && finishTime >= submitTime ? finishTime : submitTime
    search.set(
      'endTime',
      String((boundedFinish + TASK_LOG_TIME_MARGIN_SECONDS) * 1000)
    )
  }
  return `/usage-logs/common?${search.toString()}`
}
