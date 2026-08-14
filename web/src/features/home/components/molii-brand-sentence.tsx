/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { MoliiWordmark } from '@/components/layout/components/molii-wordmark'

interface MoliiBrandSentenceProps {
  sentence: string
}

export function MoliiBrandSentence({ sentence }: MoliiBrandSentenceProps) {
  const brandIndex = sentence.indexOf('Molii')
  if (brandIndex === -1) {
    return sentence
  }

  const prefix = sentence.slice(0, brandIndex)
  const suffix = sentence.slice(brandIndex + 'Molii'.length)

  return (
    <span data-home-molii-sentence aria-label={sentence}>
      <span aria-hidden='true'>
        {prefix}
        <span data-home-molii-word className='inline whitespace-nowrap'>
          <MoliiWordmark
            alt=''
            aria-hidden='true'
            draggable={false}
            className='mx-[0.08em] inline-block h-[0.76em] translate-y-[0.06em] select-none'
          />
        </span>
        {suffix}
      </span>
    </span>
  )
}
