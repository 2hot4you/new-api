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
import type { BillingUsageSchema } from '../../types'
import {
  billingDateInZone,
  isHolidayAt,
  isValidBillingTimezone,
} from './calendar'
import { compileBillingExpression } from './parser'
import { evaluateBillingCondition } from './runtime'
import {
  TIME_DEPENDENT_FUNCTIONS,
  TIME_FUNCTIONS,
  expressionDependencies,
  type ExpressionNode,
  type CompiledBillingExpression,
  type TokenVariable,
} from './types'

export type TokenTierCondition = {
  var: 'p' | 'c' | 'len'
  op: '<' | '<=' | '>' | '>='
  value: number
}
export type TokenTier = {
  conditionText?: string
  imageCount?: boolean
  billingUnit?: 'token' | 'request'
  fixedPrice?: number
  label: string
  conditions: TokenTierCondition[]
  prices: Partial<Record<TokenVariable, number>>
}
export type TimeTokenTier = TokenTier & {
  conditionText: string
  timeConditions: { condition: ExpressionNode; matches: boolean }[]
}
export type DailyTimePricingPeriod = {
  /** Inclusive minute of the local day, from 0 through 1439. */
  startMinute: number
  /** Exclusive minute of the local day, from 1 through 1440. */
  endMinute: number
  tiers: TokenTier[]
}

export function flattenBinary(
  node: ExpressionNode,
  operator: string
): ExpressionNode[] {
  if (node.kind !== 'binary' || node.operator !== operator) return [node]
  return [
    ...flattenBinary(node.left, operator),
    ...flattenBinary(node.right, operator),
  ]
}

function tokenConditions(node: ExpressionNode): TokenTierCondition[] | null {
  const conditions: TokenTierCondition[] = []
  for (const part of flattenBinary(node, '&&')) {
    if (
      part.kind !== 'binary' ||
      !['<', '<=', '>', '>='].includes(part.operator) ||
      part.left.kind !== 'variable' ||
      !['p', 'c', 'len'].includes(part.left.name) ||
      part.right.kind !== 'literal' ||
      typeof part.right.value !== 'number' ||
      part.right.value < 0
    ) {
      return null
    }
    conditions.push({
      var: part.left.name as TokenTierCondition['var'],
      op: part.operator as TokenTierCondition['op'],
      value: part.right.value,
    })
  }
  return conditions
}

function nonnegativePriceLiteral(node: ExpressionNode): number | null {
  if (
    node.kind === 'literal' &&
    typeof node.value === 'number' &&
    node.value >= 0
  ) {
    return node.value
  }
  if (
    node.kind === 'unary' &&
    node.operator === '-' &&
    node.operand.kind === 'literal' &&
    node.operand.value === 0
  ) {
    return -0
  }
  return null
}

function tokenTier(
  node: ExpressionNode,
  conditions: TokenTierCondition[]
): TokenTier | null {
  if (
    node.kind !== 'call' ||
    node.name !== 'tier' ||
    node.args[0]?.kind !== 'literal' ||
    typeof node.args[0].value !== 'string'
  ) {
    return null
  }
  const prices: TokenTier['prices'] = {}
  const body = node.args[1]
  if (
    body.kind === 'call' &&
    body.name === 'fixed' &&
    body.args[0].kind === 'literal' &&
    typeof body.args[0].value === 'number'
  ) {
    return {
      label: node.args[0].value,
      conditions,
      prices,
      billingUnit: 'request',
      fixedPrice: body.args[0].value,
    }
  }
  for (const term of flattenBinary(node.args[1], '+')) {
    if (term.kind !== 'binary' || term.operator !== '*') {
      return null
    }
    let input = term.left
    let price = nonnegativePriceLiteral(term.right)
    if (price === null) {
      price = nonnegativePriceLiteral(term.left)
      if (price === null) return null
      input = term.right
    }
    let variable: TokenVariable
    if (
      input.kind === 'variable' &&
      input.name !== 'len' &&
      input.name !== 'image_count'
    ) {
      variable = input.name
    } else {
      if (
        input.kind !== 'call' ||
        input.name !== 'max' ||
        input.args.length !== 2
      ) {
        return null
      }
      let remainder = input.args[0]
      if (remainder.kind === 'literal' && remainder.value === 0) {
        remainder = input.args[1]
      } else if (
        input.args[1].kind !== 'literal' ||
        input.args[1].value !== 0
      ) {
        return null
      }
      if (
        remainder.kind !== 'binary' ||
        remainder.operator !== '-' ||
        remainder.left.kind !== 'variable' ||
        remainder.left.name !== 'len' ||
        remainder.right.kind !== 'variable' ||
        remainder.right.name !== 'ai'
      ) {
        return null
      }
      variable = 'p'
    }
    if (Object.hasOwn(prices, variable)) return null
    prices[variable] = price
  }
  if (Object.keys(prices).length === 0) return null
  return { label: node.args[0].value, conditions, prices }
}

