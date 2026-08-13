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
import type { PricingModel } from '../types'
import { sortModelsByReleaseDate } from './model-directory'

function isSameVendor(left: PricingModel, right: PricingModel): boolean {
  if (left.vendor_id != null && right.vendor_id != null) {
    return left.vendor_id === right.vendor_id
  }
  return Boolean(
    left.vendor_name &&
    right.vendor_name &&
    left.vendor_name === right.vendor_name
  )
}

export function getRelatedModels(
  currentModel: PricingModel,
  models: PricingModel[],
  limit = 6
): PricingModel[] {
  return sortModelsByReleaseDate(
    models.filter(
      (model) =>
        model.model_name !== currentModel.model_name &&
        isSameVendor(currentModel, model)
    )
  ).slice(0, limit)
}
