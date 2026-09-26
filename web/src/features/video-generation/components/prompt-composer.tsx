/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import {
  AlertTriangle,
  FileImage,
  Film,
  Image,
  Music,
  Search,
  X,
} from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  HoverCard,
  HoverCardContent,
  HoverCardTrigger,
} from '@/components/ui/hover-card'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import type {
  AssetType,
  TemporaryAsset,
} from '@/features/temporary-assets/lib/asset-utils'
import { cn } from '@/lib/utils'

import type {
  VideoStudioMedia,
  VideoStudioMediaRole,
  VideoStudioMode,
} from '../types'

type AssetFilter = 'all' | AssetType

const typeMeta = {
  image: { label: 'Image', mention: '图片', icon: Image },
  video: { label: 'Video', mention: '视频', icon: Film },
  audio: { label: 'Audio', mention: '音频', icon: Music },
} as const

function assetAvailable(asset: TemporaryAsset | undefined): boolean {
  if (!asset) return false
  const status = asset.status.toUpperCase()
  return (
    ['ACTIVE', 'SUCCESS'].includes(status) &&
    (!asset.expires_at || asset.expires_at > Date.now() / 1000)
  )
}

function roleForAsset(
  mode: VideoStudioMode,
  type: AssetType,
  media: VideoStudioMedia[]
): VideoStudioMediaRole {
  if (mode === 'frames') {
    return media.some((item) => item.role === 'first_frame')
      ? 'last_frame'
      : 'first_frame'
  }
  if (type === 'video') return 'reference_video'
  if (type === 'audio') return 'reference_audio'
  return 'reference_image'
}

function mentionIndexForMedia(
  item: VideoStudioMedia,
  media: VideoStudioMedia[]
): number {
  if (item.mentionIndex) return item.mentionIndex
  return (
    media.filter((candidate) => candidate.type === item.type).indexOf(item) + 1
  )
}

function nextMentionIndex(media: VideoStudioMedia[], type: AssetType): number {
  return (
    Math.max(
      0,
      ...media
        .filter((item) => item.type === type)
        .map((item) => mentionIndexForMedia(item, media))
    ) + 1
  )
}

function mentionRange(value: string, cursor: number) {
  const before = value.slice(0, cursor)
  const match = before.match(/(^|\s)@([^\s@]*)$/)
  if (!match || match.index == null) return null
  return {
    start: match.index + match[1].length,
    end: cursor,
    query: match[2],
  }
}

function AssetPreview({ asset }: { asset: TemporaryAsset }) {
  if (!asset.preview_url) {
    return (
      <div className='bg-muted text-muted-foreground flex aspect-video items-center justify-center rounded-md'>
        <FileImage className='size-8' />
      </div>
    )
  }
  if (asset.asset_type === 'video') {
    return (
      <video
        className='aspect-video w-full rounded-md object-cover'
        src={asset.preview_url}
        controls
        muted
      />
    )
  }
  if (asset.asset_type === 'audio') {
    return <audio className='w-full' src={asset.preview_url} controls />
  }
  return (
    <img
      className='aspect-video w-full rounded-md object-cover'
      src={asset.preview_url}
      alt={asset.name || asset.id}
    />
  )
}

function AssetChipThumbnail({
  asset,
  fallback: Fallback,
}: {
  asset?: TemporaryAsset
  fallback: typeof Image
}) {
  if (asset?.preview_url && asset.asset_type === 'image') {
    return (
      <img
        src={asset.preview_url}
        alt=''
        className='size-7 rounded object-cover'
      />
    )
  }
  if (asset?.preview_url && asset.asset_type === 'video') {
    return (
      <video
        src={asset.preview_url}
        className='size-7 rounded object-cover'
        muted
      />
    )
  }
  return <Fallback className='size-4 shrink-0' />
}

