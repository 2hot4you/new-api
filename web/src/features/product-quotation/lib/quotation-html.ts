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
import type {
  QuoteModelSection,
  QuotePriceDimension,
  QuoteProviderSection,
  QuotationSnapshot,
} from '../types'
import { formatQuoteAmount } from './quotation-format'

const CSP =
  "default-src 'none'; style-src 'unsafe-inline'; img-src data:; font-src data:; base-uri 'none'; form-action 'none'"

export function escapeHtml(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;')
}

function displayText(value: string | null | undefined): string {
  return escapeHtml(value || '—')
}

function displayNumber(value: number | null | undefined): string {
  return typeof value === 'number' && Number.isFinite(value)
    ? escapeHtml(String(value))
    : '—'
}

function displayDiscount(value: number | null): string {
  return typeof value === 'number' && Number.isFinite(value)
    ? `${escapeHtml(String(value))} 折`
    : '—'
}

function priceBasisLabel(snapshot: QuotationSnapshot): string {
  if (snapshot.priceBasis.type === 'raw') return '原始模型价（1x）'
  return `用户分组：${displayText(snapshot.priceBasis.group)}（${displayNumber(snapshot.priceBasis.ratio)}x）`
}

function statusLabel(dimension: QuotePriceDimension): string {
  return dimension.status === 'needs_confirmation' ? '待确认' : '已确认'
}

function renderDimension(dimension: QuotePriceDimension): string {
  const statusClass =
    dimension.status === 'needs_confirmation' ? 'status pending' : 'status'
  return `<tr>
    <td>${displayText(dimension.label)}</td>
    <td>${displayText(dimension.sourceType)}</td>
    <td class="number">${escapeHtml(formatQuoteAmount(dimension.catalogAmount, dimension.currency))}</td>
    <td class="number">${escapeHtml(formatQuoteAmount(dimension.sourceAmount, dimension.currency))}</td>
    <td class="number quote-price">${escapeHtml(formatQuoteAmount(dimension.quoteAmount, dimension.currency))}</td>
    <td>${displayText(dimension.currency)}</td>
    <td>${displayText(dimension.unit)}</td>
    <td>${displayText(dimension.condition)}</td>
    <td><span class="${statusClass}">${statusLabel(dimension)}</span></td>
  </tr>`
}

function unavailableLabel(model: QuoteModelSection): string {
  if (model.available) return ''
  if (model.unavailableReason === 'group_unavailable') {
    return '<span class="status pending">所选分组不可用</span>'
  }
  return '<span class="status pending">价格目录中不存在</span>'
}

function renderModel(model: QuoteModelSection): string {
  const dimensions = model.dimensions.map(renderDimension).join('\n')
  const emptyRow = `<tr><td colspan="9" class="empty">没有可显示的价格维度</td></tr>`

  return `<section class="model-block">
    <div class="model-heading">
      <div>
        <h3>${displayText(model.displayName)}</h3>
        <p class="model-id">${displayText(model.modelId)}</p>
      </div>
      ${unavailableLabel(model)}
    </div>
    <table>
      <thead>
        <tr>
          <th>价格维度</th>
          <th>来源类型</th>
          <th>目录原价</th>
          <th>基准价格</th>
          <th>报价</th>
          <th>币种</th>
          <th>单位</th>
          <th>条件</th>
          <th>状态</th>
        </tr>
      </thead>
      <tbody>${dimensions || emptyRow}</tbody>
    </table>
  </section>`
}

function renderProvider(provider: QuoteProviderSection): string {
  const models = provider.models.map(renderModel).join('\n')
  const note = provider.note
    ? `<div class="provider-note"><strong>Provider 备注</strong><div>${escapeHtml(provider.note)}</div></div>`
    : ''

  return `<section class="provider-block">
    <div class="provider-heading">
      <h2>${displayText(provider.providerName)}</h2>
      <div class="provider-pricing">
        <span>折扣：${displayDiscount(provider.discount)}</span>
        <span>折扣系数：${displayNumber(provider.discountCoefficient)}</span>
      </div>
    </div>
    ${note}
    ${models || '<p class="empty">此 Provider 没有选中的模型。</p>'}
  </section>`
}

