/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import {
  CheckSquare2,
  Copy,
  ExternalLink,
  Image,
  Music,
  RefreshCw,
  Trash2,
  Video,
} from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { SectionPageLayout } from '@/components/layout'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { CompactDateTimeRangePicker } from '@/features/usage-logs/components/compact-date-time-range-picker'
import { api } from '@/lib/api'
import dayjs from '@/lib/dayjs'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import {
  AssetTypeFilter,
  type AssetTypeFilterValue,
} from './components/asset-type-filter'
import {
  CreateAssetCard,
  type COSUploadConfig,
} from './components/create-asset-card'
import {
  refreshTemporaryAsset,
  replaceTemporaryAsset,
} from './lib/asset-actions'
import {
  type AssetType,
  type TemporaryAsset,
  TEMPORARY_ASSET_GRID_CLASS_NAME,
  filterTemporaryAssets,
  getAssetStatusLabel,
  getAssetStatusVariant,
  getAssetTypeLabel,
  getPendingAssetIDs,
  toggleTemporaryAssetSelection,
  toggleVisibleTemporaryAssetSelection,
} from './lib/asset-utils'

type AssetStats = {
  total: number
  processing: number
  success: number
  failed: number
  expired: number
  expiring_soon: number
  users: number
  by_type: Record<AssetType, number>
}

function getAssetErrorMessage(error: unknown): string | undefined {
  if (!error || typeof error !== 'object') return undefined
  const response = 'response' in error ? error.response : undefined
  if (!response || typeof response !== 'object') return undefined
  const data = 'data' in response ? response.data : undefined
  if (!data || typeof data !== 'object') return undefined
  const message = 'message' in data ? data.message : undefined
  return typeof message === 'string' ? message : undefined
}

const assetIcon = { image: Image, video: Video, audio: Music }

function AssetPreview(props: {
  item: TemporaryAsset
  alt: string
  missingLabel: string
  failedLabel: string
}) {
  const [failed, setFailed] = useState(false)
  const Icon = assetIcon[props.item.asset_type] ?? Image
  const previewURL = props.item.preview_url || props.item.source_url
  if (!previewURL) {
    return (
      <div className='text-muted-foreground flex flex-col items-center gap-2 px-3 text-center text-xs'>
        <Icon className='size-5' />
        <span>{props.missingLabel}</span>
      </div>
    )
  }
  if (failed) {
    return (
      <div className='text-muted-foreground flex flex-col items-center gap-2 px-3 text-center text-xs'>
        <Icon className='size-5' />
        <span>{props.failedLabel}</span>
      </div>
    )
  }
  if (props.item.asset_type === 'image') {
    return (
      <img
        src={previewURL}
        alt={props.alt}
        className='size-full object-cover'
        loading='lazy'
        referrerPolicy='no-referrer'
        onError={() => setFailed(true)}
      />
    )
  }
  if (props.item.asset_type === 'video') {
    return (
      <video
        src={previewURL}
        className='size-full object-cover'
        controls
        preload='metadata'
        onError={() => setFailed(true)}
      />
    )
  }
  return <Music className='size-5' />
}

