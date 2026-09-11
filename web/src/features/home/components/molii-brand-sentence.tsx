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
  brandName?: string
}

const BRAND_COLORS = [
  {
    color: 'pink',
    gradient: 'from-[#ffb3c7] to-[#f58cad]',
  },
  {
    color: 'blue',
    gradient: 'from-[#62cdf6] to-[#22aee8]',
  },
] as const

function splitGraphemes(value: string): string[] {
  if (typeof Intl.Segmenter === 'function') {
    return [
      ...new Intl.Segmenter(undefined, { granularity: 'grapheme' }).segment(
        value
      ),
    ].map(({ segment }) => segment)
  }

  return Array.from(value)
}

export function MoliiBrandSentence({
  sentence,
  brandName = 'Molii',
}: MoliiBrandSentenceProps) {
  const brandIndex = brandName ? sentence.indexOf(brandName) : -1
  if (brandIndex === -1) {
    return sentence
  }

  const prefix = sentence.slice(0, brandIndex)
  const suffix = sentence.slice(brandIndex + brandName.length)
  const graphemes = splitGraphemes(brandName)

  return (
    <span
      data-home-brand-sentence
      data-home-molii-sentence
      aria-label={sentence}
    >
      <span aria-hidden='true'>
        {prefix}
        <span data-home-brand-word className='inline whitespace-nowrap'>
          {graphemes.map((grapheme, index) => {
            const color = BRAND_COLORS[index % BRAND_COLORS.length]

            return (
              <span
                key={`${index}-${grapheme}`}
                data-home-brand-letter={index}
                data-color={color.color}
                className={`inline-block whitespace-pre bg-gradient-to-b bg-clip-text text-transparent ${color.gradient}`}
              >
                {grapheme}
              </span>
            )
          })}
        </span>
        {suffix}
      </span>
    </span>
  )
}
