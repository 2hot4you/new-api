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
import assert from 'node:assert/strict'

import { describe, test, vi } from 'vitest'

import type { QuotationSnapshot } from '../../types'
import { downloadQuotationHtml } from '../download-html'
import {
  buildQuotationHtml,
  escapeHtml,
  type QuotationHtmlLabels,
  type QuotationHtmlOptions,
} from '../quotation-html'

const enLabels = Object.freeze({
  emptyValue: '—',
  customer: 'Customer',
  quotedBy: 'Quoted by',
  quoteDate: 'Quote date',
  globalDiscount: 'Global discount',
  priceBasis: 'Price basis',
  pricingVersion: 'Pricing version',
  fetchedAt: 'Pricing fetched at',
  rawPriceBasis: 'Raw model price (1x)',
  groupPriceBasis: 'User group',
  discount: 'Discount',
  discountCoefficient: 'Discount coefficient',
  discountSuffix: '/10',
  providerNote: 'Provider note',
  priceDimension: 'Price dimension',
  sourceType: 'Source type',
  catalogPrice: 'Catalog price',
  basisPrice: 'Basis price',
  quotePrice: 'Quoted price',
  currency: 'Currency',
  unit: 'Unit',
  condition: 'Condition',
  status: 'Status',
  ready: 'Confirmed',
  needsConfirmation: 'Needs confirmation',
  groupUnavailable: 'Unavailable for selected group',
  catalogMissing: 'Missing from pricing catalog',
  noDimensions: 'No price dimensions to display.',
  noProviderModels: 'No selected models for this provider.',
  noModels: 'No selected models.',
  usageExamples: 'Usage examples',
  sourceTypes: Object.freeze({
    fixed_token: 'Fixed token',
    request: 'Per request',
    dynamic: 'Dynamic',
    task_usage: 'Task usage',
    video: 'Video',
    grok: 'Grok',
  }),
}) satisfies QuotationHtmlLabels

const zhLabels = Object.freeze({
  emptyValue: '—',
  customer: '客户',
  quotedBy: '报价人',
  quoteDate: '报价日期',
  globalDiscount: '全局折扣',
  priceBasis: '价格基准',
  pricingVersion: '价格版本',
  fetchedAt: '价格获取时间',
  rawPriceBasis: '原始模型价（1x）',
  groupPriceBasis: '用户分组',
  discount: '折扣',
  discountCoefficient: '折扣系数',
  discountSuffix: ' 折',
  providerNote: 'Provider 备注',
  priceDimension: '价格维度',
  sourceType: '来源类型',
  catalogPrice: '目录原价',
  basisPrice: '基准价格',
  quotePrice: '报价',
  currency: '币种',
  unit: '单位',
  condition: '条件',
  status: '状态',
  ready: '已确认',
  needsConfirmation: '待确认',
  groupUnavailable: '所选分组不可用',
  catalogMissing: '价格目录中不存在',
  noDimensions: '没有可显示的价格维度',
  noProviderModels: '此 Provider 没有选中的模型。',
  noModels: '没有选中的模型。',
  usageExamples: '用量示例',
  sourceTypes: Object.freeze({
    fixed_token: '固定 Token',
    request: '按次',
    dynamic: '动态',
    task_usage: 'Task 用量',
    video: '视频',
    grok: 'Grok',
  }),
}) satisfies QuotationHtmlLabels

const zhOptions = Object.freeze({
  locale: 'zh-CN',
  labels: zhLabels,
  dimensionLabels: Object.freeze({
    Input: '输入',
    'Dynamic tier': '动态阶梯',
  }),
  unitLabels: Object.freeze({
    '1M token': '每百万 Token',
    'request / second': '请求 / 秒',
  }),
  conditionLabels: Object.freeze({
    'duration > 10': '时长 > 10',
  }),
}) satisfies QuotationHtmlOptions

