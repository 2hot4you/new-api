/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import {
  AlertTriangle,
  Check,
  FileUp,
  Image,
  Link2,
  Music,
  Plus,
  Search,
  Trash2,
  Video,
} from 'lucide-react'
import { useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Progress, ProgressLabel } from '@/components/ui/progress'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import type { COSUploadConfig } from '@/features/temporary-assets/components/create-asset-card'
import {
  getAssetStatusLabel,
  type AssetType,
  type TemporaryAsset,
} from '@/features/temporary-assets/lib/asset-utils'
import { createTemporaryAssetFromFile } from '@/features/temporary-assets/lib/cos-upload'
import {
  fileNameWithoutExtension,
  inferAssetType,
} from '@/features/temporary-assets/lib/upload-utils'
import { useMediaQuery } from '@/hooks'
import { cn } from '@/lib/utils'

import type {
  VideoStudioMedia,
  VideoStudioMediaRole,
  VideoStudioMode,
} from '../types'

type PickerTarget = VideoStudioMediaRole | 'references'
type AssetFilter = 'all' | AssetType

const mediaIcon = { image: Image, video: Video, audio: Music }

function referenceRole(type: AssetType): VideoStudioMediaRole {
  if (type === 'video') return 'reference_video'
  if (type === 'audio') return 'reference_audio'
  return 'reference_image'
}

function mentionName(type: AssetType): string {
  if (type === 'image') return '图片'
  if (type === 'video') return '视频'
  return '音频'
}

function pickerTitle(target: PickerTarget | null): string {
  if (target === 'first_frame') return 'Choose first frame'
  if (target === 'last_frame') return 'Choose last frame'
  return 'Choose reference media'
}

function assetFilterLabel(filter: AssetFilter): string {
  if (filter === 'all') return 'All'
  if (filter === 'image') return 'Image'
  if (filter === 'video') return 'Video'
  return 'Audio'
}

function mediaReferenceLabel(
  item: VideoStudioMedia,
  publicURLLabel: string
): string {
  if (item.mentionIndex) {
    return `@${mentionName(item.type)}${item.mentionIndex}`
  }
  if (item.source === 'asset') return item.value
  return publicURLLabel
}

function isAvailable(asset: TemporaryAsset): boolean {
  const status = asset.status.toUpperCase()
  return (
    (status === 'ACTIVE' || status === 'SUCCESS') &&
    (!asset.expires_at || asset.expires_at > Date.now() / 1000)
  )
}

function nextMentionIndex(media: VideoStudioMedia[], type: AssetType): number {
  return (
    Math.max(
      0,
      ...media
        .filter((item) => item.type === type)
        .map((item) => item.mentionIndex ?? 0)
    ) + 1
  )
}

function createMedia(
  item: Omit<VideoStudioMedia, 'clientId' | 'role'>,
  role: VideoStudioMediaRole,
  media: VideoStudioMedia[]
): VideoStudioMedia {
  return {
    ...item,
    clientId: crypto.randomUUID(),
    role,
    mentionIndex: role.startsWith('reference_')
      ? nextMentionIndex(media, item.type)
      : undefined,
  }
}

function AssetThumbnail({ asset }: { asset: TemporaryAsset }) {
  const Icon = mediaIcon[asset.asset_type]
  if (asset.preview_url && asset.asset_type === 'image') {
    return (
      <img
        src={asset.preview_url}
        alt=''
        className='size-12 rounded-md object-cover'
      />
    )
  }
  if (asset.preview_url && asset.asset_type === 'video') {
    return (
      <video
        src={asset.preview_url}
        className='size-12 rounded-md object-cover'
        muted
      />
    )
  }
  return (
    <div className='bg-muted flex size-12 items-center justify-center rounded-md'>
      <Icon className='text-muted-foreground size-5' />
    </div>
  )
}

