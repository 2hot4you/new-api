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
  ArrowDown,
  ArrowUp,
  Building2,
  GripVertical,
  Loader2,
  Pencil,
  Plus,
  RefreshCcw,
  Trash2,
} from 'lucide-react'
import { Reorder, useDragControls } from 'motion/react'
import {
  useEffect,
  useRef,
  useState,
  type KeyboardEvent,
  type PointerEvent,
} from 'react'
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

import {
  deleteVendor,
  getVendorOrder,
  getVendors,
  saveVendorOrder,
} from '../../api'
import { modelsQueryKeys, vendorsQueryKeys } from '../../lib'
import type { Vendor } from '../../types'

type VendorManagementDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreateVendor: () => void
  onEditVendor: (vendor: Vendor) => void
}

type VendorManagementMode = 'list' | 'order'

type VendorOrderItemProps = {
  vendor: Vendor
  index: number
  count: number
  disabled: boolean
  onMove: (index: number, direction: 'up' | 'down') => void
}

function VendorOrderItem(props: VendorOrderItemProps) {
  const { t } = useTranslation()
  const dragControls = useDragControls()
  const iconKey = props.vendor.icon || props.vendor.name

  const handleDragStart = (event: PointerEvent<HTMLButtonElement>) => {
    if (props.disabled) return
    dragControls.start(event)
  }

  const handleDragKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (props.disabled) return
    if (event.key === 'ArrowUp') {
      event.preventDefault()
      props.onMove(props.index, 'up')
    }
    if (event.key === 'ArrowDown') {
      event.preventDefault()
      props.onMove(props.index, 'down')
    }
  }

  return (
    <Reorder.Item
      value={props.vendor}
      dragListener={false}
      dragControls={dragControls}
      drag={props.disabled ? false : 'y'}
      className='bg-background flex min-w-0 items-start gap-2 rounded-lg border p-2 sm:items-center sm:gap-3 sm:p-4'
    >
      <Button
        type='button'
        variant='ghost'
        size='icon-sm'
        disabled={props.disabled}
        className='text-muted-foreground cursor-grab touch-none active:cursor-grabbing'
        aria-label={t('Drag {{name}} to reorder', { name: props.vendor.name })}
        onPointerDown={handleDragStart}
        onKeyDown={handleDragKeyDown}
      >
        <GripVertical className='size-4' aria-hidden='true' />
      </Button>
      <div
        className='bg-muted/50 flex size-9 shrink-0 items-center justify-center rounded-lg border sm:size-10'
        data-vendor-logo={iconKey}
        aria-label={t('Vendor logo')}
      >
        {getLobeIcon(iconKey, 28)}
      </div>
      <div className='min-w-0 flex-1'>
        <div data-vendor-name className='truncate text-sm font-semibold'>
          {props.vendor.name}
        </div>
        <code className='text-muted-foreground hidden truncate text-xs sm:block'>
          {props.vendor.icon || '—'}
        </code>
      </div>
      <div className='flex shrink-0'>
        <Button
          type='button'
          variant='ghost'
          size='icon-sm'
          disabled={props.disabled || props.index === 0}
          aria-label={t('Move {{name}} up', { name: props.vendor.name })}
          onClick={() => props.onMove(props.index, 'up')}
        >
          <ArrowUp className='size-4' aria-hidden='true' />
        </Button>
        <Button
          type='button'
          variant='ghost'
          size='icon-sm'
          disabled={props.disabled || props.index === props.count - 1}
          aria-label={t('Move {{name}} down', { name: props.vendor.name })}
          onClick={() => props.onMove(props.index, 'down')}
        >
          <ArrowDown className='size-4' aria-hidden='true' />
        </Button>
      </div>
    </Reorder.Item>
  )
}

