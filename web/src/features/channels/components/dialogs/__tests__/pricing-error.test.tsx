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
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, test, vi } from 'vitest'

import { Button } from '@/components/ui/button'
import { api } from '@/lib/api'

import { channelSchema } from '../../../types'
import { ChannelsProvider, useChannels } from '../../channels-provider'
import { ChannelTestDialog } from '../channel-test-dialog'

const channel = channelSchema.parse({
  id: 42,
  type: 1,
  key: '',
  status: 1,
  name: 'Catalog pricing test',
  models: 'catalog-model',
  created_time: 0,
  test_time: 0,
  response_time: 0,
  balance_updated_time: 0,
})

function ChannelTestHarness() {
  const channels = useChannels()
  return (
    <>
      <Button onClick={() => channels.setCurrentRow(channel)}>Open test</Button>
      <ChannelTestDialog open onOpenChange={() => undefined} />
    </>
  )
}

afterEach(() => localStorage.clear())

test('temporary catalog failure shows retry-later guidance and preserves safe details without a pricing-settings action', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: false,
      message:
        'catalog pricing selection: publication pending (request id: fixture-request)',
      error_code: 'catalog_pricing_unavailable',
      time: 0,
    },
  })
  const user = userEvent.setup()
  render(
    <QueryClientProvider client={new QueryClient()}>
      <ChannelsProvider>
        <ChannelTestHarness />
      </ChannelsProvider>
    </QueryClientProvider>
  )
  await user.click(screen.getByRole('button', { name: 'Open test' }))
  await user.click(screen.getByRole('button', { name: 'Test Connection' }))
  const summary =
    'Pricing service is temporarily unavailable. Please try again later.'
  expect(await screen.findByText(summary)).toBeVisible()
  expect(
    screen.queryByRole('button', { name: 'Go to Settings' })
  ).not.toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: 'Details' }))
  const details = screen.getByRole('dialog', { name: 'Details' })
  expect(within(details).getByText(/fixture-request/)).toBeVisible()
})

test('genuine missing price keeps its configuration guidance and settings action', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: false,
      message: 'No quote for catalog-model',
      error_code: 'model_price_error',
      time: 0,
    },
  })
  const user = userEvent.setup()
  render(
    <QueryClientProvider client={new QueryClient()}>
      <ChannelsProvider>
        <ChannelTestHarness />
      </ChannelsProvider>
    </QueryClientProvider>
  )
  await user.click(screen.getByRole('button', { name: 'Open test' }))
  await user.click(screen.getByRole('button', { name: 'Test Connection' }))
  expect(
    await screen.findByText(
      'Model price is not configured. Please complete model pricing in settings.'
    )
  ).toBeVisible()
  const open = vi.spyOn(window, 'open').mockImplementation(() => null)
  await user.click(screen.getByRole('button', { name: 'Go to Settings' }))
  expect(open).toHaveBeenCalledWith(
    '/system-settings/billing/model-pricing',
    '_blank'
  )
})