function FrameSlot(props: {
  role: 'first_frame' | 'last_frame'
  item?: VideoStudioMedia
  asset?: TemporaryAsset
  onChoose: () => void
  onRemove: () => void
}) {
  const { t } = useTranslation()
  const required = props.role === 'first_frame'
  const title = required ? t('First frame') : t('Last frame')
  const chooseLabel = required
    ? t('Choose first frame')
    : t('Choose last frame')
  const previewURL =
    props.asset?.preview_url ??
    (props.item?.source === 'url' ? props.item.value : undefined)
  const available = props.item
    ? props.item.source === 'url' ||
      (props.asset ? isAvailable(props.asset) : false)
    : false

  return (
    <div className='group bg-muted/20 flex min-h-40 flex-col overflow-hidden rounded-xl border'>
      <button
        type='button'
        aria-label={chooseLabel}
        className='hover:bg-muted/50 relative flex min-h-32 flex-1 flex-col items-center justify-center gap-2 overflow-hidden p-4 text-center transition-colors'
        onClick={props.onChoose}
      >
        {previewURL ? (
          <img
            src={previewURL}
            alt=''
            className='absolute inset-0 size-full object-cover opacity-20'
          />
        ) : null}
        <div className='bg-background relative flex size-10 items-center justify-center rounded-full border'>
          {props.item ? (
            <Image className='size-5' />
          ) : (
            <Plus className='size-5' />
          )}
        </div>
        <div className='relative max-w-full min-w-0'>
          <p className='text-sm font-medium'>
            {title} · {required ? t('Required') : t('Optional')}
          </p>
          <p className='text-muted-foreground mt-1 truncate text-xs'>
            {props.item?.name || chooseLabel}
          </p>
          {props.item && (
            <p
              className={cn(
                'mt-1 text-xs',
                available ? 'text-emerald-600' : 'text-red-600'
              )}
            >
              {available ? t('Available') : t('Unavailable')}
            </p>
          )}
        </div>
      </button>
      {props.item && (
        <div className='flex items-center justify-end gap-1 border-t px-2 py-1.5'>
          {previewURL && (
            <a
              href={previewURL}
              target='_blank'
              rel='noreferrer'
              className='hover:bg-muted rounded-md px-2 py-1 text-xs font-medium'
            >
              {t('Preview')}
            </a>
          )}
          <Button
            type='button'
            size='sm'
            variant='ghost'
            onClick={props.onChoose}
          >
            {t('Replace')}
          </Button>
          <Button
            type='button'
            size='icon-sm'
            variant='ghost'
            aria-label={`${t('Remove media')} · ${title}`}
            onClick={props.onRemove}
          >
            <Trash2 />
          </Button>
        </div>
      )}
    </div>
  )
}

