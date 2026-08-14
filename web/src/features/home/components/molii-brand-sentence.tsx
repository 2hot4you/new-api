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
interface MoliiBrandSentenceProps {
  sentence: string
}

const MOLII_LETTERS = [
  {
    id: 'm',
    value: 'M',
    color: 'pink',
    gradient: 'from-[#ffb3c7] to-[#f58cad]',
  },
  {
    id: 'o',
    value: 'o',
    color: 'blue',
    gradient: 'from-[#62cdf6] to-[#22aee8]',
  },
  {
    id: 'l',
    value: 'l',
    color: 'pink',
    gradient: 'from-[#ffb3c7] to-[#f58cad]',
  },
  {
    id: 'i-first',
    value: 'i',
    color: 'blue',
    gradient: 'from-[#62cdf6] to-[#22aee8]',
  },
  {
    id: 'i-second',
    value: 'i',
    color: 'pink',
    gradient: 'from-[#ffb3c7] to-[#f58cad]',
  },
] as const

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
          {MOLII_LETTERS.map((letter) => (
            <span
              key={letter.id}
              data-home-molii-letter={letter.id}
              data-color={letter.color}
              className={`inline-block bg-gradient-to-b bg-clip-text text-transparent ${letter.gradient}`}
            >
              {letter.value}
            </span>
          ))}
        </span>
        {suffix}
      </span>
    </span>
  )
}
