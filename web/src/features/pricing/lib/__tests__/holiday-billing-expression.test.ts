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
import { createInstance } from 'i18next'
import { assert, describe, expect, test } from 'vitest'

import { billingExpressionUsesTime } from '../../hooks/use-billing-time'
import { buildRequestRuleExpr, tryParseRequestRuleExpr } from '../billing-expr'
import { formatBillingCondition } from '../billing-expression/condition-display'
import {
  isTimeCondition,
  mergeDailyTimePricing,
  readTimeTokenPricing,
} from '../billing-expression/display'
import { compileBillingExpression } from '../billing-expression/parser'
import { evaluateBillingExpression } from '../billing-expression/runtime'
import { TIME_FUNCTIONS } from '../billing-expression/types'
import {
  parseVisualBillingDocument,
  serializeVisualBillingDocument,
} from '../billing-expression/visual'

const holidayExpression =
  'is_holiday("CN", "Asia/Shanghai") ? tier("holiday", p * 1) : tier("regular", p * 2)'

function evaluateTier(expression: string, now: string): string {
  const result = evaluateBillingExpression(expression, {
    now: new Date(now),
    tokens: { p: 100 },
  })
  assert(result.status === 'success')
  return result.matchedTier
}

describe('holiday billing expression', () => {
  test('keeps is_holiday separate from single-string numeric time functions', () => {
    expect(TIME_FUNCTIONS).not.toContain('is_holiday')
    const compiled = compileBillingExpression(holidayExpression)
    assert(compiled.status === 'ready')
    expect([...compiled.functions]).toEqual(['is_holiday', 'tier'])

    for (const source of [
      'is_holiday("CN") ? 1 : 2',
      'is_holiday("CN", "UTC", "extra") ? 1 : 2',
      'is_holiday(1, "UTC") ? 1 : 2',
      'is_holiday("CN", 1) ? 1 : 2',
    ]) {
      expect(compileBillingExpression(source).status).toBe('invalid')
    }
  })

  test.each([
    [
      'trims and uppercases CN on an explicit rest day',
      ' cn ',
      '2026-10-01T01:00:00+08:00',
      'holiday',
    ],
    [
      'does not infer an ordinary weekend',
      'CN',
      '2026-10-11T10:00:00+08:00',
      'regular',
    ],
    [
      'keeps a makeup workday non-holiday',
      'CN',
      '2026-10-10T10:00:00+08:00',
      'regular',
    ],
    [
      'fails closed for an unknown country',
      'US',
      '2026-10-01T10:00:00+08:00',
      'regular',
    ],
    [
      'fails closed for an uncovered year',
      'CN',
      '2027-01-01T10:00:00+08:00',
      'regular',
    ],
  ])('%s', (_name, country, now, tier) => {
    expect(
      evaluateTier(
        holidayExpression.replace('"CN"', JSON.stringify(country)),
        now
      )
    ).toBe(tier)
  })

  test.each(['', 'Invalid/Zone', 'Local'])(
    'falls back to UTC for timezone %j',
    (timezone) => {
      expect(
        evaluateTier(
          holidayExpression.replace(
            '"Asia/Shanghai"',
            JSON.stringify(timezone)
          ),
          '2025-12-31T16:30:00Z'
        )
      ).toBe('regular')
    }
  )

  test('traces holiday-only request rules and marks a matching rule', () => {
    const source =
      '(is_holiday("CN", "Asia/Shanghai") ? 0.5 : 1) * tier("base", p * 2)'
    const compiled = compileBillingExpression(source)
    assert(compiled.status === 'ready')
    expect(compiled.requestRules).toHaveLength(1)
    expect(compiled.requestRules[0].cond).toBe(
      'is_holiday("CN", "Asia/Shanghai")'
    )
    expect(
      evaluateBillingExpression(compiled, {
        now: new Date('2026-10-01T10:00:00+08:00'),
        tokens: { p: 100 },
      })
    ).toMatchObject({
      status: 'success',
      cost: 100,
      requestRules: [{ multiplier: 0.5, matched: true }],
    })
  })

  test('parses a compact holiday request rule for visual editing', () => {
    const groups = tryParseRequestRuleExpr(
      '(is_holiday("CN","Asia/Shanghai") ? 0.5 : 1)'
    )
    expect(groups?.[0].conditions).toEqual([
      {
        source: 'holiday',
        country: 'CN',
        timezone: 'Asia/Shanghai',
        path: 'CN',
        mode: 'eq',
        value: 'true',
      },
    ])
    expect(buildRequestRuleExpr(groups ?? [])).toBe(
      '(is_holiday("CN", "Asia/Shanghai") ? 0.5 : 1)'
    )
  })

  test('decodes parser-supported hex escapes in holiday request rules', () => {
    const groups = tryParseRequestRuleExpr(
      '(is_holiday("\\x43\\x4e", "\\x41sia/Shanghai") ? 0.5 : 1)'
    )

    expect(groups?.[0].conditions).toEqual([
      {
        source: 'holiday',
        country: 'CN',
        timezone: 'Asia/Shanghai',
        path: 'CN',
        mode: 'eq',
        value: 'true',
      },
    ])
  })

  test('treats a holiday-only pricing branch as time dependent', () => {
    const compiled = compileBillingExpression(holidayExpression)
    assert(compiled.status === 'ready')
    assert(compiled.ast.kind === 'conditional')
    expect(isTimeCondition(compiled.ast.condition)).toBe(true)
    expect(billingExpressionUsesTime(holidayExpression)).toBe(true)
    expect(
      readTimeTokenPricing(
        holidayExpression,
        new Date('2026-10-01T10:00:00+08:00')
      )?.currentTiers.map((tier) => tier.label)
    ).toEqual(['holiday'])
  })

  test('formats a friendly holiday condition', async () => {
    const translations = createInstance()
    await translations.init({
      lng: 'en',
      resources: { en: { translation: {} } },
    })
    expect(
      formatBillingCondition(
        'is_holiday("CN", "Asia/Shanghai")',
        translations.t,
        'en'
      )
    ).toBe('CN statutory holiday (Asia/Shanghai)')
    expect(
      formatBillingCondition(
        '!is_holiday("CN", "Asia/Shanghai")',
        translations.t,
        'en'
      )
    ).toBe('Not CN statutory holiday (Asia/Shanghai)')
  })

  test('round-trips an unedited holiday condition without changing its source', () => {
    const source =
      'v1: is_holiday(\' CN \', "Asia/Shanghai") ? tier("holiday", p * 1.00) : tier("regular", p * 2)'
    const document = parseVisualBillingDocument(source)
    assert(document)
    assert(document.root.kind === 'branch')
    expect(document.root.condition).toMatchObject({
      kind: 'holiday',
      country: ' CN ',
      timezone: 'Asia/Shanghai',
    })
    expect(serializeVisualBillingDocument(document)).toEqual({
      ok: true,
      source,
    })
  })

  test('merges adjacent per-minute results and keeps tier price structures', () => {
    const source =
      'is_holiday("CN", "Asia/Shanghai") ? tier("holiday", p * 0.5) : hour("Asia/Shanghai") >= 9 && hour("Asia/Shanghai") < 18 ? tier("day", p * 2 + c * 4) : tier("night", p * 1 + c * 2)'
    expect(
      mergeDailyTimePricing(source, '2026-01-05', 'Asia/Shanghai')
    ).toEqual([
      {
        startMinute: 0,
        endMinute: 540,
        tiers: [
          {
            label: 'night',
            conditions: [],
            prices: { p: 1, c: 2 },
          },
        ],
      },
      {
        startMinute: 540,
        endMinute: 1080,
        tiers: [
          {
            label: 'day',
            conditions: [],
            prices: { p: 2, c: 4 },
          },
        ],
      },
      {
        startMinute: 1080,
        endMinute: 1440,
        tiers: [
          {
            label: 'night',
            conditions: [],
            prices: { p: 1, c: 2 },
          },
        ],
      },
    ])
    expect(
      mergeDailyTimePricing(source, '2026-10-01', 'Asia/Shanghai')
    ).toEqual([
      {
        startMinute: 0,
        endMinute: 1440,
        tiers: [
          {
            label: 'holiday',
            conditions: [],
            prices: { p: 0.5 },
          },
        ],
      },
    ])
  })

  test.each([
    ['tier("base", max(p, 1))', '2026-01-01', 'Asia/Shanghai'],
    [holidayExpression, '2026-02-30', 'Asia/Shanghai'],
    [holidayExpression, '2026-01-01', 'Invalid/Zone'],
  ])(
    'returns null when a day cannot be visualized safely',
    (source, day, zone) => {
      expect(mergeDailyTimePricing(source, day, zone)).toBeNull()
    }
  )
})
