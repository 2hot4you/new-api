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
import cnCalendarJson from '../../../../../../pkg/billingexpr/calendars/cn.v1.json'

type CalendarYear = {
  holidays: string[]
  makeupWorkdays: string[]
}

type BillingCalendar = {
  schema: string
  version: number
  country: string
  supportedYears: number[]
  years: Record<string, CalendarYear>
}

const cnCalendar = cnCalendarJson as BillingCalendar
const holidayDates = new Map(
  Object.entries(cnCalendar.years).map(([year, calendar]) => [
    year,
    new Set(calendar.holidays),
  ])
)

function validatedTimezone(zone: string): string | null {
  const timezone = zone.trim() || 'UTC'
  if (timezone === 'Local') return null
  try {
    if (!/^[A-Z][A-Za-z0-9_+-]*(?:\/[A-Z][A-Za-z0-9_+-]*)*$/.test(timezone)) {
      throw new RangeError('timezone')
    }
    const formatter = new Intl.DateTimeFormat('en-US', {
      timeZone: timezone,
    })
    const resolved = formatter.resolvedOptions().timeZone
    if (
      resolved.toLowerCase() === timezone.toLowerCase() &&
      resolved !== timezone
    ) {
      throw new RangeError('timezone')
    }
    return timezone
  } catch {
    return null
  }
}

/** Match the backend's timezone parsing: empty or invalid zones use UTC. */
export function billingDateInZone(now: Date, zone: string): string {
  const timezone = validatedTimezone(zone) ?? 'UTC'
  if (!Number.isFinite(now.getTime())) return ''
  if (timezone === 'UTC') return now.toISOString().slice(0, 10)
  const parts = Object.fromEntries(
    new Intl.DateTimeFormat('en-US', {
      timeZone: timezone,
      calendar: 'gregory',
      numberingSystem: 'latn',
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
    })
      .formatToParts(now)
      .map((part) => [part.type, part.value])
  )
  return `${parts.year}-${parts.month}-${parts.day}`
}

/** True only for dates explicitly listed as statutory rest days. */
export function isHolidayAt(country: string, zone: string, now: Date): boolean {
  if (country.trim().toUpperCase() !== cnCalendar.country) return false
  const date = billingDateInZone(now, zone)
  if (!date) return false
  return holidayDates.get(date.slice(0, 4))?.has(date) ?? false
}

/** Strict validation for UI calendar projection; unlike expression runtime it does not fallback. */
export function isValidBillingTimezone(zone: string): boolean {
  return Boolean(zone.trim() && validatedTimezone(zone))
}
