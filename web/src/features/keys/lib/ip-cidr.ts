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
import { Address4, Address6 } from 'ip-address'

export type IpCidrEntry = {
  key: string
  value: string
  valid: boolean
}

export function normalizeIpCidr(entry: string): string | null {
  if (!entry || entry.includes('%')) return null

  const segments = entry.split('/')
  if (segments.length > 2 || !segments[0]) return null

  try {
    if (segments[0].includes(':')) {
      const address = new Address6(entry)
      return segments.length === 2
        ? `${address.startAddress().correctForm()}/${address.subnetMask}`
        : address.correctForm()
    }

    const address = new Address4(entry)
    if (address.correctForm() !== segments[0]) return null
    return segments.length === 2
      ? `${address.startAddress().correctForm()}/${address.subnetMask}`
      : address.correctForm()
  } catch {
    return null
  }
}

export function getIpCidrEntries(value: string): IpCidrEntry[] {
  const seen = new Set<string>()
  return value
    .split('\n')
    .map((entry) => entry.trim())
    .filter(Boolean)
    .flatMap<IpCidrEntry>((rawValue, index) => {
      const normalized = normalizeIpCidr(rawValue)
      if (!normalized) {
        return [
          {
            key: `invalid-${index}-${rawValue}`,
            value: rawValue,
            valid: false,
          },
        ]
      }
      if (seen.has(normalized)) return []
      seen.add(normalized)
      return [
        {
          key: normalized,
          value: normalized,
          valid: true,
        },
      ]
    })
}

export function isIpCidrDraftValid(draft: string): boolean {
  const entries = draft.split(/[\s,]+/).filter(Boolean)
  return entries.every((entry) => normalizeIpCidr(entry) !== null)
}

export function isIpCidrValueValid(value: string): boolean {
  return getIpCidrEntries(value).every((entry) => entry.valid)
}