export function VendorManagementDialog(props: VendorManagementDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [vendorToDelete, setVendorToDelete] = useState<Vendor | null>(null)
  const [isDeleting, setIsDeleting] = useState(false)
  const [deleteError, setDeleteError] = useState('')
  const [mode, setMode] = useState<VendorManagementMode>('list')
  const [orderDraft, setOrderDraft] = useState<Vendor[]>([])
  const [isLoadingOrder, setIsLoadingOrder] = useState(false)
  const [isSavingOrder, setIsSavingOrder] = useState(false)
  const [orderError, setOrderError] = useState('')
  const orderRequestGeneration = useRef(0)

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

  const discardOrder = () => {
    orderRequestGeneration.current += 1
    setMode('list')
    setOrderDraft([])
    setIsLoadingOrder(false)
    setOrderError('')
  }

  useEffect(() => {
    if (!props.open) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      discardOrder()
    }
  }, [props.open])

  const loadVendorOrder = async () => {
    const generation = orderRequestGeneration.current + 1
    orderRequestGeneration.current = generation
    setIsLoadingOrder(true)
    setOrderDraft([])
    setOrderError('')
    try {
      const response = await getVendorOrder()
      if (generation !== orderRequestGeneration.current) return
      if (!response.success) {
        setOrderError(
          response.message
            ? t(response.message)
            : t('Unable to load vendor order')
        )
        return
      }
      setOrderDraft(response.data ?? [])
    } catch (error: unknown) {
      if (generation !== orderRequestGeneration.current) return
      setOrderError(
        (error as Error)?.message || t('Unable to load vendor order')
      )
    } finally {
      if (generation === orderRequestGeneration.current) {
        setIsLoadingOrder(false)
      }
    }
  }

  const handleEnterOrderMode = () => {
    if (!vendorsQuery.data?.length || isLoadingOrder) return
    setMode('order')
    void loadVendorOrder()
  }

  const handleMoveOrderItem = (index: number, direction: 'up' | 'down') => {
    if (isSavingOrder || isLoadingOrder) return
    const targetIndex = direction === 'up' ? index - 1 : index + 1
    if (targetIndex < 0 || targetIndex >= orderDraft.length) return
    setOrderDraft((current) => {
      const next = [...current]
      ;[next[index], next[targetIndex]] = [next[targetIndex], next[index]]
      return next
    })
  }

  const handleSaveOrder = async () => {
    if (isSavingOrder) return
    setIsSavingOrder(true)
    setOrderError('')
    try {
      const response = await saveVendorOrder(
        orderDraft.map((vendor) => vendor.id)
      )
      if (!response.success) {
        setOrderError(
          response.message
            ? t(response.message)
            : t('Failed to save vendor order')
        )
        return
      }
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: vendorsQueryKeys.lists() }),
        queryClient.invalidateQueries({ queryKey: modelsQueryKeys.lists() }),
        queryClient.invalidateQueries({ queryKey: ['pricing'] }),
      ])
      toast.success(t('Vendor order saved successfully'))
      discardOrder()
    } catch (error: unknown) {
      setOrderError(
        (error as Error)?.message || t('Failed to save vendor order')
      )
    } finally {
      setIsSavingOrder(false)
    }
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
  } else if (mode === 'order') {
    content = (
      <div className='space-y-3'>
        {isLoadingOrder ? (
          <div
            className='text-muted-foreground flex min-h-56 items-center justify-center gap-2 text-sm'
            aria-busy='true'
          >
            <Loader2 className='size-4 animate-spin' aria-hidden='true' />
            {t('Loading...')}
          </div>
        ) : (
          <>
            {orderError ? (
              <Alert variant='destructive'>
                <AlertTitle>{t('Operation failed')}</AlertTitle>
                <AlertDescription className='space-y-3'>
                  <p>{orderError}</p>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={() => void loadVendorOrder()}
                  >
                    <RefreshCcw className='size-4' aria-hidden='true' />
                    {t('Retry')}
                  </Button>
                </AlertDescription>
              </Alert>
            ) : null}
            <Reorder.Group
              axis='y'
              values={orderDraft}
              onReorder={(vendors) => {
                if (!isSavingOrder) setOrderDraft(vendors)
              }}
              className='flex flex-col gap-2'
            >
              {orderDraft.map((vendor, index) => (
                <VendorOrderItem
                  key={vendor.id}
                  vendor={vendor}
                  index={index}
                  count={orderDraft.length}
                  disabled={isSavingOrder}
                  onMove={handleMoveOrderItem}
                />
              ))}
            </Reorder.Group>
          </>
        )}
      </div>
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
                <div
                  data-vendor-name
                  className='truncate text-sm font-semibold'
                >
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
        onOpenChange={(open) => {
          if (!open && isSavingOrder) return
          if (!open) discardOrder()
          props.onOpenChange(open)
        }}
        title={t('Manage Vendors')}
        description={t(
          'Manage vendor names, descriptions, and LobeHub icon variants.'
        )}
        contentHeight='min(34rem, calc(100vh - 14rem))'
        contentClassName='sm:max-w-3xl'
        footer={
          mode === 'order' ? (
            <>
              <Button
                type='button'
                variant='outline'
                disabled={isSavingOrder}
                onClick={discardOrder}
              >
                {t('Cancel')}
              </Button>
              <Button
                type='button'
                disabled={isSavingOrder || isLoadingOrder || !orderDraft.length}
                onClick={handleSaveOrder}
              >
                {isSavingOrder ? t('Saving...') : t('Save order')}
              </Button>
              <Button type='button' disabled>
                <Plus className='size-4' aria-hidden='true' />
                {t('Add Vendor')}
              </Button>
            </>
          ) : (
            <>
              <Button
                type='button'
                variant='outline'
                disabled={!vendorsQuery.data?.length}
                onClick={handleEnterOrderMode}
              >
                <GripVertical className='size-4' aria-hidden='true' />
                {t('Reorder Vendors')}
              </Button>
              <Button type='button' onClick={props.onCreateVendor}>
                <Plus className='size-4' aria-hidden='true' />
                {t('Add Vendor')}
              </Button>
            </>
          )
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
