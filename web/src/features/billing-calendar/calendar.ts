import {
  mergeDailyTimePricing,
  readTokenTierChain,
  type TokenTier,
} from '@/features/pricing/lib/billing-expression/display'
import { compileBillingExpression } from '@/features/pricing/lib/billing-expression/parser'
import type { TokenVariable } from '@/features/pricing/lib/billing-expression/types'

export type BillingCalendarClassification =
  | 'workday'
  | 'weekend'
  | 'holiday'
  | 'makeup_workday'
  | 'unknown'

export type BillingCalendarDate = {
  date: string
  name?: string
}

export type BillingCalendarMetadata = {
  schema: string
  version: number
  country: string
  timezone: string
  coverage_start: string
  coverage_end: string
  supported_years: number[]
  source: {
    title: string
    url: string
    published_at?: string
  }
  holidays: BillingCalendarDate[]
  makeup_workdays: BillingCalendarDate[]
}

export type BillingCalendarDay = {
  date: string
  classification: BillingCalendarClassification
  covered: boolean
  name?: string
}

export type BillingDaySegment = {
  start: string
  end: string
  tier: string
  prices: Partial<Record<TokenVariable, number>>
  fixed_price?: number
}

export type BillingDaySegments =
  | { status: 'ready'; segments: BillingDaySegment[] }
  | { status: 'unsupported' }

function parseCalendarDate(date: string): Date | null {
  const match = date.match(/^(\d{4})-(\d{2})-(\d{2})$/)
  if (!match) return null
  const parsed = new Date(`${date}T12:00:00Z`)
  if (!Number.isFinite(parsed.getTime())) return null
  return parsed.getUTCFullYear() === Number(match[1]) &&
    parsed.getUTCMonth() + 1 === Number(match[2]) &&
    parsed.getUTCDate() === Number(match[3])
    ? parsed
    : null
}

export function billingCalendarDay(
  date: string,
  calendar: BillingCalendarMetadata
): BillingCalendarDay {
  const parsed = parseCalendarDate(date)
  const year = parsed?.getUTCFullYear()
  const covered =
    year !== undefined &&
    calendar.supported_years.includes(year) &&
    date >= calendar.coverage_start &&
    date <= calendar.coverage_end
  if (!parsed || !covered) {
    return {
      date,
      classification: 'unknown',
      covered: false,
      name: undefined,
    }
  }
  const holiday = calendar.holidays.find((item) => item.date === date)
  if (holiday) {
    return {
      date,
      classification: 'holiday',
      covered,
      name: holiday.name,
    }
  }
  const makeup = calendar.makeup_workdays.find((item) => item.date === date)
  if (makeup) {
    return {
      date,
      classification: 'makeup_workday',
      covered,
      name: makeup.name,
    }
  }
  const weekday = parsed?.getUTCDay() ?? 1
  return {
    date,
    classification: weekday === 0 || weekday === 6 ? 'weekend' : 'workday',
    covered,
    name: undefined,
  }
}

function minuteLabel(minute: number): string {
  if (minute === 24 * 60) return '24:00'
  return `${String(Math.floor(minute / 60)).padStart(2, '0')}:${String(
    minute % 60
  ).padStart(2, '0')}`
}

function segmentForTier(
  tier: TokenTier,
  start: number,
  end: number
): BillingDaySegment {
  return {
    start: minuteLabel(start),
    end: minuteLabel(end),
    tier: tier.label,
    prices: tier.prices,
    ...(tier.fixedPrice === undefined ? {} : { fixed_price: tier.fixedPrice }),
  }
}

export function buildBillingDaySegments(
  expression: string,
  date: string,
  timezone = 'Asia/Shanghai'
): BillingDaySegments {
  if (!parseCalendarDate(date)) return { status: 'unsupported' }
  const compiled = compileBillingExpression(expression)
  if (compiled.status !== 'ready') return { status: 'unsupported' }

  const periods = mergeDailyTimePricing(expression, date, timezone)
  if (!periods) {
    const tiers = readTokenTierChain(compiled.ast)
    if (!tiers || tiers.length !== 1) return { status: 'unsupported' }
    return {
      status: 'ready',
      segments: [segmentForTier(tiers[0], 0, 24 * 60)],
    }
  }

  if (periods.some((period) => period.tiers.length !== 1)) {
    return { status: 'unsupported' }
  }
  return {
    status: 'ready',
    segments: periods.map((period) =>
      segmentForTier(period.tiers[0], period.startMinute, period.endMinute)
    ),
  }
}