/** Legacy token summary contract: ordered linear chain, never a minimum or partial price extraction. */
export function readTokenTierChain(node: ExpressionNode): TokenTier[] | null {
  if (
    node.kind === 'conditional' &&
    node.condition.kind === 'binary' &&
    node.condition.operator === '||'
  ) {
    const inputs = [node.condition.left, node.condition.right]
    const present = new Set<string>()
    for (const part of inputs) {
      if (
        part.kind === 'binary' &&
        part.operator === '>' &&
        part.left.kind === 'variable' &&
        (part.left.name === 'ai' || part.left.name === 'ao') &&
        part.right.kind === 'literal' &&
        part.right.value === 0
      ) {
        present.add(part.left.name)
      }
    }
    if (present.size === 2) {
      const audio = tokenTier(node.yes, [])
      const text = tokenTier(node.no, [])
      if (audio && text) {
        return [
          { ...audio, conditionText: 'Audio requests' },
          { ...text, conditionText: 'Text-only requests' },
        ]
      }
    }
  }
  if (node.kind === 'binary' && node.operator === '*') {
    for (const [factor, pricing] of [
      [node.left, node.right],
      [node.right, node.left],
    ]) {
      if (factor.kind === 'variable' && factor.name === 'image_count') {
        return (
          readTokenTierChain(pricing)?.map((tier) => ({
            ...tier,
            imageCount: true,
          })) ?? null
        )
      }
    }
  }
  const tiers: TokenTier[] = []
  let remaining = node
  while (remaining.kind === 'conditional') {
    const conditions = tokenConditions(remaining.condition)
    if (!conditions) return null
    const tier = tokenTier(remaining.yes, conditions)
    if (!tier) return null
    tiers.push(tier)
    remaining = remaining.no
  }
  const fallback = tokenTier(remaining, [])
  if (!fallback) return null
  return [...tiers, fallback]
}

export function isTimeCondition(node: ExpressionNode): boolean {
  const dependencies = expressionDependencies(node)
  return (
    dependencies.variables.size === 0 &&
    [...dependencies.functions].some((name) =>
      (TIME_DEPENDENT_FUNCTIONS as readonly string[]).includes(name)
    ) &&
    [...dependencies.functions].every((name) =>
      [
        ...TIME_DEPENDENT_FUNCTIONS,
        'min',
        'max',
        'abs',
        'ceil',
        'floor',
      ].includes(name)
    )
  )
}

