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
import { useEffect, useState } from 'react'

import type { Vendor } from '../../types'
import { VendorManagementDialog } from './vendor-management-dialog'
import { VendorMutateDialog } from './vendor-mutate-dialog'

type VendorManagementProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

type VendorManagementView = 'list' | 'mutate'

export function VendorManagement(props: VendorManagementProps) {
  const [view, setView] = useState<VendorManagementView>('list')
  const [currentVendor, setCurrentVendor] = useState<Vendor | null>(null)

  useEffect(() => {
    if (!props.open) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setView('list')
      setCurrentVendor(null)
    }
  }, [props.open])

  const returnToList = () => {
    setView('list')
    setCurrentVendor(null)
  }

  const showForm = (vendor: Vendor | null) => {
    setCurrentVendor(vendor)
    setView('mutate')
  }

  const handleListOpenChange = (open: boolean) => {
    if (!open) {
      returnToList()
      props.onOpenChange(false)
    }
  }

  return (
    <>
      <VendorManagementDialog
        open={props.open && view === 'list'}
        onOpenChange={handleListOpenChange}
        onCreateVendor={() => showForm(null)}
        onEditVendor={showForm}
      />
      <VendorMutateDialog
        open={props.open && view === 'mutate'}
        onOpenChange={(open) => !open && returnToList()}
        currentVendor={currentVendor}
      />
    </>
  )
}
