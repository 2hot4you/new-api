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
 * Replace only markers whose row key maps to a configured model/vendor icon;
 * summary rows such as Total and Others keep VChart's native marker.
 */
export function decorateChartTooltipIcons(
  tooltipElement: HTMLElement,
  actualTooltip: ChartTooltipActual,
  iconByKey: ReadonlyMap<string, string | undefined>
): void {
  const shapeColumn =
    tooltipElement.querySelector<HTMLElement>('[data-col="shape"]')
  if (!shapeColumn || !actualTooltip.content) return

  const shapeRows = [...shapeColumn.children]
  actualTooltip.content.forEach((item, index) => {
    const key = item.key
    const shapeRow = shapeRows[index]
    if (!key || !shapeRow || !iconByKey.has(key)) return

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
