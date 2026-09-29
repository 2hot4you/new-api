import { api } from '@/lib/api'
import { createServerError } from '@/lib/server-error-message'

import type { BillingCalendarMetadata } from './calendar'

export type BillingCalendarModel = {
  model_name: string
  billing_expr: string
}

export type BillingCalendarData = {
  models: BillingCalendarModel[]
  calendar: BillingCalendarMetadata
}

export async function getBillingCalendar(): Promise<BillingCalendarData> {
  const response = await api.get('/api/billing-calendar')
  if (!response.data?.success) {
    throw createServerError(response.data, 'Unable to load billing calendar')
  }
  return response.data.data as BillingCalendarData
}