export function VideoStudioPromptComposer(props: {
  value: string
  mode: VideoStudioMode
  media: VideoStudioMedia[]
  assets: TemporaryAsset[]
  onChange: (value: string) => void
  onMediaChange: (media: VideoStudioMedia[]) => void
}) {
  const { t } = useTranslation()
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const [open, setOpen] = useState(false)
  const [cursor, setCursor] = useState(props.value.length)
  const [filter, setFilter] = useState<AssetFilter>('all')
  const [search, setSearch] = useState('')
  const referencesEnabled = props.mode === 'references'
  const currentMention = referencesEnabled
    ? mentionRange(props.value, cursor)
    : null

  useEffect(() => {
    if (!referencesEnabled) {
      setOpen(false)
      setSearch('')
      return
    }
    const mention = mentionRange(props.value, cursor)
    if (mention) {
      setOpen(true)
      setSearch(mention.query)
    } else {
      setOpen(false)
    }
  }, [cursor, props.value, referencesEnabled])

  const assets = useMemo(() => {
    const needle = search.trim().toLowerCase()
    return [...props.assets]
      .filter(
        (asset) => props.mode !== 'frames' || asset.asset_type === 'image'
      )
      .filter((asset) => filter === 'all' || asset.asset_type === filter)
      .filter((asset) => {
        if (!needle) return true
        return `${asset.name ?? ''} ${asset.id}`.toLowerCase().includes(needle)
      })
      .sort((left, right) => right.created_at - left.created_at)
  }, [filter, props.assets, props.mode, search])

  const selectedAssets = referencesEnabled
    ? props.media
        .filter((item) => item.source === 'asset')
        .map((item) => ({
          media: item,
          asset: props.assets.find((asset) => asset.id === item.value),
        }))
    : []

  const selectAsset = (asset: TemporaryAsset) => {
    if (!assetAvailable(asset)) return
    let nextMedia = props.media
    const existing = nextMedia.find(
      (item) => item.source === 'asset' && item.value === asset.id
    )
    if (!existing) {
      nextMedia = [
        ...nextMedia,
        {
          clientId: crypto.randomUUID(),
          type: asset.asset_type,
          source: 'asset',
          role: roleForAsset(props.mode, asset.asset_type, nextMedia),
          value: asset.id,
          name: asset.name || asset.id,
          expiresAt: asset.expires_at,
          mentionIndex: nextMentionIndex(nextMedia, asset.asset_type),
        },
      ]
      props.onMediaChange(nextMedia)
    }
    const selected = nextMedia.find(
      (item) => item.source === 'asset' && item.value === asset.id
    )
    const typeIndex = selected
      ? mentionIndexForMedia(selected, nextMedia)
      : nextMentionIndex(nextMedia, asset.asset_type)
    const token = `@${typeMeta[asset.asset_type].mention}${typeIndex}`
    const range = currentMention ?? {
      start: props.value.length,
      end: props.value.length,
    }
    const prefix = range.start === props.value.length ? ' ' : ''
    const value = `${props.value.slice(0, range.start)}${prefix}${token} ${props.value.slice(range.end)}`
    props.onChange(value)
    setOpen(false)
    setSearch('')
    requestAnimationFrame(() => textareaRef.current?.focus())
  }

  const removeAsset = (clientID: string) => {
    props.onMediaChange(
      props.media.filter((item) => item.clientId !== clientID)
    )
  }

  return (
    <div className='focus-within:border-ring focus-within:ring-ring/50 relative rounded-lg border bg-transparent focus-within:ring-3'>
      {selectedAssets.length > 0 && (
        <div className='flex flex-wrap gap-2 border-b p-2.5'>
          {selectedAssets.map(({ media, asset }) => {
            const available = assetAvailable(asset)
            const Icon = typeMeta[media.type].icon
            const chip = (
              <div
                key={media.clientId}
                data-testid={`prompt-asset-${media.value}`}
                data-asset-status={available ? 'available' : 'unavailable'}
                className={cn(
                  'flex max-w-full items-center gap-2 rounded-md border px-2 py-1.5 text-xs',
                  available
                    ? 'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-200'
                    : 'border-red-200 bg-red-50 text-red-800 dark:border-red-900 dark:bg-red-950/40 dark:text-red-200'
                )}
              >
                <AssetChipThumbnail asset={asset} fallback={Icon} />
                <span className='max-w-48 min-w-0'>
                  <span className='block truncate font-medium'>
                    @{typeMeta[media.type].mention}
                    {mentionIndexForMedia(media, props.media)}
                  </span>
                  <span className='block truncate text-[11px]'>
                    {media.name}
                  </span>
                  {!available && (
                    <span className='block truncate text-[11px]'>
                      {asset?.error?.message ||
                        t('This asset is not available for generation.')}
                    </span>
                  )}
                </span>
                {!available && <AlertTriangle className='size-3.5 shrink-0' />}
                <Button
                  type='button'
                  variant='ghost'
                  size='icon-xs'
                  aria-label={t('Remove media')}
                  onClick={() => removeAsset(media.clientId)}
                >
                  <X />
                </Button>
              </div>
            )
            if (!asset) return chip
            return (
              <HoverCard key={media.clientId}>
                <HoverCardTrigger render={chip} />
                <HoverCardContent className='w-80 space-y-2'>
                  <AssetPreview asset={asset} />
                  <div>
                    <p className='font-medium'>{asset.name || asset.id}</p>
                    <p className='text-muted-foreground text-xs break-all'>
                      {asset.id}
                    </p>
                    {!available && (
                      <p className='mt-1 text-xs text-red-600'>
                        {asset.error?.message ||
                          t('This asset is not available for generation.')}
                      </p>
                    )}
                  </div>
                </HoverCardContent>
              </HoverCard>
            )
          })}
        </div>
      )}

      <Textarea
        ref={textareaRef}
        id='video-studio-prompt'
        aria-label={t('Prompt')}
        value={props.value}
        className='min-h-56 resize-y rounded-none border-0 focus-visible:ring-0'
        placeholder={
          referencesEnabled
            ? t(
                'Describe the video, camera movement, scene, dialogue, and sound... Type @ to reference an asset.'
              )
            : t(
                'Describe the video, camera movement, scene, dialogue, and sound...'
              )
        }
        onClick={(event) => setCursor(event.currentTarget.selectionStart)}
        onKeyUp={(event) => setCursor(event.currentTarget.selectionStart)}
        onChange={(event) => {
          const nextCursor = event.currentTarget.selectionStart
          props.onChange(event.currentTarget.value)
          setCursor(nextCursor)
        }}
        onKeyDown={(event) => {
          if (event.key === 'Escape') setOpen(false)
        }}
      />

      {open && (
        <div className='bg-popover absolute inset-x-2 top-full z-40 mt-2 rounded-lg border p-3 shadow-xl'>
          <div className='mb-3 flex flex-wrap items-center gap-2'>
            {(props.mode === 'frames'
              ? ([
                  ['all', 'All'],
                  ['image', 'Image'],
                ] as const)
              : ([
                  ['all', 'All'],
                  ['image', 'Image'],
                  ['video', 'Video'],
                  ['audio', 'Audio'],
                ] as const)
            ).map(([value, label]) => (
              <Button
                key={value}
                type='button'
                size='sm'
                variant={filter === value ? 'default' : 'outline'}
                aria-label={t(label)}
                onClick={() => setFilter(value)}
              >
                {t(label)}
              </Button>
            ))}
            <div className='relative ml-auto min-w-48 flex-1 sm:max-w-64'>
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
            className='grid max-h-72 gap-2 overflow-y-auto sm:grid-cols-2'
          >
            {assets.map((asset) => {
              const available = assetAvailable(asset)
              const Icon = typeMeta[asset.asset_type].icon
              return (
                <button
                  key={asset.id}
                  type='button'
                  role='option'
                  aria-selected={props.media.some(
                    (item) => item.source === 'asset' && item.value === asset.id
                  )}
                  disabled={!available}
                  className={cn(
                    'flex min-w-0 items-center gap-2 rounded-md border p-2 text-left transition-colors hover:bg-accent disabled:cursor-not-allowed disabled:opacity-70',
                    !available &&
                      'border-red-200 bg-red-50/60 dark:border-red-900 dark:bg-red-950/20'
                  )}
                  onClick={() => selectAsset(asset)}
                >
                  {asset.preview_url && asset.asset_type === 'image' ? (
                    <img
                      src={asset.preview_url}
                      alt=''
                      className='size-10 rounded object-cover'
                    />
                  ) : (
                    <div className='bg-muted flex size-10 items-center justify-center rounded'>
                      <Icon className='size-4' />
                    </div>
                  )}
                  <span className='min-w-0 flex-1'>
                    <span className='block truncate text-sm font-medium'>
                      {asset.name || asset.id}
                    </span>
                    <span className='text-muted-foreground block truncate text-xs'>
                      {asset.id}
                    </span>
                  </span>
                  {!available && (
                    <AlertTriangle className='size-4 shrink-0 text-red-600' />
                  )}
                </button>
              )
            })}
            {assets.length === 0 && (
              <p className='text-muted-foreground col-span-full py-8 text-center text-sm'>
                {t('No matching assets')}
              </p>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
