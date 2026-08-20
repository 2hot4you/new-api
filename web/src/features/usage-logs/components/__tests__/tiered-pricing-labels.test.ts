import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type { UsageLog } from '../../data/schema'
import type { LogOtherData } from '../../types'
import { getCommonLogDetailPreviewText } from '../columns/common-logs-columns'

const t = (key: string, opts?: Record<string, unknown>) => {
  if (key === 'Single-request input {{range}} Tokens') {
    return `单次输入 ${String(opts?.range)} Token`
  }
  if (key === 'Base-period price') return '基础时段价格'
  if (key === 'Default price') return '默认价格'
  return key
}

function tieredOther(expression: string, matchedTier: string): LogOtherData {
  return {
    billing_mode: 'tiered_expr',
    expr_b64: Buffer.from(expression, 'utf8').toString('base64'),
    matched_tier: matchedTier,
  }
}

const consumeLog = { type: 2, quota: 1 } as UsageLog

describe('usage log tiered pricing labels', () => {
  test('summarizes the matched input range without exposing long_context', () => {
    const text = getCommonLogDetailPreviewText(
      consumeLog,
      tieredOther(
        'len <= 128000 ? tier("short_context", p * 1 + c * 2) : tier("long_context", p * 2 + c * 4)',
        'long_context'
      ),
      t,
      false
    )

    assert.match(text ?? '', /单次输入 > 128K Token/)
    assert.doesNotMatch(text ?? '', /long_context/)
  })

  test('summarizes a time-based base tier as the base-period price', () => {
    const text = getCommonLogDetailPreviewText(
      consumeLog,
      tieredOther(
        '(tier("base", p * 1.5 + c * 4.5)) * (hour("Asia/Shanghai") >= 9 && hour("Asia/Shanghai") < 12 ? 2 : 1)',
        'base'
      ),
      t,
      false
    )

    assert.match(text ?? '', /基础时段价格/)
    assert.doesNotMatch(text ?? '', /base/)
  })
})
