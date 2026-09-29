import { describe, expect, test } from 'vitest'

import { combineBillingExpr } from '@/features/pricing/lib/billing-expr'

import {
  billingCalendarDay,
  buildBillingCalendarIcs,
  buildBillingDaySegments,
  type BillingCalendarMetadata,
} from '../calendar'

const calendar = {
  schema: 'billingexpr.calendar',
  version: 1,
  country: 'CN',
  timezone: 'Asia/Shanghai',
  coverage_start: '2026-10-01',
  coverage_end: '2026-12-31',
  supported_years: [2026],
  source: {
    title: '2026 holiday notice',
    url: 'https://www.gov.cn/example',
  },
  holidays: [{ date: '2026-10-01', name: 'National Day' }],
  makeup_workdays: [{ date: '2026-10-10', name: 'National Day makeup' }],
} as BillingCalendarMetadata & {
  coverage_start: string
  coverage_end: string
}

describe('billing calendar classifications', () => {
  test.each([
    ['2026-10-01', 'holiday', 'National Day'],
    ['2026-10-10', 'makeup_workday', 'National Day makeup'],
    ['2026-10-11', 'weekend', undefined],
    ['2026-10-12', 'workday', undefined],
  ] as const)('classifies %s as %s', (date, classification, name) => {
    expect(billingCalendarDay(date, calendar)).toEqual({
      date,
      classification,
      covered: true,
      name,
    })
  })

  test('marks uncovered years without guessing a calendar override', () => {
    expect(billingCalendarDay('2027-01-01', calendar)).toEqual({
      date: '2027-01-01',
      classification: 'unknown',
      covered: false,
      name: undefined,
    })
  })

  test('does not classify dates before the official coverage start', () => {
    expect(billingCalendarDay('2026-09-30', calendar)).toEqual({
      date: '2026-09-30',
      classification: 'unknown',
      covered: false,
      name: undefined,
    })
  })

  test.each(['2026-02-30', '2026-13-01', 'not-a-date'])(
    'rejects nonexistent ISO date %s',
    (date) => {
      expect(billingCalendarDay(date, calendar)).toEqual({
        date,
        classification: 'unknown',
        covered: false,
        name: undefined,
      })
      expect(buildBillingDaySegments(expression, date)).toEqual({
        status: 'unsupported',
      })
    }
  )
})

describe('Apple Calendar export', () => {
  test('exports covered visualized periods with tier, prices and classification', () => {
    const ics = buildBillingCalendarIcs({
      modelName: 'deepseek/calendar',
      expression,
      month: '2026-10',
      calendar,
    })

    expect(ics).toContain('BEGIN:VCALENDAR\r\n')
    expect(ics).toContain('DTSTART;TZID=Asia/Shanghai:20261001T000000')
    expect(ics).toContain('SUMMARY:deepseek/calendar · off_peak')
    expect(ics).toContain('Classification: statutory holiday')
    expect(ics).toContain('Input: $1 / 1M token')
    expect(ics).not.toContain('20260930')
    expect(ics.match(/BEGIN:VEVENT/g)?.length).toBeGreaterThan(31)
  })

  test('does not export uncovered months', () => {
    expect(
      buildBillingCalendarIcs({
        modelName: 'deepseek-calendar',
        expression,
        month: '2026-09',
        calendar,
      })
    ).not.toContain('BEGIN:VEVENT')
  })
})

const expression = `
is_holiday("CN", "Asia/Shanghai")
  ? tier("off_peak", p * 1 + c * 2)
  : weekday("Asia/Shanghai") >= 1 && weekday("Asia/Shanghai") <= 5 &&
    ((hour("Asia/Shanghai") >= 9 && hour("Asia/Shanghai") < 12) ||
     (hour("Asia/Shanghai") >= 14 && hour("Asia/Shanghai") < 18))
    ? tier("peak", p * 2 + c * 4)
    : tier("off_peak", p * 1 + c * 2)
`