function timeTierBranches(
  compiled: CompiledBillingExpression,
  node: ExpressionNode,
  path: TimeTokenTier['timeConditions']
): TimeTokenTier[] | null {
  if (node.kind === 'conditional' && isTimeCondition(node.condition)) {
    const yes = timeTierBranches(compiled, node.yes, [
      ...path,
      { condition: node.condition, matches: true },
    ])
    const no = timeTierBranches(compiled, node.no, [
      ...path,
      { condition: node.condition, matches: false },
    ])
    if (!yes || !no) return null
    return [...yes, ...no]
  }
  const tiers = readTokenTierChain(node)
  if (!tiers || path.length === 0) return null
  const timeDescription = path
    .map(({ condition, matches }) => {
      const source = compiled.source.slice(condition.start, condition.end)
      return matches ? `(${source})` : `!(${source})`
    })
    .join(' && ')
  return tiers.map((tier) => ({
    ...tier,
    timeConditions: path,
    conditionText: [
      timeDescription,
      ...tier.conditions.map(
        (condition) => `${condition.var} ${condition.op} ${condition.value}`
      ),
    ].join(' && '),
  }))
}

export function readTimeTokenPricing(
  source: string,
  now?: Date
): { tiers: TimeTokenTier[]; currentTiers: TimeTokenTier[] } | null {
  const compiled = compileBillingExpression(source)
  if (compiled.status !== 'ready') return null
  const tiers = timeTierBranches(compiled, compiled.ast, [])
  if (!tiers) return null
  const currentTiers = now
    ? tiers.filter((tier) =>
        tier.timeConditions.every(
          ({ condition, matches }) =>
            evaluateBillingCondition(compiled, condition, now) === matches
        )
      )
    : []
  return { tiers, currentTiers }
}

function localMinuteInstant(
  day: string,
  minute: number,
  timezone: string,
  formatter: Intl.DateTimeFormat
): Date | null {
  const match = day.match(/^(\d{4})-(\d{2})-(\d{2})$/)
  if (!match || minute < 0 || minute >= 1440) return null
  const year = Number(match[1])
  const month = Number(match[2])
  const date = Number(match[3])
  const hour = Math.floor(minute / 60)
  const minuteOfHour = minute % 60
  const calendarCheck = new Date(Date.UTC(year, month - 1, date))
  if (
    calendarCheck.getUTCFullYear() !== year ||
    calendarCheck.getUTCMonth() !== month - 1 ||
    calendarCheck.getUTCDate() !== date
  ) {
    return null
  }
  let timestamp = Date.UTC(year, month - 1, date, hour, minuteOfHour)
  for (let attempt = 0; attempt < 3; attempt++) {
    const parts = Object.fromEntries(
      formatter
        .formatToParts(new Date(timestamp))
        .map((part) => [part.type, part.value])
    )
    const represented = Date.UTC(
      Number(parts.year),
      Number(parts.month) - 1,
      Number(parts.day),
      Number(parts.hour),
      Number(parts.minute)
    )
    const desired = Date.UTC(year, month - 1, date, hour, minuteOfHour)
    if (represented === desired) break
    timestamp += desired - represented
  }
  const instant = new Date(timestamp)
  const parts = Object.fromEntries(
    formatter.formatToParts(instant).map((part) => [part.type, part.value])
  )
  if (
    billingDateInZone(instant, timezone) !== day ||
    Number(parts.hour) !== hour ||
    Number(parts.minute) !== minuteOfHour
  ) {
    return null
  }
  return instant
}

function publicTimeTier(tier: TimeTokenTier): TokenTier {
  return {
    label: tier.label,
    conditions: tier.conditions,
    prices: tier.prices,
    ...(tier.imageCount ? { imageCount: true } : {}),
    ...(tier.billingUnit ? { billingUnit: tier.billingUnit } : {}),
    ...(tier.fixedPrice !== undefined ? { fixedPrice: tier.fixedPrice } : {}),
  }
}

type DailyTierPlan = {
  tier: TokenTier
  timeConditions: TimeTokenTier['timeConditions']
}

type DailyPricingPlan = {
  tiers: DailyTierPlan[]
  rules: CompiledBillingExpression['requestRules']
}

