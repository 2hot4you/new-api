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
import { DescriptionDialog } from './dialogs/description-dialog'
import { MissingModelsDialog } from './dialogs/missing-models-dialog'
import { PrefillGroupManagement } from './dialogs/prefill-group-management'
import { VendorManagement } from './dialogs/vendor-management'
import { VendorMutateDialog } from './dialogs/vendor-mutate-dialog'
import { ModelMutateDrawer } from './drawers/model-mutate-drawer'
import { useModels } from './models-provider'

export function ModelsDialogs() {
  const {
    open,
    setOpen,
    currentRow,
    currentVendor,
    descriptionData,
    setDescriptionData,
  } = useModels()

  return (
    <>
      {/* Model Create/Update Drawer */}
      <ModelMutateDrawer
        open={
          open === 'create-model' ||
          open === 'update-model' ||
          open === 'price-model'
        }
        initialSection={open === 'price-model' ? 'pricing' : 'metadata'}
        onOpenChange={(v) => !v && setOpen(null)}
        currentRow={currentRow}
      />

      {/* Vendor Management */}
      <VendorManagement
        open={open === 'manage-vendors'}
        onOpenChange={(v) => !v && setOpen(null)}
      />

      <VendorMutateDialog
        key={`${open}-${currentVendor?.id ?? 'new'}`}
        open={open === 'create-vendor' || open === 'update-vendor'}
        onOpenChange={(value) => !value && setOpen(null)}
        currentVendor={currentVendor}
      />

      {/* Missing Models Dialog */}
      <MissingModelsDialog
        open={open === 'missing-models'}
        onOpenChange={(v) => !v && setOpen(null)}
      />

      {/* Prefill Groups Management */}
      <PrefillGroupManagement
        open={open === 'prefill-groups'}
        onOpenChange={(v) => !v && setOpen(null)}
      />

      {/* Description Dialog */}
      <DescriptionDialog
        open={open === 'description'}
        onOpenChange={(v) => {
          if (!v) {
            setOpen(null)
            setDescriptionData(null)
          }
        }}
        modelName={descriptionData?.modelName || ''}
        description={descriptionData?.description || ''}
      />
    </>
  )
}