const enOptions = Object.freeze({
  locale: 'en',
  labels: enLabels,
  dimensionLabels: Object.freeze({
    Input: 'Input',
    'Dynamic tier': 'Dynamic tier',
  }),
  unitLabels: Object.freeze({
    '1M token': '1M token',
    'request / second': 'request / second',
  }),
}) satisfies QuotationHtmlOptions

function quotationSnapshot(
  overrides: Partial<QuotationSnapshot> = {}
): QuotationSnapshot {
  return {
    title: 'Acme & Partners',
    customer: 'Acme, Inc.',
    quotedBy: 'Molii Sales',
    quoteDate: '2026-09-08',
    pricingVersion: 'pricing-v3',
    fetchedAt: '2026-09-08T10:00:00.000Z',
    globalDiscount: 8,
    priceBasis: { type: 'group', group: 'vip', ratio: 1.5 },
    providers: [
      {
        providerId: 10,
        providerName: 'Provider A',
        note: 'Line one\nLine two',
        discount: 5,
        discountCoefficient: 0.5,
        models: [
          {
            modelId: 'model-a',
            displayName: 'Model A',
            available: true,
            unavailableReason: null,
            dimensions: [
              {
                key: 'input',
                label: 'Input',
                sourceType: 'fixed_token',
                catalogAmount: 2,
                sourceAmount: 3,
                quoteAmount: 1.5,
                currency: 'USD',
                unit: '1M token',
                condition: null,
                status: 'ready',
              },
              {
                key: 'dynamic',
                label: 'Dynamic tier',
                sourceType: 'dynamic',
                catalogAmount: null,
                sourceAmount: null,
                quoteAmount: null,
                currency: 'CNY',
                unit: 'request / second',
                condition: 'duration > 10',
                status: 'needs_confirmation',
              },
            ],
          },
        ],
      },
    ],
    ...overrides,
  }
}

function deepFreeze<T>(value: T): T {
  if (value && typeof value === 'object') {
    Object.freeze(value)
    for (const child of Object.values(value)) deepFreeze(child)
  }
  return value
}

