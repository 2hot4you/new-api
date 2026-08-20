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
import {
  ArrowDown01Icon,
  ArrowUp01Icon,
  Drag01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Reorder, useDragControls } from 'motion/react'
import {
  useEffect,
  useState,
  type KeyboardEvent,
  type PointerEvent,
} from 'react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'

import { getModelOrder, getVendors, saveModelOrder } from '../api'
import { modelsQueryKeys, vendorsQueryKeys } from '../lib'
import type { Model, Vendor } from '../types'

type ModelOrderEditorProps = {
  onSaved: () => void
  onCancel: () => void
  onSavingChange?: (isSaving: boolean) => void
}

type ModelOrderRowProps = {
  item: Model
  index: number
  count: number
  vendorName?: string
  isSaving: boolean
  onMove: (index: number, direction: 'up' | 'down') => void
}

function ModelOrderRow({
  item,
  index,
  count,
  vendorName,
  isSaving,
  onMove,
}: ModelOrderRowProps) {
  const { t } = useTranslation()
  const dragControls = useDragControls()
  const modelName = item.display_name || item.model_name

  const handleDragStart = (event: PointerEvent<HTMLButtonElement>) => {
    if (isSaving) return
    dragControls.start(event)
  }

  const handleDragKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (isSaving) return
    if (event.key === 'ArrowUp') {
      event.preventDefault()
      onMove(index, 'up')
    }
    if (event.key === 'ArrowDown') {
      event.preventDefault()
      onMove(index, 'down')
    }
  }

  return (
    <Reorder.Item
      value={item}
      dragListener={false}
      drag={isSaving ? false : 'y'}
      dragControls={dragControls}
      data-model-order-item
      data-model-id={item.id}
      className='bg-background flex items-center gap-3 rounded-lg border p-3'
    >
      <Button
        type='button'
        variant='ghost'
        size='icon-sm'
        className='text-muted-foreground cursor-grab touch-none active:cursor-grabbing'
        aria-label={t('Drag {{model}} to reorder', { model: item.model_name })}
        disabled={isSaving}
        onPointerDown={handleDragStart}
        onKeyDown={handleDragKeyDown}
      >
        <HugeiconsIcon icon={Drag01Icon} strokeWidth={2} aria-hidden='true' />
      </Button>
      <div className='min-w-0 flex-1'>
        <p className='truncate text-sm font-medium'>{modelName}</p>
        <p className='text-muted-foreground truncate text-xs'>
          {vendorName || t('Unknown vendor')}
        </p>
      </div>
      <span
        className={
          item.status === 1
            ? 'text-success-foreground bg-success/10 rounded-full px-2 py-1 text-xs'
            : 'bg-muted text-muted-foreground rounded-full px-2 py-1 text-xs'
        }
      >
        {item.status === 1 ? t('Enabled') : t('Disabled')}
      </span>
      <div className='flex shrink-0 gap-1'>
        <Button
          type='button'
          variant='ghost'
          size='icon-sm'
          disabled={isSaving || index === 0}
          aria-label={t('Move {{model}} up', { model: item.model_name })}
          onClick={() => onMove(index, 'up')}
        >
          <HugeiconsIcon
            icon={ArrowUp01Icon}
            strokeWidth={2}
            aria-hidden='true'
          />
        </Button>
        <Button
          type='button'
          variant='ghost'
          size='icon-sm'
          disabled={isSaving || index === count - 1}
          aria-label={t('Move {{model}} down', { model: item.model_name })}
          onClick={() => onMove(index, 'down')}
        >
          <HugeiconsIcon
            icon={ArrowDown01Icon}
            strokeWidth={2}
            aria-hidden='true'
          />
        </Button>
      </div>
    </Reorder.Item>
  )
}

