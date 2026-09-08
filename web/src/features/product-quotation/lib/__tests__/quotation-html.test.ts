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
import { buildQuotationHtml, escapeHtml } from '../quotation-html'

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
    const html = buildQuotationHtml(snapshot)

    assert.match(html, /Acme &amp; Partners/)
    assert.match(html, /Provider A/)
    assert.match(html, /Input/)
    assert.match(html, /\$2/)
    assert.match(html, /\$3/)
    assert.match(html, /\$1\.5/)
    assert.match(html, /USD/)
    assert.match(html, /1M token/)
    assert.match(html, /Dynamic tier/)
    assert.match(html, /CNY/)
    assert.match(html, /request \/ second/)
    assert.match(html, /duration &gt; 10/)
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

    const html = buildQuotationHtml(snapshot)

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

    const html = buildQuotationHtml(snapshot)

    assert.equal(html.includes(attack), false)
    assert.equal(html.includes('<script'), false)
    assert.equal(html.includes('https://evil.example'), true)
    assert.ok(html.includes(escapeHtml(attack)))
  })

  test('is script-free and self-contained with restrictive CSP and A4 print protection', () => {
    const html = buildQuotationHtml(quotationSnapshot())

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
})

describe('quotation HTML download', () => {
  test('downloads a UTF-8 HTML Blob with a sanitized filename and revokes the URL after clicking', () => {
    const events: string[] = []
    let downloadedFilename = ''
    const createObjectURL = vi
      .spyOn(URL, 'createObjectURL')
      .mockImplementation((blob) => {
        if (!(blob instanceof Blob)) assert.fail('expected an HTML Blob')
        assert.equal(blob.type, 'text/html;charset=utf-8')
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
      quotationSnapshot({ title: ' Acme / Q3:*? ', quoteDate: '2026-09-08' })
    )

    assert.equal(downloadedFilename, 'Acme  Q3-2026-09-08.html')
    assert.deepEqual(events, ['create', 'click', 'revoke'])
    assert.equal(createObjectURL.mock.calls.length, 1)
    assert.equal(revokeObjectURL.mock.calls.length, 1)
    assert.equal(document.querySelector('a[download]'), null)
  })
})