export function VideoStudioMediaPicker(props: {
  mode: VideoStudioMode
  media: VideoStudioMedia[]
  assets: TemporaryAsset[]
  uploadConfig?: COSUploadConfig
  onChange: (media: VideoStudioMedia[]) => void
  onAssetsChanged: () => Promise<unknown>
}) {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const fileInput = useRef<HTMLInputElement>(null)
  const [target, setTarget] = useState<PickerTarget | null>(null)
  const [filter, setFilter] = useState<AssetFilter>('all')
  const [search, setSearch] = useState('')
  const [selectedIDs, setSelectedIDs] = useState<Set<string>>(new Set())
  const [urlType, setURLType] = useState<AssetType>('image')
  const [url, setURL] = useState('')
  const [uploading, setUploading] = useState(false)
  const [progress, setProgress] = useState(0)

  const firstFrame = props.media.find((item) => item.role === 'first_frame')
  const lastFrame = props.media.find((item) => item.role === 'last_frame')

  const visibleAssets = useMemo(() => {
    const needle = search.trim().toLowerCase()
    return [...props.assets]
      .filter(
        (asset) => target === 'references' || asset.asset_type === 'image'
      )
      .filter((asset) => filter === 'all' || asset.asset_type === filter)
      .filter((asset) => {
        if (!needle) return true
        return `${asset.name ?? ''} ${asset.id}`.toLowerCase().includes(needle)
      })
      .sort((left, right) => right.created_at - left.created_at)
  }, [filter, props.assets, search, target])

  const openPicker = (nextTarget: PickerTarget) => {
    setTarget(nextTarget)
    setFilter(nextTarget === 'references' ? 'all' : 'image')
    setSearch('')
    setURLType('image')
    setURL('')
    setSelectedIDs(new Set())
  }

  const closePicker = () => {
    setTarget(null)
    setSelectedIDs(new Set())
  }

  const addForTarget = (
    item: Omit<VideoStudioMedia, 'clientId' | 'role'>,
    explicitTarget = target
  ) => {
    if (!explicitTarget) return
    const role =
      explicitTarget === 'references'
        ? referenceRole(item.type)
        : explicitTarget
    if (role === 'first_frame' || role === 'last_frame') {
      if (item.type !== 'image') {
        toast.error(t('Frame mode only accepts images'))
        return
      }
      const withoutSlot = props.media.filter((current) => current.role !== role)
      props.onChange([...withoutSlot, createMedia(item, role, withoutSlot)])
      closePicker()
      return
    }
    props.onChange([...props.media, createMedia(item, role, props.media)])
  }

  const addSelectedReferences = () => {
    const nextMedia = [...props.media]
    for (const asset of [...props.assets].sort(
      (left, right) => right.created_at - left.created_at
    )) {
      if (!selectedIDs.has(asset.id) || !isAvailable(asset)) continue
      if (
        nextMedia.some(
          (item) => item.source === 'asset' && item.value === asset.id
        )
      ) {
        continue
      }
      const role = referenceRole(asset.asset_type)
      nextMedia.push(
        createMedia(
          {
            type: asset.asset_type,
            source: 'asset',
            value: asset.id,
            name: asset.name || asset.id,
            expiresAt: asset.expires_at,
          },
          role,
          nextMedia
        )
      )
    }
    props.onChange(nextMedia)
    closePicker()
  }

  const selectAsset = (asset: TemporaryAsset) => {
    if (!isAvailable(asset)) return
    if (target === 'references') {
      setSelectedIDs((current) => {
        const next = new Set(current)
        if (next.has(asset.id)) next.delete(asset.id)
        else next.add(asset.id)
        return next
      })
      return
    }
    addForTarget({
      type: asset.asset_type,
      source: 'asset',
      value: asset.id,
      name: asset.name || asset.id,
      expiresAt: asset.expires_at,
    })
  }

  const upload = async (file: File | undefined) => {
    if (!file) return
    const type = inferAssetType(file.name, file.type)
    if (!type) {
      toast.error(t('Unsupported temporary asset file type'))
      return
    }
    if (target !== 'references' && type !== 'image') {
      toast.error(t('Frame mode only accepts images'))
      return
    }
    const limit = props.uploadConfig?.limits[type]
    if (limit && file.size > limit) {
      toast.error(t('The selected file exceeds the upload limit'))
      return
    }
    setUploading(true)
    setProgress(0)
    try {
      const asset = await createTemporaryAssetFromFile({
        file,
        assetType: type,
        name: fileNameWithoutExtension(file.name).slice(0, 80),
        onProgress: setProgress,
      })
      addForTarget({
        type,
        source: 'asset',
        value: asset.id,
        name: asset.name || file.name,
        expiresAt: asset.expires_at,
      })
      await props.onAssetsChanged()
      toast.success(t('File uploaded and added to this request'))
    } catch {
      toast.error(t('Failed to upload temporary asset'))
    } finally {
      setUploading(false)
      if (fileInput.current) fileInput.current.value = ''
    }
  }

  const addURL = () => {
    const value = url.trim()
    try {
      const parsed = new URL(value)
      if (!['http:', 'https:'].includes(parsed.protocol)) throw new Error()
    } catch {
      toast.error(t('Enter a valid public HTTP or HTTPS URL'))
      return
    }
    addForTarget({ type: urlType, source: 'url', value, name: value })
    setURL('')
  }

  if (props.mode === 'text') {
    return (
      <p className='text-muted-foreground text-sm'>
        {t('Text mode does not include reference media.')}
      </p>
    )
  }

  const title = t(pickerTitle(target))

  return (
    <div className='space-y-4'>
      {props.mode === 'frames' ? (
        <div className='grid gap-3 sm:grid-cols-2'>
          <FrameSlot
            role='first_frame'
            item={firstFrame}
            asset={props.assets.find((asset) => asset.id === firstFrame?.value)}
            onChoose={() => openPicker('first_frame')}
            onRemove={() =>
              props.onChange(
                props.media.filter((item) => item.role !== 'first_frame')
              )
            }
          />
          <FrameSlot
            role='last_frame'
            item={lastFrame}
            asset={props.assets.find((asset) => asset.id === lastFrame?.value)}
            onChoose={() => openPicker('last_frame')}
            onRemove={() =>
              props.onChange(
                props.media.filter((item) => item.role !== 'last_frame')
              )
            }
          />
        </div>
      ) : (
        <div className='space-y-3'>
          <Button
            type='button'
            variant='outline'
            onClick={() => openPicker('references')}
          >
            <Plus />
            {t('Add reference media')}
          </Button>
          <div className='grid gap-2 sm:grid-cols-2'>
            {props.media.map((item) => {
              const Icon = mediaIcon[item.type]
              return (
                <div
                  key={item.clientId}
                  className='bg-muted/30 flex min-w-0 items-center gap-3 rounded-lg border p-3'
                >
                  <Icon className='text-muted-foreground size-4 shrink-0' />
                  <div className='min-w-0 flex-1'>
                    <p className='truncate text-sm font-medium'>{item.name}</p>
                    <p className='text-muted-foreground text-xs'>
                      {mediaReferenceLabel(item, t('Public URL'))}
                    </p>
                  </div>
                  <Button
                    type='button'
                    size='icon-sm'
                    variant='ghost'
                    aria-label={t('Remove media')}
                    onClick={() =>
                      props.onChange(
                        props.media.filter(
                          (current) => current.clientId !== item.clientId
                        )
                      )
                    }
                  >
                    <Trash2 />
                  </Button>
                </div>
              )
            })}
          </div>
        </div>
      )}

      <Sheet
        open={target !== null}
        onOpenChange={(open) => !open && closePicker()}
      >
        <SheetContent
          side={isMobile ? 'bottom' : 'right'}
          className={
            isMobile
              ? sideDrawerContentClassName('h-[88dvh] rounded-t-2xl')
              : sideDrawerContentClassName('sm:max-w-2xl')
          }
        >
          <SheetHeader className={sideDrawerHeaderClassName()}>
            <SheetTitle>{title}</SheetTitle>
            <SheetDescription>
              {target === 'references'
                ? t(
                    'Select images, videos, or audio from your temporary asset library.'
                  )
                : t(
                    'Select an available image, upload a new one, or enter a public URL.'
                  )}
            </SheetDescription>
          </SheetHeader>

          <div className={sideDrawerFormClassName('gap-5')}>
            <div className='flex flex-wrap gap-2'>
              {(target === 'references'
                ? (['all', 'image', 'video', 'audio'] as const)
                : (['image'] as const)
              ).map((value) => (
                <Button
                  key={value}
                  type='button'
                  size='sm'
                  variant={filter === value ? 'default' : 'outline'}
                  onClick={() => setFilter(value)}
                >
                  {t(assetFilterLabel(value))}
                </Button>
              ))}
              <div className='relative min-w-48 flex-1'>
                <Search className='text-muted-foreground absolute top-1/2 left-2.5 size-4 -translate-y-1/2' />
                <Input
                  type='search'
                  className='pl-8'
                  value={search}
                  placeholder={t('Search name or asset ID')}
                  onChange={(event) => setSearch(event.target.value)}
                />
              </div>
            </div>

            <div
              role='listbox'
              aria-label={t('Temporary assets')}
              aria-multiselectable={target === 'references'}
              className='grid gap-2 sm:grid-cols-2'
            >
              {visibleAssets.map((asset) => {
                const available = isAvailable(asset)
                const alreadySelected = props.media.some(
                  (item) => item.source === 'asset' && item.value === asset.id
                )
                const selected = alreadySelected || selectedIDs.has(asset.id)
                return (
                  <button
                    key={asset.id}
                    type='button'
                    role='option'
                    aria-selected={selected}
                    disabled={!available || alreadySelected}
                    className={cn(
                      'relative flex min-w-0 items-center gap-3 rounded-lg border p-3 text-left transition-colors hover:bg-accent disabled:cursor-not-allowed disabled:opacity-70',
                      selected && 'border-primary bg-primary/5',
                      !available &&
                        'border-red-200 bg-red-50/70 dark:border-red-900 dark:bg-red-950/20'
                    )}
                    onClick={() => selectAsset(asset)}
                  >
                    <AssetThumbnail asset={asset} />
                    <span className='min-w-0 flex-1'>
                      <span className='block truncate text-sm font-medium'>
                        {asset.name || asset.id}
                      </span>
                      <span className='text-muted-foreground block truncate text-xs'>
                        {asset.id}
                      </span>
                      <span
                        className={cn(
                          'mt-1 block text-xs',
                          available ? 'text-emerald-600' : 'text-red-600'
                        )}
                      >
                        {available
                          ? t('Available')
                          : asset.error?.message ||
                            t(getAssetStatusLabel(asset.status))}
                      </span>
                    </span>
                    {selected && (
                      <Check className='text-primary size-4 shrink-0' />
                    )}
                    {!available && (
                      <AlertTriangle className='size-4 shrink-0 text-red-600' />
                    )}
                  </button>
                )
              })}
              {visibleAssets.length === 0 && (
                <p className='text-muted-foreground col-span-full py-10 text-center text-sm'>
                  {t('No matching assets')}
                </p>
              )}
            </div>

            <div className='space-y-3 border-t pt-5'>
              <p className='text-sm font-medium'>{t('Add new media')}</p>
              <input
                ref={fileInput}
                className='sr-only'
                type='file'
                accept={
                  target === 'references'
                    ? '.jpg,.jpeg,.png,.webp,.bmp,.tif,.tiff,.gif,.mp4,.mov,.wav,.mp3'
                    : '.jpg,.jpeg,.png,.webp,.bmp,.tif,.tiff,.gif'
                }
                disabled={!props.uploadConfig?.enabled || uploading}
                onChange={(event) => void upload(event.target.files?.[0])}
              />
              <Button
                type='button'
                variant='outline'
                disabled={!props.uploadConfig?.enabled || uploading}
                onClick={() => fileInput.current?.click()}
              >
                <FileUp />
                {uploading ? t('Uploading...') : t('Upload from device')}
              </Button>
              {uploading && (
                <Progress value={progress}>
                  <ProgressLabel>{progress}%</ProgressLabel>
                </Progress>
              )}
              <div className='grid gap-2 sm:grid-cols-[120px_1fr_auto]'>
                {target === 'references' && (
                  <Select
                    value={urlType}
                    onValueChange={(value) => setURLType(value as AssetType)}
                  >
                    <SelectTrigger aria-label={t('URL media type')}>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value='image'>{t('Image')}</SelectItem>
                      <SelectItem value='video'>{t('Video')}</SelectItem>
                      <SelectItem value='audio'>{t('Audio')}</SelectItem>
                    </SelectContent>
                  </Select>
                )}
                <Input
                  value={url}
                  className={target === 'references' ? '' : 'sm:col-span-2'}
                  placeholder='https://...'
                  aria-label={t('Public media URL')}
                  onChange={(event) => setURL(event.target.value)}
                />
                <Button type='button' variant='outline' onClick={addURL}>
                  <Link2 />
                  {t('Add URL')}
                </Button>
              </div>
            </div>
          </div>

          <SheetFooter className={sideDrawerFooterClassName()}>
            <Button type='button' variant='outline' onClick={closePicker}>
              {t('Cancel')}
            </Button>
            {target === 'references' && (
              <Button
                type='button'
                disabled={selectedIDs.size === 0}
                aria-label={`Add selected (${selectedIDs.size})`}
                onClick={addSelectedReferences}
              >
                {t('Add selected')} ({selectedIDs.size})
              </Button>
            )}
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </div>
  )
}