function dailyPricingPlan(
  compiled: CompiledBillingExpression
): DailyPricingPlan | null {
  const ruleByNode = new Map(
    compiled.requestRules.map((rule) => [rule.node, rule] as const)
  )
  const rules: CompiledBillingExpression['requestRules'] = []
  const baseParts: ExpressionNode[] = []
  for (const part of flattenBinary(compiled.ast, '*')) {
    const rule = ruleByNode.get(part)
    if (!rule) {
      baseParts.push(part)
      continue
    }
    if (!isTimeCondition(rule.condition)) return null
    rules.push(rule)
  }
  if (baseParts.length !== 1) return null

  const timeTiers = timeTierBranches(compiled, baseParts[0], [])
  if (timeTiers) {
    return {
      rules,
      tiers: timeTiers.map((tier) => ({
        tier: publicTimeTier(tier),
        timeConditions: tier.timeConditions,
      })),
    }
  }
  const tiers = readTokenTierChain(baseParts[0])
  if (!tiers) return null
  return {
    rules,
    tiers: tiers.map((tier) => ({ tier, timeConditions: [] })),
  }
}

const WEEKDAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']
const normalizedTimezoneCache = new Map<string, string | null>()

function normalizedTimezone(zone: string): string | null {
  const timezone = zone.trim() || 'UTC'
  if (normalizedTimezoneCache.has(timezone)) {
    return normalizedTimezoneCache.get(timezone) ?? null
  }
  if (timezone === 'Local') return null
  try {
    if (!/^[A-Z][A-Za-z0-9_+-]*(?:\/[A-Z][A-Za-z0-9_+-]*)*$/.test(timezone)) {
      throw new RangeError('timezone')
    }
    const resolved = new Intl.DateTimeFormat('en-US', {
      timeZone: timezone,
    }).resolvedOptions().timeZone
    if (
      resolved.toLowerCase() === timezone.toLowerCase() &&
      resolved !== timezone
    ) {
      throw new RangeError('timezone')
    }
    normalizedTimezoneCache.set(timezone, timezone)
    return timezone
  } catch {
    normalizedTimezoneCache.set(timezone, 'UTC')
    return 'UTC'
  }
}

type TimeEvaluationContext = {
  now: Date
  timeFormatters: Map<string, Intl.DateTimeFormat>
  dateFormatters: Map<string, Intl.DateTimeFormat>
  timeValues: Map<string, Record<(typeof TIME_FUNCTIONS)[number], number>>
  dates: Map<string, string>
  holidays: Map<string, boolean>
}

function timeValues(
  zone: string,
  context: TimeEvaluationContext
): Record<(typeof TIME_FUNCTIONS)[number], number> {
  const timezone = normalizedTimezone(zone)
  if (!timezone) throw new Error('server Local timezone')
  const cached = context.timeValues.get(timezone)
  if (cached) return cached
  let formatter = context.timeFormatters.get(timezone)
  if (!formatter) {
    formatter = new Intl.DateTimeFormat('en-US', {
      timeZone: timezone,
      calendar: 'gregory',
      numberingSystem: 'latn',
      hourCycle: 'h23',
      year: 'numeric',
      hour: 'numeric',
      minute: 'numeric',
      weekday: 'short',
      month: 'numeric',
      day: 'numeric',
    })
    context.timeFormatters.set(timezone, formatter)
  }
  const parts = Object.fromEntries(
    formatter.formatToParts(context.now).map((part) => [part.type, part.value])
  )
  const values = {
    hour: Number(parts.hour),
    minute: Number(parts.minute),
    weekday: WEEKDAYS.indexOf(parts.weekday),
    month: Number(parts.month),
    day: Number(parts.day),
  }
  context.timeValues.set(timezone, values)
  return values
}

function dateInZone(zone: string, context: TimeEvaluationContext): string {
  const timezone = normalizedTimezone(zone)
  if (!timezone) throw new Error('server Local timezone')
  const cached = context.dates.get(timezone)
  if (cached) return cached
  let formatter = context.dateFormatters.get(timezone)
  if (!formatter) {
    formatter = new Intl.DateTimeFormat('en-US', {
      timeZone: timezone,
      calendar: 'gregory',
      numberingSystem: 'latn',
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
    })
    context.dateFormatters.set(timezone, formatter)
  }
  const parts = Object.fromEntries(
    formatter.formatToParts(context.now).map((part) => [part.type, part.value])
  )
  const date = `${parts.year}-${parts.month}-${parts.day}`
  context.dates.set(timezone, date)
  return date
}

