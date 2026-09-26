/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { FileUp, Image, Link2, Music, Plus, Trash2, Video } from 'lucide-react'
import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Progress, ProgressLabel } from '@/components/ui/progress'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import type { COSUploadConfig } from '@/features/temporary-assets/components/create-asset-card'
import type {
  AssetType,
  TemporaryAsset,
} from '@/features/temporary-assets/lib/asset-utils'
import { createTemporaryAssetFromFile } from '@/features/temporary-assets/lib/cos-upload'
import {
  fileNameWithoutExtension,
  inferAssetType,
} from '@/features/temporary-assets/lib/upload-utils'

import type {
  VideoStudioMedia,
  VideoStudioMediaRole,
  VideoStudioMode,
} from '../types'

const mediaIcon = { image: Image, video: Video, audio: Music }

function roleForMedia(
  mode: VideoStudioMode,
  type: AssetType,
  selected: VideoStudioMedia[]
): VideoStudioMediaRole {
  if (mode === 'frames') {
    return selected.some((item) => item.role === 'first_frame')
      ? 'last_frame'
      : 'first_frame'
  }
  if (type === 'video') return 'reference_video'
  if (type === 'audio') return 'reference_audio'
  return 'reference_image'
}

function isAvailable(asset: TemporaryAsset): boolean {
  const status = asset.status.toUpperCase()
  return (
    (status === 'ACTIVE' || status === 'SUCCESS') &&
    (!asset.expires_at || asset.expires_at > Date.now() / 1000)
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
  const fileInput = useRef<HTMLInputElement>(null)
  const [urlType, setURLType] = useState<AssetType>('image')
  const [url, setURL] = useState('')
  const [uploading, setUploading] = useState(false)
  const [progress, setProgress] = useState(0)
  const availableAssets = props.assets.filter(isAvailable)
  const firstFrame = props.media.find((item) => item.role === 'first_frame')
  const lastFrame = props.media.find((item) => item.role === 'last_frame')
  const frameSlotsFull =
    props.mode === 'frames' && Boolean(firstFrame && lastFrame)

  const append = (item: Omit<VideoStudioMedia, 'clientId' | 'role'>) => {
    if (props.mode === 'frames' && item.type !== 'image') {
      toast.error(t('Frame mode only accepts images'))
      return
    }
    if (frameSlotsFull) {
      toast.error(
        t('Frame mode supports one first frame and one optional last frame.')
      )
      return
    }
    props.onChange([
      ...props.media,
      {
        ...item,
        clientId: crypto.randomUUID(),
        role: roleForMedia(props.mode, item.type, props.media),
      },
    ])
  }

  const upload = async (file: File | undefined) => {
    if (!file) return
    const type = inferAssetType(file.name, file.type)
    if (!type) {
      toast.error(t('Unsupported temporary asset file type'))
      return
    }
    if (props.mode === 'frames' && type !== 'image') {
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
      append({
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
    append({ type: urlType, source: 'url', value, name: value })
    setURL('')
  }

  if (props.mode === 'text') {
    return (
      <p className='text-muted-foreground text-sm'>
        {t('Text mode does not include reference media.')}
      </p>
    )
  }

  return (
    <div className='space-y-4'>
      {props.mode === 'frames' && (
        <div className='grid gap-3 sm:grid-cols-2'>
          <div className='rounded-lg border p-3'>
            <p className='text-sm font-medium'>
              {t('First frame')} · {t('Required')}
            </p>
            <p className='text-muted-foreground mt-1 truncate text-xs'>
              {firstFrame?.name || '—'}
            </p>
          </div>
          <div className='rounded-lg border p-3'>
            <p className='text-sm font-medium'>
              {t('Last frame')} · {t('Optional')}
            </p>
            <p className='text-muted-foreground mt-1 truncate text-xs'>
              {lastFrame?.name || '—'}
            </p>
          </div>
        </div>
      )}
      <div className='grid gap-3 sm:grid-cols-[1fr_auto]'>
        <div>
          <Label htmlFor='video-studio-asset'>
            {props.mode === 'frames'
              ? t('First / last frame')
              : t('Temporary asset library')}
          </Label>
          <Select
            disabled={frameSlotsFull}
            onValueChange={(id) => {
              const asset = availableAssets.find((item) => item.id === id)
              if (!asset) return
              append({
                type: asset.asset_type,
                source: 'asset',
                value: asset.id,
                name: asset.name || asset.id,
                expiresAt: asset.expires_at,
              })
            }}
          >
            <SelectTrigger id='video-studio-asset'>
              <SelectValue
                placeholder={
                  frameSlotsFull
                    ? t('First / last frame')
                    : t('Choose an available asset')
                }
              />
            </SelectTrigger>
            <SelectContent>
              {availableAssets
                .filter(
                  (item) =>
                    props.mode !== 'frames' || item.asset_type === 'image'
                )
                .map((item) => (
                  <SelectItem key={item.id} value={item.id}>
                    {item.name || item.id} · {item.asset_type}
                  </SelectItem>
                ))}
            </SelectContent>
          </Select>
        </div>
        <div className='flex items-end'>
          <input
            ref={fileInput}
            className='sr-only'
            type='file'
            accept={
              props.mode === 'frames'
                ? '.jpg,.jpeg,.png,.webp,.bmp,.tif,.tiff,.gif'
                : '.jpg,.jpeg,.png,.webp,.bmp,.tif,.tiff,.gif,.mp4,.mov,.wav,.mp3'
            }
            disabled={
              !props.uploadConfig?.enabled || uploading || frameSlotsFull
            }
            onChange={(event) => void upload(event.target.files?.[0])}
          />
          <Button
            type='button'
            variant='outline'
            disabled={
              !props.uploadConfig?.enabled || uploading || frameSlotsFull
            }
            onClick={() => fileInput.current?.click()}
          >
            <FileUp />
            {uploading ? t('Uploading...') : t('Upload from device')}
          </Button>
        </div>
      </div>
      {uploading && (
        <Progress value={progress}>
          <ProgressLabel>{progress}%</ProgressLabel>
        </Progress>
      )}

      <div className='grid gap-2 sm:grid-cols-[140px_1fr_auto]'>
        <Select
          value={urlType}
          onValueChange={(value) => setURLType(value as AssetType)}
        >
          <SelectTrigger aria-label={t('URL media type')}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value='image'>{t('Image')}</SelectItem>
            {props.mode !== 'frames' && (
              <>
                <SelectItem value='video'>{t('Video')}</SelectItem>
                <SelectItem value='audio'>{t('Audio')}</SelectItem>
              </>
            )}
          </SelectContent>
        </Select>
        <Input
          value={url}
          placeholder='https://...'
          aria-label={t('Public media URL')}
          onChange={(event) => setURL(event.target.value)}
        />
        <Button
          type='button'
          variant='outline'
          disabled={frameSlotsFull}
          onClick={addURL}
        >
          <Link2 />
          {t('Add URL')}
        </Button>
      </div>

      <div className='space-y-2'>
        {props.media.map((item, index) => {
          const Icon = mediaIcon[item.type]
          return (
            <div
              key={item.clientId}
              className='bg-muted/30 flex items-center gap-3 rounded-lg border p-3'
            >
              <Icon className='text-muted-foreground size-4 shrink-0' />
              <div className='min-w-0 flex-1'>
                <p className='truncate text-sm font-medium'>{item.name}</p>
                <p className='text-muted-foreground text-xs'>
                  {t(item.role)} ·{' '}
                  {item.source === 'asset' ? item.value : t('Public URL')}
                </p>
              </div>
              {props.mode === 'frames' && (
                <Select
                  value={item.role}
                  onValueChange={(role) =>
                    props.onChange(
                      props.media.map((current, currentIndex) => {
                        if (currentIndex === index) {
                          return {
                            ...current,
                            role: role as VideoStudioMediaRole,
                          }
                        }
                        if (current.role === role) {
                          return {
                            ...current,
                            role:
                              role === 'first_frame'
                                ? 'last_frame'
                                : 'first_frame',
                          }
                        }
                        return current
                      })
                    )
                  }
                >
                  <SelectTrigger className='w-32' aria-label={t('Frame role')}>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='first_frame'>
                      {t('First frame')}
                    </SelectItem>
                    <SelectItem value='last_frame'>
                      {t('Last frame')}
                    </SelectItem>
                  </SelectContent>
                </Select>
              )}
              <Button
                type='button'
                size='icon'
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
        {props.media.length === 0 && (
          <div className='text-muted-foreground flex min-h-20 items-center justify-center rounded-lg border border-dashed text-sm'>
            <Plus className='mr-2 size-4' />
            {props.mode === 'frames'
              ? t('Frame mode requires exactly one first-frame image.')
              : t('Add reference media for this generation')}
          </div>
        )}
      </div>
    </div>
  )
}
