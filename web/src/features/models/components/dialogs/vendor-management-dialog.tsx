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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Building2,
  Loader2,
  Pencil,
  Plus,
  RefreshCcw,
  Trash2,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Dialog } from '@/components/dialog'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { getLobeIcon } from '@/lib/lobe-icon'

import { deleteVendor, getVendors } from '../../api'
import { modelsQueryKeys, vendorsQueryKeys } from '../../lib'
import type { Vendor } from '../../types'

type VendorManagementDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreateVendor: () => void
  onEditVendor: (vendor: Vendor) => void
}

export function VendorManagementDialog(props: VendorManagementDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [vendorToDelete, setVendorToDelete] = useState<Vendor | null>(null)
  const [isDeleting, setIsDeleting] = useState(false)
  const [deleteError, setDeleteError] = useState('')

  const vendorsQuery = useQuery({
    queryKey: vendorsQueryKeys.list({ page_size: 1000 }),
    queryFn: async () => {
      const response = await getVendors({ page_size: 1000 })
      if (!response.success) {
        throw new Error(response.message || t('Unable to load vendors'))
      }
      return response.data?.items ?? []
    },
    enabled: props.open,
  })

  const handleDeleteOpenChange = (open: boolean) => {
    if (isDeleting) return
    setVendorToDelete(open ? vendorToDelete : null)
    setDeleteError('')
  }

  const handleDelete = async () => {
    if (!vendorToDelete || isDeleting) return
    setIsDeleting(true)
    setDeleteError('')
    try {
      const response = await deleteVendor(vendorToDelete.id)
      if (!response.success) {
        setDeleteError(response.message || t('Failed to delete vendor'))
        return
      }
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: vendorsQueryKeys.lists() }),
        queryClient.invalidateQueries({ queryKey: modelsQueryKeys.lists() }),
      ])
      toast.success(t('Vendor deleted successfully'))
      setVendorToDelete(null)
    } catch (error: unknown) {
      setDeleteError((error as Error)?.message || t('Failed to delete vendor'))
    } finally {
      setIsDeleting(false)
    }
  }

  let content: React.ReactNode
  if (vendorsQuery.isLoading) {
    content = (
      <div
        className='text-muted-foreground flex min-h-56 items-center justify-center gap-2 text-sm'
        aria-busy='true'
      >
        <Loader2 className='size-4 animate-spin' aria-hidden='true' />
        {t('Loading...')}
      </div>
    )
  } else if (vendorsQuery.error) {
    content = (
      <Alert variant='destructive'>
        <AlertTitle>{t('Unable to load vendors')}</AlertTitle>
        <AlertDescription className='space-y-3'>
          <p>{(vendorsQuery.error as Error).message}</p>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => vendorsQuery.refetch()}
          >
            <RefreshCcw className='size-4' aria-hidden='true' />
            {t('Retry')}
          </Button>
        </AlertDescription>
      </Alert>
    )
  } else if (!vendorsQuery.data?.length) {
    content = (
      <Empty className='min-h-56 border'>
        <EmptyHeader>
          <EmptyMedia variant='icon'>
            <Building2 aria-hidden='true' />
          </EmptyMedia>
          <EmptyTitle>{t('No vendors found')}</EmptyTitle>
          <EmptyDescription>
            {t('Add a vendor to start organizing marketplace models.')}
          </EmptyDescription>
        </EmptyHeader>
        <Button type='button' size='sm' onClick={props.onCreateVendor}>
          <Plus className='size-4' aria-hidden='true' />
          {t('Add Vendor')}
        </Button>
      </Empty>
    )
  } else {
    content = (
      <div className='divide-y rounded-lg border'>
        {vendorsQuery.data.map((vendor) => {
          const iconKey = vendor.icon || vendor.name
          return (
            <article
              key={vendor.id}
              className='flex min-w-0 items-start gap-3 p-3 sm:items-center sm:p-4'
            >
              <div
                className='bg-muted/50 flex size-10 shrink-0 items-center justify-center rounded-lg border'
                data-vendor-logo={iconKey}
                aria-label={t('Vendor logo')}
              >
                {getLobeIcon(iconKey, 28)}
              </div>
              <div className='min-w-0 flex-1'>
                <div className='truncate text-sm font-semibold'>
                  {vendor.name}
                </div>
                <code className='text-muted-foreground block truncate text-xs'>
                  {vendor.icon || '—'}
                </code>
                {vendor.description ? (
                  <p className='text-muted-foreground mt-1 line-clamp-2 text-xs'>
                    {vendor.description}
                  </p>
                ) : null}
              </div>
              <div className='flex shrink-0 gap-1'>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon-sm'
                  aria-label={t('Edit vendor {{name}}', { name: vendor.name })}
                  title={t('Edit vendor {{name}}', { name: vendor.name })}
                  onClick={() => props.onEditVendor(vendor)}
                >
                  <Pencil className='size-4' aria-hidden='true' />
                </Button>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon-sm'
                  aria-label={t('Delete vendor {{name}}', {
                    name: vendor.name,
                  })}
                  title={t('Delete vendor {{name}}', { name: vendor.name })}
                  onClick={() => {
                    setDeleteError('')
                    setVendorToDelete(vendor)
                  }}
                >
                  <Trash2
                    className='text-destructive size-4'
                    aria-hidden='true'
                  />
                </Button>
              </div>
            </article>
          )
        })}
      </div>
    )
  }

  return (
    <>
      <Dialog
        open={props.open}
        onOpenChange={props.onOpenChange}
        title={t('Manage Vendors')}
        description={t(
          'Manage vendor names, descriptions, and LobeHub icon variants.'
        )}
        contentHeight='min(34rem, calc(100vh - 14rem))'
        contentClassName='sm:max-w-3xl'
        footer={
          <Button type='button' onClick={props.onCreateVendor}>
            <Plus className='size-4' aria-hidden='true' />
            {t('Add Vendor')}
          </Button>
        }
      >
        {content}
      </Dialog>

      <ConfirmDialog
        open={Boolean(vendorToDelete)}
        onOpenChange={handleDeleteOpenChange}
        title={t('Delete Vendor')}
        desc={t(
          'Are you sure you want to delete vendor "{{name}}"? This action cannot be undone.',
          { name: vendorToDelete?.name ?? '' }
        )}
        destructive
        confirmText={isDeleting ? t('Deleting...') : t('Delete')}
        isLoading={isDeleting}
        handleConfirm={handleDelete}
      >
        {deleteError ? (
          <Alert variant='destructive'>
            <AlertTitle>{t('Operation failed')}</AlertTitle>
            <AlertDescription>{deleteError}</AlertDescription>
          </Alert>
        ) : null}
      </ConfirmDialog>
    </>
  )
}