export function TemporaryAssets() {
  const { t } = useTranslation()
  const [items, setItems] = useState<TemporaryAsset[]>([])
  const [selectedIDs, setSelectedIDs] = useState<Set<string>>(new Set())
  const [deleteTargetIDs, setDeleteTargetIDs] = useState<string[]>([])
  const [deleting, setDeleting] = useState(false)
  const [loading, setLoading] = useState(true)
  const [uploadConfig, setUploadConfig] = useState<COSUploadConfig>({
    enabled: false,
    limits: {
      image: 30 * 1024 * 1024,
      video: 50 * 1024 * 1024,
      audio: 15 * 1024 * 1024,
    },
  })
  const [stats, setStats] = useState<AssetStats | null>(null)
  const [filterType, setFilterType] = useState<AssetTypeFilterValue>('all')
  const [filterID, setFilterID] = useState('')
  const [filterStart, setFilterStart] = useState<Date | undefined>()
  const [filterEnd, setFilterEnd] = useState<Date | undefined>()
  const [sourceURLDrafts, setSourceURLDrafts] = useState<
    Record<string, string>
  >({})
  const [savingSourceURLID, setSavingSourceURLID] = useState('')
  const [refreshingIDs, setRefreshingIDs] = useState<Set<string>>(new Set())
  const isAdmin = useAuthStore(
    (state) => (state.auth.user?.role ?? 0) >= ROLE.ADMIN
  )

  const listEndpoint = isAdmin ? '/api/assets/admin' : '/api/assets/self'

  const load = useCallback(
    async (silent = false) => {
      if (!silent) setLoading(true)
      try {
        const response = await api.get(listEndpoint)
        const nextItems = (response.data?.data ?? []) as TemporaryAsset[]
        setItems(nextItems)
        const availableIDs = new Set(nextItems.map((item) => item.id))
        setSelectedIDs(
          (current) =>
            new Set([...current].filter((id) => availableIDs.has(id)))
        )
      } catch {
        if (!silent) toast.error(t('Failed to load temporary assets'))
      } finally {
        if (!silent) setLoading(false)
      }
    },
    [listEndpoint, t]
  )

  useEffect(() => {
    void load(false)
  }, [load])

  useEffect(() => {
    void api
      .get('/api/assets/self/upload-config')
      .then((response) => {
        if (response.data?.data) setUploadConfig(response.data.data)
      })
      .catch(() =>
        setUploadConfig((current) => ({ ...current, enabled: false }))
      )
  }, [])

  useEffect(() => {
    if (!isAdmin) return
    void api
      .get('/api/assets/admin/stats')
      .then((response) => setStats(response.data?.data ?? null))
      .catch(() => setStats(null))
  }, [isAdmin, items])

  const pendingIDs = useMemo(() => getPendingAssetIDs(items), [items])
  const pendingKey = pendingIDs.join(',')

  useEffect(() => {
    if (!pendingKey) return
    const endpoint = isAdmin ? '/api/assets/admin' : '/api/assets/self'
    const timer = window.setInterval(() => {
      void Promise.allSettled(
        pendingIDs.map((id) => api.get(`${endpoint}/${id}`))
      ).then(() => load(true))
    }, 10_000)
    return () => window.clearInterval(timer)
  }, [isAdmin, load, pendingIDs, pendingKey])

  const filteredItems = useMemo(() => {
    const start = filterStart ? Math.floor(filterStart.getTime() / 1000) : 0
    const end = filterEnd ? Math.floor(filterEnd.getTime() / 1000) : 0
    return filterTemporaryAssets(items, {
      assetType: filterType,
      assetID: filterID,
      startTimestamp: start,
      endTimestamp: end,
    })
  }, [filterEnd, filterID, filterStart, filterType, items])

  const visibleIDs = useMemo(
    () => filteredItems.map((item) => item.id),
    [filteredItems]
  )
  const allVisibleSelected =
    visibleIDs.length > 0 && visibleIDs.every((id) => selectedIDs.has(id))

  const refresh = async (id: string) => {
    if (refreshingIDs.has(id)) return
    setRefreshingIDs((current) => new Set(current).add(id))
    try {
      const refreshedAsset = await refreshTemporaryAsset(
        (url) => api.get(url),
        isAdmin,
        id
      )
      setItems((current) => replaceTemporaryAsset(current, refreshedAsset))
      toast.success(t('Asset status refreshed'))
    } catch (error) {
      toast.error(
        getAssetErrorMessage(error) || t('Failed to refresh asset status')
      )
    } finally {
      setRefreshingIDs((current) => {
        const next = new Set(current)
        next.delete(id)
        return next
      })
    }
  }

  const confirmDelete = async () => {
    if (deleteTargetIDs.length === 0) return
    setDeleting(true)
    const endpoint = isAdmin ? '/api/assets/admin' : '/api/assets/self'
    const results = await Promise.allSettled(
      deleteTargetIDs.map((id) => api.delete(`${endpoint}/${id}`))
    )
    const deletedIDs = new Set(
      deleteTargetIDs.filter(
        (_, index) => results[index]?.status === 'fulfilled'
      )
    )
    const failedCount = deleteTargetIDs.length - deletedIDs.size
    if (deletedIDs.size > 0) {
      setItems((current) => current.filter((item) => !deletedIDs.has(item.id)))
      setSelectedIDs(
        (current) => new Set([...current].filter((id) => !deletedIDs.has(id)))
      )
      toast.success(
        t('Successfully deleted {{count}} temporary assets', {
          count: deletedIDs.size,
        })
      )
    }
    if (failedCount > 0) {
      toast.error(
        t('Failed to delete {{count}} temporary assets', {
          count: failedCount,
        })
      )
    }
    setDeleting(false)
    setDeleteTargetIDs([])
  }

  const saveSourceURL = async (id: string) => {
    const sourceURL = (sourceURLDrafts[id] ?? '').trim()
    if (!sourceURL) return
    setSavingSourceURLID(id)
    try {
      const response = await api.patch(`${listEndpoint}/${id}/source-url`, {
        url: sourceURL,
      })
      const saved = response.data?.data as TemporaryAsset | undefined
      if (saved) {
        setItems((current) =>
          current.map((item) => (item.id === id ? saved : item))
        )
      } else {
        await load(true)
      }
      setSourceURLDrafts((current) => {
        const next = { ...current }
        delete next[id]
        return next
      })
      toast.success(t('Source URL saved'))
    } catch (error) {
      toast.error(getAssetErrorMessage(error) || t('Failed to save source URL'))
    } finally {
      setSavingSourceURLID('')
    }
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Temporary Assets')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          {isAdmin && stats && (
            <Card>
              <CardHeader>
                <CardTitle>{t('Platform Asset Statistics')}</CardTitle>
                <CardDescription>
                  {t(
                    'Only active Redis mappings are counted. No media content is accessed.'
                  )}
                </CardDescription>
              </CardHeader>
              <CardContent className='grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-10'>
                {[
                  [t('Total'), stats.total],
                  [t('Processing'), stats.processing],
                  [t('Successful'), stats.success],
                  [t('Failed'), stats.failed],
                  [t('Expired'), stats.expired],
                  [t('Images'), stats.by_type.image ?? 0],
                  [t('Videos'), stats.by_type.video ?? 0],
                  [t('Audio'), stats.by_type.audio ?? 0],
                  [t('Users'), stats.users],
                  [t('Expiring within 6 hours'), stats.expiring_soon],
                ].map(([label, value]) => (
                  <div
                    key={String(label)}
                    className='bg-muted/50 rounded-lg p-3'
                  >
                    <div className='text-muted-foreground text-xs'>{label}</div>
                    <div className='mt-1 text-xl font-semibold tabular-nums'>
                      {value}
                    </div>
                  </div>
                ))}
              </CardContent>
            </Card>
          )}
          <CreateAssetCard
            uploadConfig={uploadConfig}
            onCreated={() => load(false)}
          />

          <Card>
            <CardHeader>
              <div>
                <CardTitle>
                  {isAdmin
                    ? t('Platform Temporary Assets')
                    : t('My Temporary Assets')}
                </CardTitle>
                <CardDescription>
                  {t(
                    'Mappings expire automatically and are not a permanent media library.'
                  )}
                </CardDescription>
              </div>
              <CardAction>
                <Button
                  variant='outline'
                  size='sm'
                  onClick={() => void load(false)}
                >
                  <RefreshCw className='size-3.5' />
                  {t('Refresh')}
                </Button>
              </CardAction>
            </CardHeader>
            <CardContent>
              <div className='mb-4 space-y-3'>
                <AssetTypeFilter
                  value={filterType}
                  onValueChange={setFilterType}
                />
                <div className='grid gap-2 md:grid-cols-[minmax(12rem,1fr)_minmax(18rem,1.4fr)_auto]'>
                  <Input
                    value={filterID}
                    onChange={(event) => setFilterID(event.target.value)}
                    placeholder={t('Filter by asset ID')}
                  />
                  <CompactDateTimeRangePicker
                    start={filterStart}
                    end={filterEnd}
                    onChange={({ start, end }) => {
                      setFilterStart(start)
                      setFilterEnd(end)
                    }}
                  />
                  <Button
                    type='button'
                    variant='outline'
                    onClick={() => {
                      setFilterType('all')
                      setFilterID('')
                      setFilterStart(undefined)
                      setFilterEnd(undefined)
                    }}
                  >
                    {t('Reset')}
                  </Button>
                </div>
              </div>
              <div className='mb-4 flex min-h-9 flex-wrap items-center gap-2'>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  disabled={visibleIDs.length === 0 || deleting}
                  onClick={() =>
                    setSelectedIDs((current) =>
                      toggleVisibleTemporaryAssetSelection(current, visibleIDs)
                    )
                  }
                >
                  <CheckSquare2 className='size-3.5' />
                  {allVisibleSelected
                    ? t('Clear selection')
                    : t('Select all (filtered)')}
                </Button>
                {selectedIDs.size > 0 && (
                  <>
                    <span className='text-muted-foreground text-sm tabular-nums'>
                      {t('{{count}} temporary assets selected', {
                        count: selectedIDs.size,
                      })}
                    </span>
                    <Button
                      type='button'
                      variant='destructive'
                      size='sm'
                      disabled={deleting}
                      onClick={() => setDeleteTargetIDs([...selectedIDs])}
                    >
                      <Trash2 className='size-3.5' />
                      {t('Delete selected temporary assets')}
                    </Button>
                  </>
                )}
              </div>
              {loading && (
                <p className='text-muted-foreground text-sm'>
                  {t('Loading...')}
                </p>
              )}
              {!loading && filteredItems.length === 0 && (
                <p className='text-muted-foreground py-8 text-center text-sm'>
                  {items.length === 0
                    ? t('No temporary assets')
                    : t('No matching assets')}
                </p>
              )}
              {!loading && filteredItems.length > 0 && (
                <div className={TEMPORARY_ASSET_GRID_CLASS_NAME}>
                  {filteredItems.map((item) => {
                    const remaining = Math.max(
                      0,
                      item.expires_at - Math.floor(Date.now() / 1000)
                    )
                    const hours = Math.floor(remaining / 3600)
                    const uri = `asset://${item.id}`
                    return (
                      <div
                        key={item.id}
                        className={`bg-card flex min-w-0 flex-col overflow-hidden rounded-lg border transition-colors ${
                          selectedIDs.has(item.id)
                            ? 'border-primary ring-primary/20 ring-2'
                            : ''
                        }`}
                      >
                        <div className='bg-muted relative flex aspect-video w-full shrink-0 items-center justify-center overflow-hidden'>
                          <div className='bg-card/90 absolute top-2 left-2 z-10 flex rounded-md border p-1.5 shadow-sm backdrop-blur-sm'>
                            <Checkbox
                              checked={selectedIDs.has(item.id)}
                              disabled={deleting}
                              aria-label={t('Select temporary asset')}
                              onCheckedChange={() =>
                                setSelectedIDs((current) =>
                                  toggleTemporaryAssetSelection(
                                    current,
                                    item.id
                                  )
                                )
                              }
                            />
                          </div>
                          <AssetPreview
                            key={`${item.id}:${item.preview_url ?? item.source_url ?? ''}`}
                            item={item}
                            alt={item.name || t('Temporary Asset')}
                            missingLabel={t('No preview URL saved')}
                            failedLabel={t('Preview failed to load')}
                          />
                        </div>
                        <div className='flex min-w-0 flex-1 flex-col gap-2 p-3'>
                          <div className='flex min-w-0 flex-wrap items-center gap-1.5 font-medium'>
                            <span className='min-w-0 flex-1 truncate'>
                              {item.name || t('Temporary Asset')}
                            </span>
                            <span className='bg-primary/10 text-primary rounded-md px-2 py-0.5 text-xs'>
                              {t(getAssetTypeLabel(item.asset_type))}
                            </span>
                            {isAdmin && (
                              <span className='bg-muted rounded-md px-2 py-0.5 text-xs font-normal'>
                                {item.username ||
                                  `${t('User')} #${item.user_id}`}
                              </span>
                            )}
                          </div>
                          <button
                            type='button'
                            title={t('Click to copy asset ID')}
                            aria-label={t('Click to copy asset ID')}
                            className='bg-muted/60 text-muted-foreground hover:bg-muted hover:text-foreground flex min-w-0 items-center gap-1.5 rounded-md px-2 py-1.5 text-left font-mono text-[11px] transition-colors'
                            onClick={() => {
                              void navigator.clipboard.writeText(item.id)
                              toast.success(t('Asset ID copied'))
                            }}
                          >
                            <span className='min-w-0 flex-1 truncate'>
                              {item.id}
                            </span>
                            <Copy className='size-3 shrink-0' />
                          </button>
                          <div className='text-muted-foreground text-xs'>
                            {t('Created At')}:{' '}
                            {dayjs
                              .unix(item.created_at)
                              .format('YYYY-MM-DD HH:mm:ss')}
                          </div>
                          {item.verified_at > 0 && (
                            <div className='text-muted-foreground text-xs'>
                              {t('Last verified')}:{' '}
                              {dayjs
                                .unix(item.verified_at)
                                .format('YYYY-MM-DD HH:mm:ss')}
                            </div>
                          )}
                          {(item.preview_url || item.source_url) && (
                            <a
                              href={item.preview_url || item.source_url}
                              target='_blank'
                              rel='noreferrer'
                              className='text-muted-foreground hover:text-foreground flex min-w-0 items-center gap-1 text-xs'
                            >
                              <ExternalLink className='size-3 shrink-0' />
                              <span className='truncate'>
                                {item.source_kind === 'cos'
                                  ? t('Open COS preview')
                                  : item.source_url}
                              </span>
                            </a>
                          )}
                          {(item.preview_url || item.source_url) &&
                            item.asset_type === 'audio' && (
                              <audio
                                src={item.preview_url || item.source_url}
                                controls
                                preload='none'
                                className='h-8 w-full'
                              />
                            )}
                          {!item.preview_url && !item.source_url && (
                            <div className='bg-muted/40 space-y-2 rounded-md p-2'>
                              <p className='text-muted-foreground text-[11px] leading-4'>
                                {t(
                                  'This legacy asset has no saved source URL. Add it to enable preview.'
                                )}
                              </p>
                              <Input
                                type='url'
                                value={sourceURLDrafts[item.id] ?? ''}
                                placeholder={t('Add source URL')}
                                className='h-8 text-xs'
                                onChange={(event) =>
                                  setSourceURLDrafts((current) => ({
                                    ...current,
                                    [item.id]: event.target.value,
                                  }))
                                }
                                onKeyDown={(event) => {
                                  if (event.key === 'Enter') {
                                    event.preventDefault()
                                    void saveSourceURL(item.id)
                                  }
                                }}
                              />
                              <Button
                                type='button'
                                variant='secondary'
                                size='sm'
                                className='w-full'
                                disabled={
                                  savingSourceURLID === item.id ||
                                  !(sourceURLDrafts[item.id] ?? '').trim()
                                }
                                onClick={() => void saveSourceURL(item.id)}
                              >
                                {savingSourceURLID === item.id
                                  ? t('Saving...')
                                  : t('Save URL')}
                              </Button>
                            </div>
                          )}
                          <div className='mt-auto space-y-2 border-t pt-2'>
                            <div className='flex flex-wrap items-center justify-between gap-1.5'>
                              <StatusBadge
                                label={t(getAssetStatusLabel(item.status))}
                                variant={getAssetStatusVariant(item.status)}
                                size='sm'
                                copyable={false}
                              />
                              <span className='text-muted-foreground text-[11px]'>
                                {t(
                                  'Local retention: {{hours}} hours remaining',
                                  { hours }
                                )}
                              </span>
                            </div>
                            <div className='flex justify-end gap-1'>
                              <Button
                                variant='ghost'
                                size='icon-sm'
                                title={t('Copy asset URI')}
                                onClick={() => {
                                  void navigator.clipboard.writeText(uri)
                                  toast.success(t('Copied'))
                                }}
                              >
                                <Copy className='size-3.5' />
                              </Button>
                              <Button
                                variant='ghost'
                                size='icon-sm'
                                title={t('Refresh')}
                                aria-label={t('Refresh')}
                                disabled={refreshingIDs.has(item.id)}
                                onClick={() => void refresh(item.id)}
                              >
                                <RefreshCw
                                  className={`size-3.5 ${
                                    refreshingIDs.has(item.id)
                                      ? 'animate-spin'
                                      : ''
                                  }`}
                                />
                              </Button>
                              <Button
                                variant='ghost'
                                size='icon-sm'
                                title={t('Delete')}
                                disabled={deleting}
                                onClick={() => setDeleteTargetIDs([item.id])}
                              >
                                <Trash2 className='size-3.5 text-red-500' />
                              </Button>
                            </div>
                          </div>
                        </div>
                      </div>
                    )
                  })}
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      </SectionPageLayout.Content>
      <ConfirmDialog
        destructive
        open={deleteTargetIDs.length > 0}
        onOpenChange={(open) => {
          if (!open && !deleting) setDeleteTargetIDs([])
        }}
        title={
          deleteTargetIDs.length === 1
            ? t('Delete temporary asset?')
            : t('Delete {{count}} temporary assets?', {
                count: deleteTargetIDs.length,
              })
        }
        desc={t(
          'This removes the temporary asset mapping. If its file is managed by Molii COS, the COS object is also deleted. This action cannot be undone.'
        )}
        confirmText={deleting ? t('Deleting...') : t('Delete')}
        isLoading={deleting}
        handleConfirm={() => void confirmDelete()}
      />
    </SectionPageLayout>
  )
}