describe('minute billing segments', () => {
  test('projects the exact combined DeepSeek tiers and holiday multiplier', () => {
    const peakCondition =
      '!is_holiday("CN", "Asia/Shanghai") && weekday("Asia/Shanghai") >= 1 && weekday("Asia/Shanghai") <= 5 && ((hour("Asia/Shanghai") >= 9 && hour("Asia/Shanghai") < 12) || (hour("Asia/Shanghai") >= 14 && hour("Asia/Shanghai") < 18))'
    const combined = combineBillingExpr(
      `${peakCondition} ? tier("peak", p * 3 + cr * 0.10 + c * 9) : tier("off_peak", p * 1.5 + cr * 0.05 + c * 4.5)`,
      '(is_holiday("CN", "Asia/Shanghai") ? 0.5 : 1)'
    )

    expect(buildBillingDaySegments(combined, '2026-09-21')).toEqual({
      status: 'ready',
      segments: [
        {
          start: '00:00',
          end: '09:00',
          tier: 'off_peak',
          prices: { p: 1.5, cr: 0.05, c: 4.5 },
        },
        {
          start: '09:00',
          end: '12:00',
          tier: 'peak',
          prices: { p: 3, cr: 0.1, c: 9 },
        },
        {
          start: '12:00',
          end: '14:00',
          tier: 'off_peak',
          prices: { p: 1.5, cr: 0.05, c: 4.5 },
        },
        {
          start: '14:00',
          end: '18:00',
          tier: 'peak',
          prices: { p: 3, cr: 0.1, c: 9 },
        },
        {
          start: '18:00',
          end: '24:00',
          tier: 'off_peak',
          prices: { p: 1.5, cr: 0.05, c: 4.5 },
        },
      ],
    })
    expect(buildBillingDaySegments(combined, '2026-10-01')).toEqual({
      status: 'ready',
      segments: [
        {
          start: '00:00',
          end: '24:00',
          tier: 'off_peak',
          prices: { p: 0.75, cr: 0.025, c: 2.25 },
        },
      ],
    })
    expect(
      buildBillingDaySegments(
        combineBillingExpr(
          'tier("request", fixed(0.02))',
          '(is_holiday("CN", "Asia/Shanghai") ? 0.5 : 1)'
        ),
        '2026-10-01'
      )
    ).toEqual({
      status: 'ready',
      segments: [
        {
          start: '00:00',
          end: '24:00',
          tier: 'request',
          prices: {},
          fixed_price: 0.01,
        },
      ],
    })
  })

  test('builds a full day without blocking for seconds', () => {
    const startedAt = performance.now()
    expect(buildBillingDaySegments(expression, '2026-09-21').status).toBe(
      'ready'
    )
    expect(performance.now() - startedAt).toBeLessThan(1_000)
  })

  test('merges adjacent results while preserving the split workday peaks', () => {
    const result = buildBillingDaySegments(expression, '2026-09-21')
    expect(result.status).toBe('ready')
    if (result.status !== 'ready') return
    expect(
      result.segments.map(({ start, end, tier }) => [start, end, tier])
    ).toEqual([
      ['00:00', '09:00', 'off_peak'],
      ['09:00', '12:00', 'peak'],
      ['12:00', '14:00', 'off_peak'],
      ['14:00', '18:00', 'peak'],
      ['18:00', '24:00', 'off_peak'],
    ])
    expect(result.segments[1].prices).toEqual({ p: 2, c: 4 })
  })

  test('uses the expression result, not the weekday label, on statutory holidays', () => {
    const result = buildBillingDaySegments(expression, '2026-10-01')
    expect(result.status).toBe('ready')
    if (result.status !== 'ready') return
    expect(result.segments).toEqual([
      {
        start: '00:00',
        end: '24:00',
        tier: 'off_peak',
        prices: { p: 1, c: 2 },
      },
    ])
  })

  test('rejects expressions that cannot be represented without guessing', () => {
    expect(
      buildBillingDaySegments(
        'param("service_tier") == "fast" ? tier("fast", p * 2) : tier("base", p)',
        '2026-09-21'
      )
    ).toEqual({ status: 'unsupported' })
  })
})
