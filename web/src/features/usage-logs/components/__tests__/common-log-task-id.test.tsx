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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { describe, expect, test } from 'vitest'

import type { UsageLog } from '../../data/schema'
import { useCommonLogsColumns } from '../columns/common-logs-columns'
import { UsageLogsProvider } from '../usage-logs-provider'

const log: UsageLog = {
  id: 1,
  user_id: 7,
  created_at: 1,
  type: 2,
  content: '',
  username: 'user',
  token_name: 'token',
  model_name: 'doubao-seedance-2-5-260628',
  quota: 10,
  prompt_tokens: 0,
  completion_tokens: 100,
  use_time: 5,
  is_stream: false,
  channel: 2,
  channel_name: 'StarAI Seedance 2.5',
  token_id: 3,
  group: 'ByteDance',
  ip: '',
  other: JSON.stringify({ is_task: true, task_id: 'task_public_123' }),
  request_id: 'request-123',
  upstream_request_id: '',
}

function TaskIdProbe({ isAdmin }: { isAdmin: boolean }) {
  const columns = useCommonLogsColumns(isAdmin, false)
  const column = columns.find((candidate) => candidate.id === 'task_id')
  if (!column) return <span>missing</span>
  const Cell = column.cell
  if (typeof Cell !== 'function') return <span>invalid</span>
  return Cell({
    row: {
      original: log,
      getValue: () => undefined,
    },
  } as never)
}

function renderProbe(isAdmin: boolean) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  client.setQueryData(['user-groups'], { success: true, data: {} })
  const i18n = createInstance()
  void i18n.init({ lng: 'en', resources: {} })
  return render(
    <QueryClientProvider client={client}>
      <I18nextProvider i18n={i18n}>
        <UsageLogsProvider>
          <TaskIdProbe isAdmin={isAdmin} />
        </UsageLogsProvider>
      </I18nextProvider>
    </QueryClientProvider>
  )
}

describe('common usage log task id', () => {
  test('renders the platform task id for administrators', () => {
    renderProbe(true)
    expect(screen.getByText('task_public_123')).toBeInTheDocument()
  })

  test('renders the public platform task id for ordinary users too', () => {
    renderProbe(false)
    expect(screen.getByText('task_public_123')).toBeInTheDocument()
    expect(screen.queryByText('missing')).toBeNull()
  })
})
