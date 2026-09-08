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
  QuotePriceSource,
  QuoteProviderSection,
  QuoteUsageExample,
  QuotationSnapshot,
} from '../types'
import { formatQuoteAmount } from './quotation-format'

const CSP =
  "default-src 'none'; style-src 'unsafe-inline'; img-src data:; font-src data:; base-uri 'none'; form-action 'none'"

export type QuotationHtmlLabels = Readonly<{
  emptyValue: string
  customer: string
  quotedBy: string
  quoteDate: string
  globalDiscount: string
  priceBasis: string
  pricingVersion: string
  fetchedAt: string
  rawPriceBasis: string
  groupPriceBasis: string
  discount: string
  discountCoefficient: string
  discountSuffix: string
  providerNote: string
  priceDimension: string
  sourceType: string
  catalogPrice: string
  basisPrice: string
  quotePrice: string
  currency: string
  unit: string
  condition: string
  status: string
  ready: string
  needsConfirmation: string
  groupUnavailable: string
  catalogMissing: string
  noDimensions: string
  noProviderModels: string
  noModels: string
  usageExamples: string
  sourceTypes: Readonly<Record<QuotePriceSource, string>>
}>

export type QuotationHtmlOptions = Readonly<{
  locale: string
  labels: QuotationHtmlLabels
  /** Exact raw dimension.label -> localized display text. */
  dimensionLabels: Readonly<Record<string, string>>
  /** Exact raw dimension.unit -> localized display text. */
  unitLabels: Readonly<Record<string, string>>
  /** Exact raw dimension.condition -> localized display text. */
  conditionLabels?: Readonly<Record<string, string>>
}>

export function escapeHtml(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;')
}

function displayText(
  value: string | null | undefined,
  labels: QuotationHtmlLabels
): string {
  return escapeHtml(value || labels.emptyValue)
}

function displayMappedText(
  value: string | null | undefined,
  lookup: Readonly<Record<string, string>> | undefined,
  labels: QuotationHtmlLabels
): string {
  if (!value) return escapeHtml(labels.emptyValue)
  const mapped = lookup && Object.hasOwn(lookup, value) ? lookup[value] : value
  return displayText(mapped, labels)
}

function displayNumber(
  value: number | null | undefined,
  labels: QuotationHtmlLabels
): string {
  return typeof value === 'number' && Number.isFinite(value)
    ? escapeHtml(String(value))
    : escapeHtml(labels.emptyValue)
}

function displayDiscount(
  value: number | null,
  labels: QuotationHtmlLabels
): string {
  return typeof value === 'number' && Number.isFinite(value)
    ? `${escapeHtml(String(value))}${escapeHtml(labels.discountSuffix)}`
    : escapeHtml(labels.emptyValue)
}

function displayAmount(
  amount: number | null,
  dimension: QuotePriceDimension,
  labels: QuotationHtmlLabels
): string {
  return typeof amount === 'number' && Number.isFinite(amount)
    ? escapeHtml(formatQuoteAmount(amount, dimension.currency))
    : escapeHtml(labels.emptyValue)
}

function priceBasisLabel(
  snapshot: QuotationSnapshot,
  labels: QuotationHtmlLabels
): string {
  if (snapshot.priceBasis.type === 'raw') {
    return escapeHtml(labels.rawPriceBasis)
  }
  return `${escapeHtml(labels.groupPriceBasis)}: ${displayText(snapshot.priceBasis.group, labels)} (${displayNumber(snapshot.priceBasis.ratio, labels)}x)`
}

function statusLabel(
  dimension: QuotePriceDimension,
  labels: QuotationHtmlLabels
): string {
  return escapeHtml(
    dimension.status === 'needs_confirmation'
      ? labels.needsConfirmation
      : labels.ready
  )
}

function renderDimension(
  dimension: QuotePriceDimension,
  { labels, dimensionLabels, unitLabels, conditionLabels }: QuotationHtmlOptions
): string {
  const statusClass =
    dimension.status === 'needs_confirmation' ? 'status pending' : 'status'
  return `<tr>
    <td>${displayMappedText(dimension.label, dimensionLabels, labels)}</td>
    <td>${displayText(labels.sourceTypes[dimension.sourceType], labels)}</td>
    <td class="number">${displayAmount(dimension.catalogAmount, dimension, labels)}</td>
    <td class="number">${displayAmount(dimension.sourceAmount, dimension, labels)}</td>
    <td class="number quote-price">${displayAmount(dimension.quoteAmount, dimension, labels)}</td>
    <td>${displayText(dimension.currency, labels)}</td>
    <td>${displayMappedText(dimension.unit, unitLabels, labels)}</td>
    <td>${displayMappedText(dimension.condition, conditionLabels, labels)}</td>
    <td><span class="${statusClass}">${statusLabel(dimension, labels)}</span></td>
  </tr>`
}

function unavailableLabel(
  model: QuoteModelSection,
  labels: QuotationHtmlLabels
): string {
  if (model.available) return ''
  if (model.unavailableReason === 'group_unavailable') {
    return `<span class="status pending">${escapeHtml(labels.groupUnavailable)}</span>`
  }
  return `<span class="status pending">${escapeHtml(labels.catalogMissing)}</span>`
}

function renderUsageExample(
  example: QuoteUsageExample,
  labels: QuotationHtmlLabels
): string {
  const facts = Object.entries(example.facts)
    .map(
      ([key, value]) => `<div class="usage-fact">
        <dt>${displayText(key, labels)}</dt>
        <dd>${typeof value === 'number' ? displayNumber(value, labels) : displayText(value, labels)}</dd>
      </div>`
    )
    .join('\n')

  return `<article class="usage-example">
    <h4>${displayText(example.label, labels)}</h4>
    <dl class="usage-facts">${facts}</dl>
  </article>`
}