const icsClassification: Record<BillingCalendarClassification, string> = {
  workday: 'workday',
  weekend: 'weekend',
  holiday: 'statutory holiday',
  makeup_workday: 'makeup workday',
  unknown: 'unavailable',
}

const icsPriceLabels: Record<string, string> = {
  p: 'Input',
  c: 'Output',
  cr: 'Cache read',
  cc: 'Cache write',
  cc1h: 'Cache create (1h)',
  img: 'Image input',
  img_cr: 'Image cache input',
  img_o: 'Image output',
  ai: 'Audio input',
  ao: 'Audio output',
}

function escapeIcsText(value: string): string {
  return value
    .replaceAll('\\', '\\\\')
    .replaceAll('\n', '\\n')
    .replaceAll(',', '\\,')
    .replaceAll(';', '\\;')
}

function compactIcsDateTime(date: string, time: string): string {
  let eventDate = date
  let eventTime = time
  if (time === '24:00') {
    const next = new Date(`${date}T12:00:00Z`)
    next.setUTCDate(next.getUTCDate() + 1)
    eventDate = next.toISOString().slice(0, 10)
    eventTime = '00:00'
  }
  return `${eventDate.replaceAll('-', '')}T${eventTime.replace(':', '')}00`
}

function icsPriceDescription(segment: BillingDaySegment): string {
  if (segment.fixed_price !== undefined) {
    return `Fixed price: $${segment.fixed_price} / request`
  }
  return Object.entries(segment.prices)
    .filter((entry): entry is [string, number] => entry[1] !== undefined)
    .map(
      ([name, price]) => `${icsPriceLabels[name] ?? name}: $${price} / 1M token`
    )
    .join(' · ')
}

export function buildBillingCalendarIcs(input: {
  modelName: string
  expression: string
  month: string
  calendar: BillingCalendarMetadata
}): string {
  const match = input.month.match(/^(\d{4})-(\d{2})$/)
  const lines = [
    'BEGIN:VCALENDAR',
    'VERSION:2.0',
    'PRODID:-//Molii//Billing Calendar//EN',
    'CALSCALE:GREGORIAN',
    `X-WR-TIMEZONE:${input.calendar.timezone}`,
  ]
  if (!match) return [...lines, 'END:VCALENDAR', ''].join('\r\n')
  const year = Number(match[1])
  const month = Number(match[2])
  if (month < 1 || month > 12) {
    return [...lines, 'END:VCALENDAR', ''].join('\r\n')
  }
  const lastDay = new Date(Date.UTC(year, month, 0)).getUTCDate()
  const uidModel = input.modelName.replaceAll(/[^A-Za-z0-9_-]+/g, '-')
  for (let dayNumber = 1; dayNumber <= lastDay; dayNumber++) {
    const date = `${match[1]}-${match[2]}-${String(dayNumber).padStart(2, '0')}`
    const classification = billingCalendarDay(date, input.calendar)
    if (!classification.covered) continue
    const result = buildBillingDaySegments(
      input.expression,
      date,
      input.calendar.timezone
    )
    if (result.status !== 'ready') continue
    for (const segment of result.segments) {
      const description = [
        `Classification: ${icsClassification[classification.classification]}`,
        classification.name ? `Name: ${classification.name}` : '',
        icsPriceDescription(segment),
      ]
        .filter(Boolean)
        .join('\n')
      lines.push(
        'BEGIN:VEVENT',
        `UID:${uidModel}-${date}-${segment.start.replace(':', '')}@billing.molii`,
        'DTSTAMP:19700101T000000Z',
        `DTSTART;TZID=${input.calendar.timezone}:${compactIcsDateTime(date, segment.start)}`,
        `DTEND;TZID=${input.calendar.timezone}:${compactIcsDateTime(date, segment.end)}`,
        `SUMMARY:${escapeIcsText(`${input.modelName} · ${segment.tier}`)}`,
        `DESCRIPTION:${escapeIcsText(description)}`,
        'END:VEVENT'
      )
    }
  }
  return [...lines, 'END:VCALENDAR', ''].join('\r\n')
}