describe('standalone quotation HTML', () => {
  test('escapes every HTML-significant character', () => {
    assert.equal(
      escapeHtml(`<tag attr="double" data='single'>& text`),
      '&lt;tag attr=&quot;double&quot; data=&#39;single&#39;&gt;&amp; text'
    )
  })

  test('renders every price dimension separately, including currency, unit, condition, and confirmation state', () => {
    const snapshot = deepFreeze(quotationSnapshot())
    const html = buildQuotationHtml(snapshot, zhOptions)

    assert.match(html, /Acme &amp; Partners/)
    assert.match(html, /Provider A/)
    assert.match(html, /输入/)
    assert.match(html, /\$2/)
    assert.match(html, /\$3/)
    assert.match(html, /\$1\.5/)
    assert.match(html, /USD/)
    assert.match(html, /每百万 Token/)
    assert.match(html, /动态阶梯/)
    assert.match(html, /CNY/)
    assert.match(html, /请求 \/ 秒/)
    assert.match(html, /时长 &gt; 10/)
    assert.match(html, /待确认/)
    assert.match(html, /white-space:\s*pre-wrap/)
    assert.match(html, /Line one\nLine two/)
  })

  test('does not mutate the immutable export snapshot and retains long provider notes', () => {
    const longNote = `Terms:\n${'long note & details\n'.repeat(4_000)}`
    const provider = quotationSnapshot().providers[0]
    assert.ok(provider)
    const snapshot = deepFreeze(
      quotationSnapshot({
        providers: [
          {
            ...provider,
            note: longNote,
          },
        ],
      })
    )

    const html = buildQuotationHtml(snapshot, zhOptions)

    assert.ok(html.includes(escapeHtml(longNote)))
    assert.equal(snapshot.providers[0]?.note, longNote)
  })

  test('escapes all rendered user and catalog text instead of creating executable markup', () => {
    const attack = `<script src="https://evil.example/x.js">alert('x')&</script>`
    const snapshot = quotationSnapshot({
      title: attack,
      customer: attack,
      quotedBy: attack,
      quoteDate: attack,
      pricingVersion: attack,
      fetchedAt: attack,
      priceBasis: { type: 'group', group: attack, ratio: 1.5 },
      providers: [
        {
          providerId: 10,
          providerName: attack,
          note: attack,
          discount: 5,
          discountCoefficient: 0.5,
          models: [
            {
              modelId: attack,
              displayName: attack,
              available: true,
              unavailableReason: null,
              dimensions: [
                {
                  key: attack,
                  label: attack,
                  sourceType: 'dynamic',
                  catalogAmount: 1,
                  sourceAmount: 1.5,
                  quoteAmount: 0.75,
                  currency: 'USD',
                  unit: attack,
                  condition: attack,
                  status: 'ready',
                },
              ],
            },
          ],
        },
      ],
    })

    const html = buildQuotationHtml(snapshot, zhOptions)

    assert.equal(html.includes(attack), false)
    assert.equal(html.includes('<script'), false)
    assert.equal(html.includes('https://evil.example'), true)
    assert.ok(html.includes(escapeHtml(attack)))
  })

  test('is script-free and self-contained with restrictive CSP and A4 print protection', () => {
    const html = buildQuotationHtml(quotationSnapshot(), zhOptions)

    assert.match(html, /^<!doctype html>/i)
    assert.match(html, /<meta charset="utf-8">/i)
    assert.match(
      html,
      /http-equiv="Content-Security-Policy" content="default-src &#39;none&#39;; style-src &#39;unsafe-inline&#39;; img-src data:; font-src data:; base-uri &#39;none&#39;; form-action &#39;none&#39;"/
    )
    assert.match(html, /<meta name="referrer" content="no-referrer">/i)
    assert.match(html, /@page\s*{[^}]*size:\s*A4/)
    assert.match(html, /break-inside:\s*avoid/)
    assert.match(html, /display:\s*table-header-group/)
    assert.match(html, /table-layout:\s*fixed/)
    assert.match(html, /overflow-wrap:\s*anywhere/)
    assert.equal(
      /<(?:script|link|iframe|form|object|embed)\b/i.test(html),
      false
    )
    assert.equal(/\b(?:src|href)\s*=/i.test(html), false)
  })

  test('renders all usage example labels and fact key/value pairs as escaped, printable text', () => {
    const attack = `<img src=x onerror="alert('usage')">&`
    const snapshot = quotationSnapshot()
    const model = snapshot.providers[0]?.models[0]
    assert.ok(model)
    model.usageExamples = [
      {
        label: `Starter ${attack}`,
        facts: {
          [`seconds ${attack}`]: 15,
          [`mode ${attack}`]: `pro ${attack}`,
        },
      },
      {
        label: 'Batch render',
        facts: { clips: 4, credits: 2.5 },
      },
    ]

    const html = buildQuotationHtml(deepFreeze(snapshot), enOptions)

    assert.match(html, /<h4>Starter &lt;img/)
    assert.match(html, /seconds &lt;img/)
    assert.match(html, />15<\/dd>/)
    assert.match(html, /mode &lt;img/)
    assert.match(html, /pro &lt;img/)
    assert.match(html, /<h4>Batch render<\/h4>/)
    assert.match(html, />clips<\/dt>/)
    assert.match(html, />4<\/dd>/)
    assert.match(html, />credits<\/dt>/)
    assert.match(html, />2\.5<\/dd>/)
    assert.equal(html.includes(attack), false)
    assert.equal(html.includes('<img'), false)
    assert.match(html, /\.usage-example[^}]*max-width:\s*100%/)
    assert.match(html, /\.usage-example[^}]*break-inside:\s*avoid/)
    assert.match(html, /\.usage-facts[^}]*overflow-wrap:\s*anywhere/)
  })

  test('uses explicit Chinese and English labels and sets the requested document language', () => {
    const zhHtml = buildQuotationHtml(
      deepFreeze(quotationSnapshot()),
      deepFreeze(zhOptions)
    )
    const enHtml = buildQuotationHtml(
      deepFreeze(quotationSnapshot()),
      deepFreeze(enOptions)
    )

    assert.match(zhHtml, /<html lang="zh-CN">/)
    assert.match(zhHtml, /<th>价格维度<\/th>/)
    assert.match(zhHtml, /<dt>客户<\/dt>/)
    assert.match(enHtml, /<html lang="en">/)
    assert.match(enHtml, /<th>Price dimension<\/th>/)
    assert.match(enHtml, /<dt>Customer<\/dt>/)
    assert.match(enHtml, /Needs confirmation/)
    assert.equal(enHtml.includes('价格维度'), false)
    assert.equal(enHtml.includes('待确认'), false)
  })

  test('localizes known fixed-token, task-usage, video, and Grok row values with raw fallback', () => {
    const snapshot = quotationSnapshot()
    const provider = snapshot.providers[0]
    assert.ok(provider)
    const template = provider.models[0]?.dimensions[0]
    assert.ok(template)
    provider.models = [
      {
        modelId: 'fixed',
        displayName: 'Fixed',
        available: true,
        unavailableReason: null,
        dimensions: [
          {
            ...template,
            key: 'input',
            label: 'Input',
            sourceType: 'fixed_token',
            unit: '1M token',
            condition: null,
          },
        ],
      },
      {
        modelId: 'task',
        displayName: 'Task',
        available: true,
        unavailableReason: null,
        dimensions: [
          {
            ...template,
            key: 'task-tier-0-base',
            label: 'Base charge',
            sourceType: 'task_usage',
            unit: 'request',
            condition: 'pro; mode = pro',
          },
          {
            ...template,
            key: 'task-tier-0-clips',
            label: 'catalog-defined-field',
            sourceType: 'task_usage',
            unit: 'count',
            condition: 'catalog-defined-condition',
          },
        ],
      },
      {
        modelId: 'video',
        displayName: 'Video',
        available: true,
        unavailableReason: null,
        dimensions: [
          {
            ...template,
            key: 'video-0-without-input',
            label: '720p without video input',
            sourceType: 'video',
            unit: '1M token',
            condition:
              '720p; fps 24; extra frames 1; Token = ceil(width x height x (fps x duration + extra frames) / 1024)',
          },
        ],
      },
      {
        modelId: 'grok',
        displayName: 'Grok',
        available: true,
        unavailableReason: null,
        dimensions: [
          {
            ...template,
            key: 'grok-output-high/720p',
            label: 'high/720p output',
            sourceType: 'grok',
            unit: 'second',
            condition: 'high/720p',
          },
        ],
      },
    ]

    const html = buildQuotationHtml(deepFreeze(snapshot), {
      ...zhOptions,
      dimensionLabels: Object.freeze({
        Input: '输入',
        'Base charge': '基础费用',
        '720p without video input': '720p 不含视频输入',
        'high/720p output': '高质量 720p 输出',
      }),
      unitLabels: Object.freeze({
        '1M token': '每百万 Token',
        request: '每次请求',
        count: '次',
        second: '秒',
      }),
      conditionLabels: Object.freeze({
        'pro; mode = pro': '专业版；模式 = 专业版',
        '720p; fps 24; extra frames 1; Token = ceil(width x height x (fps x duration + extra frames) / 1024)':
          '720p；每秒 24 帧；额外帧 1；Token 公式',
        'high/720p': '高质量 / 720p',
      }),
    })

    assert.match(html, />输入<\/td>/)
    assert.match(html, />基础费用<\/td>/)
    assert.match(html, />720p 不含视频输入<\/td>/)
    assert.match(html, />高质量 720p 输出<\/td>/)
    assert.match(html, />每百万 Token<\/td>/)
    assert.match(html, />每次请求<\/td>/)
    assert.match(html, />次<\/td>/)
    assert.match(html, />秒<\/td>/)
    assert.match(html, />专业版；模式 = 专业版<\/td>/)
    assert.match(html, />720p；每秒 24 帧；额外帧 1；Token 公式<\/td>/)
    assert.match(html, />高质量 \/ 720p<\/td>/)
    assert.match(html, />catalog-defined-field<\/td>/)
    assert.match(html, />catalog-defined-condition<\/td>/)
  })

  test('escapes hostile mapped dimension, unit, and condition labels', () => {
    const attack = `<img src=x onerror="alert('row-map')">&`
    const html = buildQuotationHtml(quotationSnapshot(), {
      ...enOptions,
      dimensionLabels: { Input: attack },
      unitLabels: { '1M token': attack },
      conditionLabels: { 'duration > 10': attack },
    })

    assert.equal(html.includes(attack), false)
    assert.equal(html.includes('<img'), false)
    assert.ok(html.split(escapeHtml(attack)).length >= 4)
  })

  test('escapes hostile locale and translated label text', () => {
    const attack = `"><img src=x onerror="alert('translation')">&`
    const html = buildQuotationHtml(quotationSnapshot(), {
      locale: attack,
      labels: { ...enLabels, customer: attack },
      dimensionLabels: {},
      unitLabels: {},
    })
    const parsed = new DOMParser().parseFromString(html, 'text/html')

    assert.equal(parsed.documentElement.lang, attack)
    assert.equal(parsed.querySelector('[onerror]'), null)
    assert.equal(html.includes(`<dt>${attack}</dt>`), false)
    assert.ok(html.includes(`<dt>${escapeHtml(attack)}</dt>`))
  })
})