function evaluateTimeNode(
  node: ExpressionNode,
  context: TimeEvaluationContext
): unknown {
  if (node.kind === 'literal') return node.value
  if (node.kind === 'variable') throw new Error('unexpected variable')
  if (node.kind === 'conditional') {
    const condition = evaluateTimeNode(node.condition, context)
    if (typeof condition !== 'boolean') throw new Error('boolean condition')
    return evaluateTimeNode(condition ? node.yes : node.no, context)
  }
  if (node.kind === 'unary') {
    const value = evaluateTimeNode(node.operand, context)
    if (node.operator === '!') {
      if (typeof value !== 'boolean') throw new Error('boolean operand')
      return !value
    }
    if (typeof value !== 'number') throw new Error('numeric operand')
    return node.operator === '-' ? -value : value
  }
  if (node.kind === 'binary') {
    const left = evaluateTimeNode(node.left, context)
    if (node.operator === '&&') {
      if (typeof left !== 'boolean') throw new Error('boolean operand')
      return left && Boolean(evaluateTimeNode(node.right, context))
    }
    if (node.operator === '||') {
      if (typeof left !== 'boolean') throw new Error('boolean operand')
      return left || Boolean(evaluateTimeNode(node.right, context))
    }
    const right = evaluateTimeNode(node.right, context)
    if (node.operator === '==') return left === right
    if (node.operator === '!=') return left !== right
    if (
      (typeof left === 'number' && typeof right === 'number') ||
      (typeof left === 'string' && typeof right === 'string')
    ) {
      if (node.operator === '<') return left < right
      if (node.operator === '<=') return left <= right
      if (node.operator === '>') return left > right
      if (node.operator === '>=') return left >= right
    }
    if (typeof left !== 'number' || typeof right !== 'number') {
      throw new Error('numeric operands')
    }
    if (node.operator === '+') return left + right
    if (node.operator === '-') return left - right
    if (node.operator === '*') return left * right
    if (node.operator === '/') return left / right
    if (node.operator === '%') return left % right
    throw new Error('unsupported operator')
  }

  const args = node.args.map((arg) => evaluateTimeNode(arg, context))
  if ((TIME_FUNCTIONS as readonly string[]).includes(node.name)) {
    if (typeof args[0] !== 'string') throw new Error('timezone')
    return timeValues(args[0], context)[
      node.name as (typeof TIME_FUNCTIONS)[number]
    ]
  }
  if (node.name === 'is_holiday') {
    if (typeof args[0] !== 'string' || typeof args[1] !== 'string') {
      throw new Error('holiday arguments')
    }
    const date = dateInZone(args[1], context)
    const key = `${args[0].trim().toUpperCase()}\u0000${args[1]}\u0000${date}`
    const cached = context.holidays.get(key)
    if (cached !== undefined) return cached
    const holiday = isHolidayAt(args[0], args[1], context.now)
    context.holidays.set(key, holiday)
    return holiday
  }
  if (node.name === 'min') return Math.min(Number(args[0]), Number(args[1]))
  if (node.name === 'max') return Math.max(Number(args[0]), Number(args[1]))
  if (node.name === 'abs') return Math.abs(Number(args[0]))
  if (node.name === 'ceil') return Math.ceil(Number(args[0]))
  if (node.name === 'floor') return Math.floor(Number(args[0]))
  throw new Error('unsupported call')
}

function scaledTier(tier: TokenTier, multiplier: number): TokenTier {
  return {
    ...tier,
    prices: Object.fromEntries(
      Object.entries(tier.prices).map(([name, price]) => [
        name,
        price === undefined ? undefined : price * multiplier,
      ])
    ),
    ...(tier.fixedPrice === undefined
      ? {}
      : { fixedPrice: tier.fixedPrice * multiplier }),
  }
}