function renderUsageExamples(
  model: QuoteModelSection,
  labels: QuotationHtmlLabels
): string {
  const examples = model.usageExamples ?? []
  if (examples.length === 0) return ''

  return `<section class="usage-examples">
    <h4 class="usage-title">${escapeHtml(labels.usageExamples)}</h4>
    ${examples.map((example) => renderUsageExample(example, labels)).join('\n')}
  </section>`
}

function renderModel(
  model: QuoteModelSection,
  options: QuotationHtmlOptions
): string {
  const { labels } = options
  const dimensions = model.dimensions
    .map((dimension) => renderDimension(dimension, options))
    .join('\n')
  const emptyRow = `<tr><td colspan="9" class="empty">${escapeHtml(labels.noDimensions)}</td></tr>`

  return `<section class="model-block">
    <div class="model-heading">
      <div>
        <h3>${displayText(model.displayName, labels)}</h3>
        <p class="model-id">${displayText(model.modelId, labels)}</p>
      </div>
      ${unavailableLabel(model, labels)}
    </div>
    <table>
      <thead>
        <tr>
          <th>${escapeHtml(labels.priceDimension)}</th>
          <th>${escapeHtml(labels.sourceType)}</th>
          <th>${escapeHtml(labels.catalogPrice)}</th>
          <th>${escapeHtml(labels.basisPrice)}</th>
          <th>${escapeHtml(labels.quotePrice)}</th>
          <th>${escapeHtml(labels.currency)}</th>
          <th>${escapeHtml(labels.unit)}</th>
          <th>${escapeHtml(labels.condition)}</th>
          <th>${escapeHtml(labels.status)}</th>
        </tr>
      </thead>
      <tbody>${dimensions || emptyRow}</tbody>
    </table>
    ${renderUsageExamples(model, labels)}
  </section>`
}

function renderProvider(
  provider: QuoteProviderSection,
  options: QuotationHtmlOptions,
  globalDiscount: number | null
): string {
  const { labels } = options
  const models = provider.models
    .map((model) => renderModel(model, options))
    .join('\n')
  const note = provider.note
    ? `<div class="provider-note"><strong>${escapeHtml(labels.providerNote)}</strong><div>${escapeHtml(provider.note)}</div></div>`
    : ''

  return `<section class="provider-block">
    <div class="provider-heading">
      <h2>${displayText(provider.providerName, labels)}</h2>
      <div class="provider-pricing">
        <span>${escapeHtml(labels.discount)}: ${displayDiscount(provider.discount ?? globalDiscount, labels)}</span>
        <span>${escapeHtml(labels.discountCoefficient)}: ${displayNumber(provider.discountCoefficient, labels)}</span>
      </div>
    </div>
    ${note}
    ${models || `<p class="empty">${escapeHtml(labels.noProviderModels)}</p>`}
  </section>`
}

export function buildQuotationHtml(
  snapshot: QuotationSnapshot,
  options: QuotationHtmlOptions
): string {
  const { locale, labels } = options
  const providers = snapshot.providers
    .map((provider) =>
      renderProvider(provider, options, snapshot.globalDiscount)
    )
    .join('\n')

  return `<!doctype html>
<html lang="${escapeHtml(locale)}">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="referrer" content="no-referrer">
  <meta http-equiv="Content-Security-Policy" content="${escapeHtml(CSP)}">
  <title>${displayText(snapshot.title, labels)}</title>
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
    .usage-examples { max-width: 100%; margin-top: 4mm; }
    .usage-title { margin: 0 0 2mm; font-size: 13px; }
    .usage-example { max-width: 100%; margin-top: 2mm; padding: 2.5mm; border: 1px solid #d7dce3; break-inside: avoid; page-break-inside: avoid; overflow: hidden; }
    .usage-example h4 { margin: 0 0 1.5mm; font-size: 12px; overflow-wrap: anywhere; }
    .usage-facts { display: grid; gap: 1mm; margin: 0; overflow-wrap: anywhere; }
    .usage-fact { display: grid; grid-template-columns: minmax(24mm, 1fr) minmax(0, 2fr); gap: 3mm; min-width: 0; }
    .usage-fact dt { color: #5f6b7a; font-weight: 700; }
    .usage-fact dd { min-width: 0; margin: 0; }
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
      .provider-block, .model-block, .provider-note, .usage-example, tr { break-inside: avoid; page-break-inside: avoid; }
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
      <h1>${displayText(snapshot.title, labels)}</h1>
      <dl class="metadata">
        <div><dt>${escapeHtml(labels.customer)}</dt><dd>${displayText(snapshot.customer, labels)}</dd></div>
        <div><dt>${escapeHtml(labels.quotedBy)}</dt><dd>${displayText(snapshot.quotedBy, labels)}</dd></div>
        <div><dt>${escapeHtml(labels.quoteDate)}</dt><dd>${displayText(snapshot.quoteDate, labels)}</dd></div>
        <div><dt>${escapeHtml(labels.globalDiscount)}</dt><dd>${displayDiscount(snapshot.globalDiscount, labels)}</dd></div>
        <div><dt>${escapeHtml(labels.priceBasis)}</dt><dd>${priceBasisLabel(snapshot, labels)}</dd></div>
        <div><dt>${escapeHtml(labels.pricingVersion)}</dt><dd>${displayText(snapshot.pricingVersion, labels)}</dd></div>
        <div><dt>${escapeHtml(labels.fetchedAt)}</dt><dd>${displayText(snapshot.fetchedAt, labels)}</dd></div>
      </dl>
    </header>
    ${providers || `<p class="empty-state">${escapeHtml(labels.noModels)}</p>`}
  </main>
</body>
</html>`
}
