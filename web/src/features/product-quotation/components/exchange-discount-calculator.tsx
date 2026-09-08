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
import { Calculator, Check, Copy } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'

import {
  calculateSuggestedGroupRatio,
  normalizeDiscount,
} from '../lib/quotation-math'

type ExchangeDiscountCalculatorProps = {
  defaultActualRate?: number
  defaultFixedRate?: number
}

function finiteNumber(value: string): number {
  if (!value.trim()) return Number.NaN
  return Number(value)
}

export function ExchangeDiscountCalculator({
  defaultActualRate = 7,
  defaultFixedRate = 7,
}: ExchangeDiscountCalculatorProps) {
  const { t } = useTranslation()
  const [actualRate, setActualRate] = useState(String(defaultActualRate))
  const [fixedRate, setFixedRate] = useState(String(defaultFixedRate))
  const [discount, setDiscount] = useState('10')
  const { copiedText, copyToClipboard } = useCopyToClipboard({ notify: false })
  const parsedDiscount = finiteNumber(discount)
  const ratio = useMemo(
    () =>
      calculateSuggestedGroupRatio(
        finiteNumber(actualRate),
        parsedDiscount,
        finiteNumber(fixedRate)
      ),
    [actualRate, fixedRate, parsedDiscount]
  )
  const ratioText = ratio === null ? null : String(ratio)
  const coefficient = normalizeDiscount(parsedDiscount)
  const copied = ratioText !== null && copiedText === ratioText

  return (
    <Card>
      <CardHeader>
        <CardTitle className='flex items-center gap-2'>
          <Calculator className='text-primary size-4' />
          {t('Exchange rate and discount calculator')}
        </CardTitle>
        <CardDescription>
          {t(
            'Calculate a suggested group ratio without changing system settings.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className='space-y-4'>
        <div className='grid gap-3 sm:grid-cols-3'>
          <div className='space-y-1.5'>
            <Label htmlFor='quotation-actual-rate'>
              {t('Actual exchange rate')}
            </Label>
            <Input
              id='quotation-actual-rate'
              type='number'
              inputMode='decimal'
              min='0'
              step='any'
              value={actualRate}
              onChange={(event) => setActualRate(event.target.value)}
              aria-invalid={finiteNumber(actualRate) <= 0}
            />
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='quotation-calculator-discount'>
              {t('Calculator discount (zhe)')}
            </Label>
            <Input
              id='quotation-calculator-discount'
              type='number'
              inputMode='decimal'
              min='0'
              max='10'
              step='any'
              value={discount}
              onChange={(event) => setDiscount(event.target.value)}
              aria-invalid={coefficient === null}
            />
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='quotation-fixed-rate'>
              {t('Fixed system exchange rate')}
            </Label>
            <Input
              id='quotation-fixed-rate'
              type='number'
              inputMode='decimal'
              min='0'
              step='any'
              value={fixedRate}
              onChange={(event) => setFixedRate(event.target.value)}
              aria-invalid={finiteNumber(fixedRate) <= 0}
            />
          </div>
        </div>

        <div className='bg-muted/50 flex flex-wrap items-center justify-between gap-3 rounded-lg border px-3 py-3'>
          <div className='min-w-0'>
            <p className='text-muted-foreground text-xs'>
              {t('Effective coefficient')}
            </p>
            <p className='font-mono text-sm tabular-nums'>
              {coefficient === null ? '—' : coefficient}
            </p>
          </div>
          <div className='min-w-0 flex-1 text-right'>
            <p className='text-muted-foreground text-xs'>
              {t('Suggested group ratio')}
            </p>
            <p className='font-mono text-lg font-semibold break-all tabular-nums'>
              {ratioText === null ? '—' : `${ratioText}x`}
            </p>
          </div>
          <Button
            type='button'
            variant='outline'
            size='sm'
            disabled={ratioText === null}
            onClick={() => ratioText && void copyToClipboard(ratioText)}
            aria-label={t('Copy group ratio')}
          >
            {copied ? <Check /> : <Copy />}
            {copied ? t('Copied') : t('Copy')}
          </Button>
        </div>
        {ratioText === null ? (
          <p role='alert' className='text-destructive text-xs'>
            {t('Enter finite positive rates and a discount from 0 to 10.')}
          </p>
        ) : (
          <p className='text-muted-foreground text-xs'>
            {t(
              'Formula: actual rate × discount coefficient ÷ fixed system rate.'
            )}
          </p>
        )}
        {copied && (
          <p role='status' className='text-success text-xs'>
            {t('Group ratio copied')}
          </p>
        )}
      </CardContent>
    </Card>
  )
}
