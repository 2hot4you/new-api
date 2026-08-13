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
import type { PricingModel } from '../types'

export function groupModelsByVendor(models: PricingModel[]): PricingModel[][] {
  const groups = new Map<string, PricingModel[]>()

  models.forEach((model, index) => {
    const vendorName = model.vendor_name?.trim()
    let key: string
    if (model.vendor_id != null) {
      key = `id:${model.vendor_id}`
    } else if (vendorName) {
      key = `name:${vendorName}`
    } else {
      key = `unknown:${model.id ?? model.model_name ?? 'model'}:${index}`
    }
    const group = groups.get(key)

    if (group) {
      group.push(model)
    } else {
      groups.set(key, [model])
    }
  })

  return [...groups.values()]
}
