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
import {
  forwardRef,
  useMemo,
  useState,
  type ComponentProps,
  type KeyboardEvent,
} from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'

type IpCidrChipInputProps = Omit<
  ComponentProps<'div'>,
  'onChange' | 'value'
> & {
  value: string
  onChange: (value: string) => void
  onValidityChange?: (valid: boolean) => void
}

function normalizeIpCidr(entry: string): string | null {
  if (!entry || entry.includes('%')) return null

  const segments = entry.split('/')
  if (segments.length > 2 || !segments[0]) return null

  try {
    if (segments[0].includes(':')) {
      const address = new Address6(entry)
      return segments.length === 2
        ? `${address.correctForm()}/${address.subnetMask}`
        : address.correctForm()
    }

    const address = new Address4(entry)
    if (address.correctForm() !== segments[0]) return null
    return segments.length === 2
      ? `${address.correctForm()}/${address.subnetMask}`
      : address.correctForm()
  } catch {
    return null
  }
}

function getEntries(value: string): string[] {
  return value
    .split('\n')
    .map((entry) => entry.trim())
    .filter(Boolean)
}

function isDraftValid(draft: string): boolean {
  const entries = draft.split(/[\s,]+/).filter(Boolean)
  return entries.every((entry) => normalizeIpCidr(entry) !== null)
}

export const IpCidrChipInput = forwardRef<HTMLDivElement, IpCidrChipInputProps>(
  function IpCidrChipInput(props, ref) {
    const { t } = useTranslation()
    const { className, id, onChange, onValidityChange, value, ...divProps } =
      props
    const [draft, setDraft] = useState('')
    const [error, setError] = useState<string | null>(null)
    const entries = useMemo(() => getEntries(value), [value])

    const addCandidates = (candidates: string[]) => {
      if (candidates.length === 0) {
        setError(null)
        onValidityChange?.(true)
        return
      }

      const normalized = candidates.map(normalizeIpCidr)
      if (normalized.some((entry) => entry === null)) {
        setError(t('Enter a valid IP address or CIDR'))
        onValidityChange?.(false)
        return
      }

      const nextEntries = [...entries]
      for (const entry of normalized) {
        if (entry && !nextEntries.includes(entry)) nextEntries.push(entry)
      }
      setDraft('')
      setError(null)
      onValidityChange?.(true)
      if (nextEntries.length !== entries.length) {
        onChange(nextEntries.join('\n'))
      }
    }

    const addDraft = () => addCandidates(draft.split(/[\s,]+/).filter(Boolean))

    const handleKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
      if (event.key !== 'Enter') return
      event.preventDefault()
      addDraft()
    }

    return (
      <div
        {...divProps}
        ref={ref}
        className={cn('flex w-full flex-col gap-2', className)}
        role='group'
      >
        {entries.length > 0 && (
          <div className='flex flex-wrap gap-2'>
            {entries.map((entry) => (
              <span
                key={entry}
                data-ip-cidr-chip={entry}
                className='bg-muted flex items-center gap-1 rounded-md px-2 py-1 text-sm'
              >
                <span>{entry}</span>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon-sm'
                  className='size-5'
                  aria-label={t('Remove {{entry}}', { entry })}
                  onClick={() =>
                    onChange(
                      entries
                        .filter((candidate) => candidate !== entry)
                        .join('\n')
                    )
                  }
                >
                  <span aria-hidden='true'>×</span>
                </Button>
              </span>
            ))}
          </div>
        )}
        <div className='flex gap-2'>
          <Input
            id={id}
            value={draft}
            onChange={(event) => {
              const nextDraft = event.target.value
              setDraft(nextDraft)
              setError(null)
              onValidityChange?.(isDraftValid(nextDraft))
            }}
            onKeyDown={handleKeyDown}
            onPaste={(event) => {
              const pasted = event.clipboardData.getData('text')
              if (!/[\s,]/.test(pasted)) return
              event.preventDefault()
              addCandidates(pasted.split(/[\s,]+/).filter(Boolean))
            }}
            placeholder={t('Enter an IP address or CIDR')}
            aria-label={t('IP address or CIDR')}
            aria-describedby={divProps['aria-describedby']}
            aria-invalid={error ? true : divProps['aria-invalid']}
          />
          <Button type='button' variant='outline' onClick={addDraft}>
            {t('Add IP address')}
          </Button>
        </div>
        {error && (
          <p role='alert' className='text-destructive text-sm'>
            {error}
          </p>
        )}
      </div>
    )
  }
)