export function ModelOrderEditor({
  onSaved,
  onCancel,
  onSavingChange,
}: ModelOrderEditorProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [items, setItems] = useState<Model[]>([])
  const [hasInitializedItems, setHasInitializedItems] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [isSaving, setIsSaving] = useState(false)
  const orderQuery = useQuery({
    queryKey: [...modelsQueryKeys.all, 'order'],
    queryFn: getModelOrder,
  })
  const vendorsQuery = useQuery({
    queryKey: vendorsQueryKeys.list({ page_size: 1000 }),
    queryFn: () => getVendors({ page_size: 1000 }),
  })

  useEffect(() => {
    if (orderQuery.data?.success) {
      setItems(orderQuery.data.data || [])
      setHasInitializedItems(true)
    }
  }, [orderQuery.data])

  useEffect(() => {
    return () => onSavingChange?.(false)
  }, [onSavingChange])

  const vendors = vendorsQuery.data?.data?.items || []
  const vendorNames = new Map(
    vendors.map((vendor: Vendor) => [vendor.id, vendor.name])
  )

  const moveItem = (index: number, direction: 'up' | 'down') => {
    if (isSaving) return
    const targetIndex = direction === 'up' ? index - 1 : index + 1
    if (targetIndex < 0 || targetIndex >= items.length) return

    setItems((current) => {
      const next = [...current]
      ;[next[index], next[targetIndex]] = [next[targetIndex], next[index]]
      return next
    })
  }

  const handleReorder = (nextItems: Model[]) => {
    if (!isSaving) {
      setItems(nextItems)
    }
  }

  const handleCancel = () => {
    if (!isSaving) {
      onCancel()
    }
  }

  const handleSave = async () => {
    if (isSaving) return
    setSaveError(null)
    setIsSaving(true)
    onSavingChange?.(true)
    let didSave = false
    try {
      const response = await saveModelOrder(items.map((item) => item.id))
      if (!response.success) {
        setSaveError(response.message || t('Failed to save model order'))
        return
      }
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: [...modelsQueryKeys.all, 'order'],
        }),
        queryClient.invalidateQueries({ queryKey: modelsQueryKeys.lists() }),
        queryClient.invalidateQueries({ queryKey: ['pricing'] }),
      ])
      didSave = true
    } catch (error) {
      setSaveError(
        error instanceof Error ? error.message : t('Failed to save model order')
      )
    } finally {
      setIsSaving(false)
      onSavingChange?.(false)
    }
    if (didSave) {
      onSaved()
    }
  }

  if (
    orderQuery.isPending ||
    (orderQuery.data?.success && !hasInitializedItems)
  ) {
    return (
      <div
        aria-busy='true'
        className='text-muted-foreground flex min-h-48 items-center justify-center text-sm'
      >
        {t('Loading model order...')}
      </div>
    )
  }

  if (orderQuery.isError || !orderQuery.data?.success) {
    let errorMessage =
      orderQuery.data?.message || t('Failed to load model order')
    if (orderQuery.isError) {
      errorMessage =
        orderQuery.error instanceof Error
          ? orderQuery.error.message
          : t('Failed to load model order')
    }

    return (
      <Alert variant='destructive'>
        <AlertTitle>{t('Unable to load model order')}</AlertTitle>
        <AlertDescription className='space-y-3'>
          <p>{errorMessage}</p>
          <Button type='button' size='sm' onClick={() => orderQuery.refetch()}>
            {t('Retry')}
          </Button>
        </AlertDescription>
      </Alert>
    )
  }

  if (items.length === 0) {
    return (
      <Empty>
        <EmptyHeader>
          <EmptyTitle>{t('No models available to order')}</EmptyTitle>
          <EmptyDescription>
            {t('Create a model before setting the marketplace order.')}
          </EmptyDescription>
        </EmptyHeader>
        <Button type='button' variant='outline' onClick={handleCancel}>
          {t('Cancel')}
        </Button>
      </Empty>
    )
  }

  return (
    <div className='flex h-full min-h-0 flex-col gap-4'>
      <div className='flex items-center justify-between gap-3'>
        <div>
          <h2 className='text-sm font-medium'>{t('Edit model order')}</h2>
          <p className='text-muted-foreground text-sm'>
            {t('Drag models or use the arrow keys to set their display order.')}
          </p>
        </div>
        <div className='flex shrink-0 gap-2'>
          <Button
            type='button'
            variant='outline'
            disabled={isSaving}
            onClick={handleCancel}
          >
            {t('Cancel')}
          </Button>
          <Button type='button' disabled={isSaving} onClick={handleSave}>
            {isSaving ? t('Saving...') : t('Save')}
          </Button>
        </div>
      </div>

      {saveError && (
        <Alert variant='destructive'>
          <AlertTitle>{t('Unable to save model order')}</AlertTitle>
          <AlertDescription>{saveError}</AlertDescription>
        </Alert>
      )}

      <Reorder.Group
        axis='y'
        values={items}
        onReorder={handleReorder}
        data-model-order-list
        className='flex min-h-0 flex-col gap-2 overflow-y-auto'
      >
        {items.map((item, index) => (
          <ModelOrderRow
            key={item.id}
            item={item}
            index={index}
            count={items.length}
            vendorName={vendorNames.get(item.vendor_id || 0)}
            isSaving={isSaving}
            onMove={moveItem}
          />
        ))}
      </Reorder.Group>
    </div>
  )
}