export function buildQuotationHtml(snapshot: QuotationSnapshot): string {
  const providers = snapshot.providers.map(renderProvider).join('\n')

  return `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="referrer" content="no-referrer">
  <meta http-equiv="Content-Security-Policy" content="${escapeHtml(CSP)}">
  <title>${displayText(snapshot.title)}</title>
  <style>
    :root { color-scheme: light; font-family: Arial, "Noto Sans SC", sans-serif; color: #172033; background: #eef1f5; }
    * { box-sizing: border-box; }
    body { margin: 0; padding: 24px; background: #eef1f5; line-height: 1.45; }
    .sheet { width: min(100%, 210mm); min-height: 297mm; margin: 0 auto; padding: 16mm 14mm; background: #fff; box-shadow: 0 8px 28px rgb(15 23 42 / 12%); }
    .document-header { padding-bottom: 9mm; border-bottom: 2px solid #172033; }
    h1, h2, h3, p { margin-top: 0; }
    h1 { margin-bottom: 5mm; font-size: 28px; line-height: 1.2; overflow-wrap: anywhere; }
    h2 { margin-bottom: 0; font-size: 20px; overflow-wrap: anywhere; }
    h3 { margin-bottom: 1mm; font-size: 16px; overflow-wrap: anywhere; }
    .metadata { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 2.5mm 8mm; margin: 0; }
    .metadata div { min-width: 0; }
    .metadata dt { color: #5f6b7a; font-size: 11px; font-weight: 700; letter-spacing: .03em; text-transform: uppercase; }
    .metadata dd { margin: .5mm 0 0; overflow-wrap: anywhere; }
    .provider-block { margin-top: 10mm; break-inside: avoid; page-break-inside: avoid; }
    .provider-heading { display: flex; align-items: baseline; justify-content: space-between; gap: 6mm; padding-bottom: 2mm; border-bottom: 1px solid #aab2bf; }
    .provider-pricing { display: flex; flex-wrap: wrap; gap: 4mm; color: #445064; font-size: 12px; }
    .provider-note { margin: 3mm 0; padding: 3mm; border-left: 3px solid #66758c; background: #f6f7f9; break-inside: avoid; page-break-inside: avoid; }
    .provider-note > div { margin-top: 1mm; white-space: pre-wrap; overflow-wrap: anywhere; }
    .model-block { margin-top: 5mm; break-inside: avoid; page-break-inside: avoid; }
    .model-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 4mm; }
    .model-id { margin-bottom: 2mm; color: #5f6b7a; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 11px; overflow-wrap: anywhere; }
    table { width: 100%; border-collapse: collapse; table-layout: fixed; font-size: 10px; }
    thead { display: table-header-group; }
    tr { break-inside: avoid; page-break-inside: avoid; }
    th, td { padding: 2mm 1.5mm; border: 1px solid #d7dce3; vertical-align: top; text-align: left; overflow-wrap: anywhere; word-break: break-word; }
    th { background: #f1f3f6; color: #394457; font-weight: 700; }
    .number { font-variant-numeric: tabular-nums; }
    .quote-price { font-weight: 700; }
    .status { display: inline-block; border-radius: 999px; padding: .5mm 1.5mm; color: #17633a; background: #e8f6ee; white-space: nowrap; }
    .status.pending { color: #8a4b08; background: #fff0db; }
    .empty { color: #697586; font-style: italic; }
    .empty-state { margin-top: 10mm; padding: 8mm; border: 1px dashed #aab2bf; color: #697586; text-align: center; }
    @page { size: A4; margin: 12mm; }
    @media print {
      :root, body { background: #fff; }
      body { padding: 0; }
      .sheet { width: auto; min-height: auto; margin: 0; padding: 0; box-shadow: none; }
      .document-header { break-after: avoid; page-break-after: avoid; }
      .provider-block, .model-block, .provider-note, tr { break-inside: avoid; page-break-inside: avoid; }
    }
    @media (max-width: 720px) {
      body { padding: 0; }
      .sheet { min-height: 100vh; padding: 8mm 5mm; box-shadow: none; }
      .metadata { grid-template-columns: 1fr; }
      .provider-heading { display: block; }
      table { font-size: 9px; }
    }
  </style>
</head>
<body>
  <main class="sheet">
    <header class="document-header">
      <h1>${displayText(snapshot.title)}</h1>
      <dl class="metadata">
        <div><dt>客户</dt><dd>${displayText(snapshot.customer)}</dd></div>
        <div><dt>报价人</dt><dd>${displayText(snapshot.quotedBy)}</dd></div>
        <div><dt>报价日期</dt><dd>${displayText(snapshot.quoteDate)}</dd></div>
        <div><dt>全局折扣</dt><dd>${displayDiscount(snapshot.globalDiscount)}</dd></div>
        <div><dt>价格基准</dt><dd>${priceBasisLabel(snapshot)}</dd></div>
        <div><dt>价格版本</dt><dd>${displayText(snapshot.pricingVersion)}</dd></div>
        <div><dt>价格获取时间</dt><dd>${displayText(snapshot.fetchedAt)}</dd></div>
      </dl>
    </header>
    ${providers || '<p class="empty-state">没有选中的模型。</p>'}
  </main>
</body>
</html>`
}
