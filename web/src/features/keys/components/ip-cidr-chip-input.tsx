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
  useEffect,
  useMemo,
  useRef,
  useState,
  type ComponentProps,
  type FocusEventHandler,
  type KeyboardEvent,
  type Ref,
} from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'

type IpCidrChipInputProps = Omit<
  ComponentProps<'div'>,
  'onBlur' | 'onChange' | 'value'
> & {
  value: string
  onChange: (value: string) => void
  resetKey?: string | number
  name?: string
  onBlur?: FocusEventHandler<HTMLInputElement>
  inputRef?: Ref<HTMLInputElement>
  onValidityChange?: (valid: boolean) => void
  onDraftStateChange?: (state: { hasDraft: boolean; isValid: boolean }) => void
}

type IpCidrEntry = {
  key: string
  value: string
  valid: boolean
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

function getEntries(value: string): IpCidrEntry[] {
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

function isDraftValid(draft: string): boolean {
  const entries = draft.split(/[\s,]+/).filter(Boolean)
  return entries.every((entry) => normalizeIpCidr(entry) !== null)
}

export const IpCidrChipInput = forwardRef<HTMLDivElement, IpCidrChipInputProps>(
  function IpCidrChipInput(props, ref) {
    const { t } = useTranslation()
    const {
      className,
      id,
      inputRef,
      name,
      onBlur,
      onChange,
      onDraftStateChange,
      onValidityChange,
      resetKey,
      value,
      ...divProps
    } = props
    const [draft, setDraft] = useState('')
    const [error, setError] = useState<string | null>(null)
    const entries = useMemo(() => getEntries(value), [value])
    const serializedEntries = entries.map((entry) => entry.value).join('\n')
    const hasLegacyInvalidEntries = entries.some((entry) => !entry.valid)
    const callbacksRef = useRef({
      onChange,
      onDraftStateChange,
      onValidityChange,
    })
    const hasLegacyInvalidEntriesRef = useRef(hasLegacyInvalidEntries)
    hasLegacyInvalidEntriesRef.current = hasLegacyInvalidEntries

    useEffect(() => {
      callbacksRef.current = {
        onChange,
        onDraftStateChange,
        onValidityChange,
      }
    }, [onChange, onDraftStateChange, onValidityChange])

    const updateDraft = (nextDraft: string, nextError: string | null) => {
      setDraft(nextDraft)
      setError(nextError)
      const isValid = !hasLegacyInvalidEntries && isDraftValid(nextDraft)
      callbacksRef.current.onValidityChange?.(isValid)
      callbacksRef.current.onDraftStateChange?.({
        hasDraft: nextDraft.trim().length > 0,
        isValid,
      })
    }

    useEffect(() => {
      setDraft('')
      setError(null)
      const isValid = !hasLegacyInvalidEntriesRef.current
      callbacksRef.current.onValidityChange?.(isValid)
      callbacksRef.current.onDraftStateChange?.({
        hasDraft: false,
        isValid,
      })
    }, [resetKey])

    useEffect(() => {
      const isValid = !hasLegacyInvalidEntries && isDraftValid(draft)
      callbacksRef.current.onValidityChange?.(isValid)
      callbacksRef.current.onDraftStateChange?.({
        hasDraft: draft.trim().length > 0,
        isValid,
      })
      if (serializedEntries !== value) {
        callbacksRef.current.onChange(serializedEntries)
      }
    }, [draft, hasLegacyInvalidEntries, serializedEntries, value])

    const addCandidates = (candidates: string[]) => {
      if (candidates.length === 0) {
        updateDraft('', null)
        return
      }

      if (hasLegacyInvalidEntries) {
        updateDraft(
          candidates.join(', '),
          t('Remove invalid saved entries before adding another IP address')
        )
        return
      }

      const normalized = candidates.map(normalizeIpCidr)
      const invalidCandidates = candidates.filter(
        (_, index) => normalized[index] === null
      )
      const nextEntries = entries.map((entry) => entry.value)
      for (const entry of normalized) {
        if (entry && !nextEntries.includes(entry)) nextEntries.push(entry)
      }
      if (nextEntries.length !== entries.length) {
        onChange(nextEntries.join('\n'))
      }
      if (invalidCandidates.length > 0) {
        updateDraft(
          invalidCandidates.join(', '),
          t('Enter a valid IP address or CIDR')
        )
        return
      }
      updateDraft('', null)
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
                key={entry.key}
                data-ip-cidr-chip={entry.value}
                className={cn(
                  'bg-muted flex items-center gap-1 rounded-md px-2 py-1 text-sm',
                  !entry.valid && 'border-destructive text-destructive border'
                )}
              >
                <span>{entry.value}</span>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon-sm'
                  className='size-5'
                  aria-label={t('Remove {{entry}}', { entry: entry.value })}
                  onClick={() => {
                    onChange(
                      entries
                        .filter((candidate) => candidate.key !== entry.key)
                        .map((candidate) => candidate.value)
                        .join('\n')
                    )
                  }}
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
            ref={inputRef}
            name={name}
            value={draft}
            onChange={(event) => {
              const nextDraft = event.target.value
              updateDraft(nextDraft, null)
            }}
            onBlur={onBlur}
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
