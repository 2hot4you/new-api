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
import { renderToStaticMarkup } from 'react-dom/server'

import { getLobeIcon } from '@/lib/lobe-icon'

type ChartTooltipActual = {
  content?: Array<{ key?: string }>
}

/**
 * VChart owns the tooltip DOM and recreates its marker column on every update.
 * Replace markers whose rendered row key maps to a configured model/vendor
 * icon; summary rows keep an empty, aligned icon slot.
 */
export function decorateChartTooltipIcons(
  tooltipElement: HTMLElement,
  actualTooltip: ChartTooltipActual,
  iconByKey: ReadonlyMap<string, string | undefined>
): void {
  const shapeColumn =
    tooltipElement.querySelector<HTMLElement>('[data-col="shape"]')
  const content = actualTooltip.content
  if (!shapeColumn || !content) return

  // VChart reserves an 8px marker column. Lobe icons are 16px wide, so they
  // otherwise overflow into the key column and sit against the label.
  shapeColumn.style.width = '20px'
  shapeColumn.style.marginRight = '8px'

  const shapeRows = [...shapeColumn.children]
  const keyRows = [
    ...(tooltipElement.querySelector<HTMLElement>('[data-col="key"]')
      ?.children ?? []),
  ]
  shapeRows.forEach((shapeRow, index) => {
    const renderedKey = keyRows[index]?.textContent?.trim()
    const key = renderedKey || content[index]?.key

    const rowElement = shapeRow as HTMLElement
    rowElement.style.display = 'flex'
    rowElement.style.alignItems = 'center'
    rowElement.style.justifyContent = 'center'

    if (!key || !iconByKey.has(key)) {
      // Transformed dimension tooltips can prepend summary rows (for example
      // Total) that have no entity icon. Keep their column width as alignment
      // space, but remove the misleading native colour marker.
      if (renderedKey) shapeRow.replaceChildren()
      return
    }

    const iconKey = iconByKey.get(key)
    const markup = renderToStaticMarkup(
      <span
        aria-hidden='true'
        data-ranking-tooltip-icon={iconKey ?? ''}
        className='inline-flex size-4 items-center justify-center'
      >
        {getLobeIcon(iconKey, 14)}
      </span>
    )
    const template = document.createElement('template')
    template.innerHTML = markup
    shapeRow.replaceChildren(template.content.cloneNode(true))
  })
}