/**
 * Evaluate one local calendar day at minute precision and merge adjacent,
 * identical tier/price results. Unsafe dates, zones or expressions return null.
 */
export function mergeDailyTimePricing(
  source: string,
  day: string,
  timezone: string
): DailyTimePricingPeriod[] | null {
  if (!isValidBillingTimezone(timezone)) return null
  const compiled = compileBillingExpression(source)
  if (compiled.status !== 'ready') return null
  const plan = dailyPricingPlan(compiled)
  if (!plan) return null
  const instantFormatter = new Intl.DateTimeFormat('en-US', {
    timeZone: timezone,
    calendar: 'gregory',
    numberingSystem: 'latn',
    hourCycle: 'h23',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
  const dayStart = localMinuteInstant(day, 0, timezone, instantFormatter)
  if (!dayStart) return null
  const dayEnd = new Date(dayStart.getTime() + 1439 * 60_000)
  const endParts = Object.fromEntries(
    instantFormatter
      .formatToParts(dayEnd)
      .map((part) => [part.type, part.value])
  )
  if (
    billingDateInZone(dayEnd, timezone) !== day ||
    Number(endParts.hour) !== 23 ||
    Number(endParts.minute) !== 59
  ) {
    return null
  }
  const sharedContext = {
    timeFormatters: new Map<string, Intl.DateTimeFormat>(),
    dateFormatters: new Map<string, Intl.DateTimeFormat>(),
    holidays: new Map<string, boolean>(),
  }
  const [yearText, monthText, dayText] = day.split('-')
  const localDate = new Date(`${day}T12:00:00Z`)
  const localDayValues = {
    hour: 0,
    minute: 0,
    weekday: localDate.getUTCDay(),
    month: Number(monthText),
    day: Number(dayText),
  }
  if (
    localDate.getUTCFullYear() !== Number(yearText) ||
    localDate.getUTCMonth() + 1 !== localDayValues.month ||
    localDate.getUTCDate() !== localDayValues.day
  ) {
    return null
  }
  const periods: DailyTimePricingPeriod[] = []
  let previousKey = ''
  for (let minute = 0; minute < 1440; minute++) {
    const now = new Date(dayStart.getTime() + minute * 60_000)
    const context: TimeEvaluationContext = {
      ...sharedContext,
      now,
      timeValues: new Map([
        [
          timezone,
          {
            ...localDayValues,
            hour: Math.floor(minute / 60),
            minute: minute % 60,
          },
        ],
      ]),
      dates: new Map([[timezone, day]]),
    }
    const conditions = new Map<ExpressionNode, boolean>()
    const matches = (condition: ExpressionNode): boolean => {
      const cached = conditions.get(condition)
      if (cached !== undefined) return cached
      const value = evaluateTimeNode(condition, context)
      if (typeof value !== 'boolean') throw new Error('boolean condition')
      conditions.set(condition, value)
      return value
    }
    let multiplier = 1
    try {
      for (const rule of plan.rules) {
        if (matches(rule.condition)) multiplier *= rule.multiplier
      }
    } catch {
      return null
    }
    const current = plan.tiers
      .filter(({ timeConditions }) =>
        timeConditions.every(
          ({ condition, matches: expected }) => matches(condition) === expected
        )
      )
      .map(({ tier }) => scaledTier(tier, multiplier))
    if (current.length === 0) return null
    const key = JSON.stringify(current)
    const previous = periods.at(-1)
    if (previous && key === previousKey) {
      previous.endMinute = minute + 1
    } else {
      periods.push({
        startMinute: minute,
        endMinute: minute + 1,
        tiers: current,
      })
      previousKey = key
    }
  }
  return periods
}

export type TaskTier = {
  conditionText?: string
  label: string
  conditions: { field: string; value: string }[]
  constant: number
  unitPrices: Record<string, number>
}

/** `unreachable` marks a branch on an enum value the schema no longer declares:
 * it can never match a request, so callers skip it instead of rejecting the chain. */
function taskConditions(
  node: ExpressionNode,
  schema: BillingUsageSchema,
  includeBoolean: boolean
): TaskTier['conditions'] | 'unreachable' | null {
  const conditions: TaskTier['conditions'] = []
  let reachable = true
  for (const term of flattenBinary(node, '&&')) {
    if (
      term.kind !== 'binary' ||
      term.operator !== '==' ||
      term.left.kind !== 'call' ||
      term.left.name !== 'u' ||
      term.left.args[0].kind !== 'literal' ||
      typeof term.left.args[0].value !== 'string' ||
      term.right.kind !== 'literal'
    ) {
      return null
    }
    const field = term.left.args[0].value
    const definition = schema[field]
    const value = term.right.value
    if (definition?.type === 'boolean') {
      if (!includeBoolean || typeof value !== 'boolean') return null
    } else if (typeof value !== 'string' || !definition?.enum) {
      return null
    } else if (!definition.enum.includes(value)) {
      reachable = false
    }
    conditions.push({ field, value: String(value) })
  }
  return reachable ? conditions : 'unreachable'
}

function taskTier(
  node: ExpressionNode,
  conditions: TaskTier['conditions'],
  schema: BillingUsageSchema
): TaskTier | null {
  if (
    node.kind !== 'call' ||
    node.name !== 'tier' ||
    node.args[0].kind !== 'literal' ||
    typeof node.args[0].value !== 'string'
  ) {
    return null
  }
  let constant = 0
  let hasConstant = false
  const unitPrices: Record<string, number> = {}
  for (let term of flattenBinary(node.args[1], '+')) {
    const literal = nonnegativePriceLiteral(term)
    if (literal !== null) {
      if (hasConstant) return null
      hasConstant = true
      constant = literal
      continue
    }
    const scaled = term.kind === 'binary' && term.operator === '/'
    if (scaled && term.kind === 'binary') {
      if (term.right.kind !== 'literal' || term.right.value !== 1000000) {
        return null
      }
      term = term.left
    }
    if (
      term.kind !== 'binary' ||
      term.operator !== '*' ||
      term.left.kind !== 'call' ||
      term.left.name !== 'u' ||
      term.left.args[0].kind !== 'literal' ||
      typeof term.left.args[0].value !== 'string' ||
      nonnegativePriceLiteral(term.right) === null
    ) {
      return null
    }
    const field = term.left.args[0].value
    const definition = schema[field]
    if (
      definition?.type !== 'number' ||
      !definition.unit ||
      Object.hasOwn(unitPrices, field)
    ) {
      return null
    }
    if ((definition.unit === 'token') !== scaled) return null
    unitPrices[field] = nonnegativePriceLiteral(term.right) ?? 0
  }
  if (Object.keys(unitPrices).length === 0) return null
  return { label: node.args[0].value, conditions, constant, unitPrices }
}

/** Keep task summaries limited to schema-backed enum tiers and canonical scaled units.
 * A branch on an enum value the schema no longer declares is unreachable and dropped,
 * so a narrowed plugin schema keeps the remaining tiers editable. */
export function readTaskTierChain(
  node: ExpressionNode,
  schema: BillingUsageSchema,
  includeBoolean: boolean
): TaskTier[] | null {
  const tiers: TaskTier[] = []
  let remaining = node
  while (remaining.kind === 'conditional') {
    const conditions = taskConditions(
      remaining.condition,
      schema,
      includeBoolean
    )
    if (conditions === null) return null
    if (conditions !== 'unreachable') {
      const tier = taskTier(remaining.yes, conditions, schema)
      if (!tier) return null
      tiers.push(tier)
    }
    remaining = remaining.no
  }
  const fallback = taskTier(remaining, [], schema)
  if (!fallback) return null
  return [...tiers, fallback]
}
