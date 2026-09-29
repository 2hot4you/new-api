import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { BillingCalendar } from '..'

const response = {
  success: true,
  data: {
    models: [
      {
        model_name: 'deepseek-calendar',
        billing_expr:
          'is_holiday("CN", "Asia/Shanghai") ? tier("holiday", p * 1) : hour("Asia/Shanghai") >= 9 && hour("Asia/Shanghai") < 18 ? tier("peak", p * 2) : tier("off_peak", p * 1)',
      },
    ],
    calendar: {
      schema: 'billingexpr.calendar',
      version: 1,
      country: 'CN',
      timezone: 'Asia/Shanghai',
      supported_years: [2026],
      coverage_start: '2026-10-01',
      coverage_end: '2026-12-31',
      source: {
        title: '2026 holiday notice',
        document: 'official notice',
        url: 'https://www.gov.cn/example',
      },
      holidays: [{ date: '2026-10-01' }],
      makeup_workdays: [{ date: '2026-10-10' }],
    },
  },
}

function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <BillingCalendar />
    </QueryClientProvider>
  )
}

beforeEach(() => {
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-10-10T12:00:00+08:00'))
})

afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

test('labels calendar classifications in text and keeps makeup status separate from expression tiers', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({ data: response })
  const user = userEvent.setup()
  renderPage()

  expect(
    await screen.findByRole('combobox', { name: 'Billing model' })
  ).toHaveValue('deepseek-calendar')
  await waitFor(() =>
    expect(
      screen.getByRole('region', { name: 'Expression result' })
    ).toHaveAttribute('aria-busy', 'false')
  )
  expect(
    screen.getByRole('status', { name: 'Expression result' })
  ).toHaveTextContent('Expression result')
  const legend = screen.getByRole('list', { name: 'Calendar legend' })
  for (const label of [
    'Workday',
    'Weekend',
    'Statutory holiday',
    'Makeup workday',
    'Unavailable',
  ]) {
    expect(within(legend).getByText(label)).toBeVisible()
  }

  await user.click(
    screen.getByRole('button', { name: '2026-10-10 Makeup workday' })
  )
  const classification = screen.getByRole('region', {
    name: 'Calendar classification',
  })
  expect(within(classification).getByText('Makeup workday')).toBeVisible()
  expect(within(classification).queryByText('peak')).not.toBeInTheDocument()

  const result = screen.getByRole('region', {
    name: 'Expression result',
  })
  expect(within(result).getByText(/Time-of-use pricing:/)).toBeVisible()
  expect(within(result).getByText('Yes')).toBeVisible()
  expect(within(result).getByText('peak')).toBeVisible()
  expect(within(result).getAllByText('off_peak').length).toBeGreaterThan(0)
  expect(within(result).queryByText('Makeup workday')).not.toBeInTheDocument()
  expect(
    screen.getByRole('link', { name: 'Open model pricing' })
  ).toHaveAttribute('href', '/models/metadata')
})

test('shows a safe loading error instead of an empty calendar', async () => {
  vi.spyOn(api, 'get').mockRejectedValue(new Error('network down'))
  renderPage()
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'Unable to load billing calendar'
  )
  expect(screen.getByRole('button', { name: 'Retry' })).toBeVisible()
})

test('marks pre-coverage cells and selected details as unavailable', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({ data: response })
  const user = userEvent.setup()
  renderPage()

  await screen.findByRole('combobox', { name: 'Billing model' })
  await user.click(
    screen.getByRole('button', { name: 'Go to the Previous Month' })
  )

  const unavailable = screen.getAllByRole('button', {
    name: /2026-09-\d{2} Unavailable/,
  })[0]
  await user.click(unavailable)
  expect(
    within(
      screen.getByRole('region', { name: 'Calendar classification' })
    ).getByText('Unavailable')
  ).toBeVisible()
  expect(screen.getByText('Calendar year is not covered')).toBeVisible()
})

test('downloads the visible model month as an Apple Calendar file', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({ data: response })
  const createObjectURL = vi
    .spyOn(URL, 'createObjectURL')
    .mockReturnValue('blob:billing-calendar')
  const revokeObjectURL = vi
    .spyOn(URL, 'revokeObjectURL')
    .mockImplementation(() => undefined)
  const click = vi
    .spyOn(HTMLAnchorElement.prototype, 'click')
    .mockImplementation(() => undefined)
  const user = userEvent.setup()
  renderPage()

  await screen.findByRole('combobox', { name: 'Billing model' })
  await user.click(
    screen.getByRole('button', { name: 'Export Apple Calendar (.ics)' })
  )

  expect(createObjectURL).toHaveBeenCalledWith(expect.any(Blob))
  expect(click).toHaveBeenCalledOnce()
  expect(revokeObjectURL).toHaveBeenCalledWith('blob:billing-calendar')
})