describe('quotation HTML download', () => {
  test('downloads localized UTF-8 HTML with a sanitized filename and revokes the URL after clicking', async () => {
    const events: string[] = []
    let downloadedFilename = ''
    let downloadedBlob: Blob | null = null
    const createObjectURL = vi
      .spyOn(URL, 'createObjectURL')
      .mockImplementation((blob) => {
        if (!(blob instanceof Blob)) assert.fail('expected an HTML Blob')
        assert.equal(blob.type, 'text/html;charset=utf-8')
        downloadedBlob = blob
        events.push('create')
        return 'blob:quotation'
      })
    const revokeObjectURL = vi
      .spyOn(URL, 'revokeObjectURL')
      .mockImplementation((url) => {
        assert.equal(url, 'blob:quotation')
        events.push('revoke')
      })
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(
      function (this: HTMLAnchorElement) {
        downloadedFilename = this.download
        assert.equal(this.href, 'blob:quotation')
        events.push('click')
      }
    )

    downloadQuotationHtml(
      quotationSnapshot({ title: ' Acme / Q3:*? ', quoteDate: '2026-09-08' }),
      enOptions
    )

    const blobToRead = downloadedBlob
    assert.ok(blobToRead)
    const exportedHtml = await new Promise<string>((resolve, reject) => {
      const reader = new FileReader()
      reader.addEventListener('load', () => resolve(String(reader.result)))
      reader.addEventListener('error', () => reject(reader.error))
      reader.readAsText(blobToRead)
    })
    assert.equal(downloadedFilename, 'Acme  Q3-2026-09-08.html')
    assert.match(exportedHtml, /<html lang="en">/)
    assert.match(exportedHtml, /<dt>Customer<\/dt>/)
    assert.deepEqual(events, ['create', 'click', 'revoke'])
    assert.equal(createObjectURL.mock.calls.length, 1)
    assert.equal(revokeObjectURL.mock.calls.length, 1)
    assert.equal(document.querySelector('a[download]'), null)
  })
})
