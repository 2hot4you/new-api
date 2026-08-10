/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import type { TaskBillingSummary } from '../types'
import { formatGrokVideoFormula } from './grok-video-billing.ts'

export type TaskBillingDisplay =
  | { kind: 'pending' }
  | { kind: 'refunded' }
  | { kind: 'refund_pending' }
  | { kind: 'unavailable' }
  | { kind: 'settled'; amount: number }

export function getTaskBillingDisplay(
  billing: TaskBillingSummary
): TaskBillingDisplay {
  switch (billing.state) {
    case 'pending':
    case 'refunded':
    case 'refund_pending':
    case 'unavailable':
      return { kind: billing.state }
    case 'settled':
      return { kind: 'settled', amount: billing.final_cost }
    default:
      return { kind: 'unavailable' }
  }
}

export function formatTaskBillingCny(value: number): string {
  return `¥${value.toFixed(6)}`
}

export function formatTaskBillingFormula(
  billing: TaskBillingSummary
): string | null {
  if (!billing.detail_available || billing.state !== 'settled') return null
  if (billing.mode === 'grok_video' && billing.grok_video) {
    return formatGrokVideoFormula(billing.grok_video)
  }
  const seedance = billing.seedance
  if (!seedance || seedance.actual_tokens <= 0) return null
  return `${seedance.actual_tokens} × ${formatTaskBillingCny(seedance.unit_price)} ÷ 1,000,000 × ${billing.group_ratio.toFixed(4)} = ${formatTaskBillingCny(billing.final_cost)}`
}
