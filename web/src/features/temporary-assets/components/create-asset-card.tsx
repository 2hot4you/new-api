/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { FileUp, Image, Music, Video } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Progress, ProgressLabel } from '@/components/ui/progress'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { api } from '@/lib/api'

import { type AssetType, getAssetTypeLabel } from '../lib/asset-utils'
import {
  fileNameWithoutExtension,
  formatUploadSize,
  inferAssetType,
} from '../lib/upload-utils'

export type COSUploadConfig = {
  enabled: boolean
  limits: Record<AssetType, number>
}

type UploadAuthorization = {
  upload_id: string
  upload_url: string
  headers: Record<string, string>
}

type CreateAssetCardProps = {
  uploadConfig: COSUploadConfig
  onCreated: () => Promise<void>
}

function getRequestError(error: unknown): string | undefined {
  if (!error || typeof error !== 'object' || !('response' in error)) return
  const response = error.response
  if (!response || typeof response !== 'object' || !('data' in response)) return
  const data = response.data
  if (!data || typeof data !== 'object' || !('message' in data)) return
  return typeof data.message === 'string' ? data.message : undefined
}

function uploadFile(
  authorization: UploadAuthorization,
  file: File,
  onProgress: (progress: number) => void
): Promise<void> {
  return new Promise((resolve, reject) => {
    const request = new XMLHttpRequest()
    request.open('PUT', authorization.upload_url)
    Object.entries(authorization.headers).forEach(([key, value]) =>
      request.setRequestHeader(key, value)
    )
    request.upload.addEventListener('progress', (event) => {
      if (event.lengthComputable) {
        onProgress(Math.round((event.loaded / event.total) * 100))
      }
    })
    request.addEventListener('load', () => {
      if (request.status >= 200 && request.status < 300) {
        onProgress(100)
        resolve()
        return
      }
      reject(new Error(`COS upload failed (${request.status})`))
    })
    request.addEventListener('error', () =>
      reject(new Error('COS upload network error'))
    )
    request.send(file)
  })
}

const uploadIcon = { image: Image, video: Video, audio: Music }

export function CreateAssetCard(props: CreateAssetCardProps) {
  const { t } = useTranslation()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [file, setFile] = useState<File | null>(null)
  const [fileType, setFileType] = useState<AssetType>('image')
  const [fileName, setFileName] = useState('')
  const [localPreview, setLocalPreview] = useState('')
  const [uploadProgress, setUploadProgress] = useState(0)
  const [uploadStage, setUploadStage] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [urlName, setURLName] = useState('')
  const [url, setURL] = useState('')
  const [urlType, setURLType] = useState<AssetType>('image')

  useEffect(() => {
    if (!file) {
      setLocalPreview('')
      return
    }
    const objectURL = URL.createObjectURL(file)
    setLocalPreview(objectURL)
    return () => URL.revokeObjectURL(objectURL)
  }, [file])

  const selectFile = (selected: File | undefined) => {
    if (!selected) return
    const inferredType = inferAssetType(selected.name, selected.type)
    if (!inferredType) {
      toast.error(t('Unsupported temporary asset file type'))
      return
    }
    const limit = props.uploadConfig.limits[inferredType]
    if (limit && selected.size > limit) {
      toast.error(
        t('File exceeds the {{size}} limit', {
          size: formatUploadSize(limit),
        })
      )
      return
    }
    setFile(selected)
    setFileType(inferredType)
    setFileName(fileNameWithoutExtension(selected.name).slice(0, 80))
    setUploadProgress(0)
    setUploadStage('')
  }

  const submitFile = async (event: React.FormEvent) => {
    event.preventDefault()
    if (!file || !fileName.trim()) return
    setSubmitting(true)
    try {
      setUploadStage(t('Preparing upload...'))
      const intentResponse = await api.post('/api/assets/self/upload-intent', {
        file_name: file.name,
        content_type: file.type || 'application/octet-stream',
        asset_type: fileType,
        name: fileName.trim(),
        file_size: file.size,
      })
      const authorization = intentResponse.data?.data as UploadAuthorization
      setUploadStage(t('Uploading to COS...'))
      await uploadFile(authorization, file, setUploadProgress)
      setUploadStage(t('Submitting to Molii Volcengine Imagine API...'))
      await api.post('/api/assets/self/upload-complete', {
        upload_id: authorization.upload_id,
      })
      toast.success(t('Temporary asset created'))
      setFile(null)
      setFileName('')
      setUploadProgress(0)
      setUploadStage('')
      if (fileInputRef.current) fileInputRef.current.value = ''
      await props.onCreated()
    } catch (error) {
      toast.error(
        getRequestError(error) || t('Failed to upload temporary asset')
      )
      setUploadStage(t('Upload failed'))
    } finally {
      setSubmitting(false)
    }
  }

  const submitURL = async (event: React.FormEvent) => {
    event.preventDefault()
    setSubmitting(true)
    try {
      await api.post('/api/assets/self', {
        name: urlName,
        url,
        asset_type: urlType,
      })
      setURLName('')
      setURL('')
      toast.success(t('Temporary asset created'))
      await props.onCreated()
    } catch (error) {
      toast.error(
        getRequestError(error) || t('Failed to create temporary asset')
      )
    } finally {
      setSubmitting(false)
    }
  }

  const PreviewIcon = uploadIcon[fileType]

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Create Temporary Asset')}</CardTitle>
        <CardDescription>
          {t(
            'Upload a local file through Molii COS, or keep using a public URL for automated workflows.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <Tabs defaultValue='file'>
          <TabsList aria-label={t('Temporary asset source')}>
            <TabsTrigger value='file'>{t('Upload local file')}</TabsTrigger>
            <TabsTrigger value='url'>{t('Add by URL')}</TabsTrigger>
          </TabsList>
          <TabsContent value='file' className='pt-3'>
            <form className='space-y-4' onSubmit={submitFile}>
              <input
                ref={fileInputRef}
                type='file'
                className='sr-only'
                accept='.jpg,.jpeg,.png,.webp,.bmp,.tif,.tiff,.gif,.mp4,.mov,.wav,.mp3'
                disabled={!props.uploadConfig.enabled || submitting}
                onChange={(event) => selectFile(event.target.files?.[0])}
              />
              <button
                type='button'
                className='border-muted-foreground/30 bg-muted/20 hover:bg-muted/40 focus-visible:ring-ring flex min-h-40 w-full items-center justify-center overflow-hidden rounded-lg border border-dashed transition-colors focus-visible:ring-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-60'
                disabled={!props.uploadConfig.enabled || submitting}
                onClick={() => fileInputRef.current?.click()}
                onDragOver={(event) => event.preventDefault()}
                onDrop={(event) => {
                  event.preventDefault()
                  selectFile(event.dataTransfer.files[0])
                }}
                onPaste={(event) => selectFile(event.clipboardData.files[0])}
              >
                {file && localPreview ? (
                  <div className='flex w-full flex-col items-center gap-3 p-3'>
                    {fileType === 'image' && (
                      <img
                        src={localPreview}
                        alt={file.name}
                        className='max-h-48 max-w-full rounded-md object-contain'
                      />
                    )}
                    {fileType === 'video' && (
                      <video
                        src={localPreview}
                        className='max-h-48 max-w-full rounded-md'
                        controls
                      />
                    )}
                    {fileType === 'audio' && <PreviewIcon className='size-8' />}
                    <span className='text-sm font-medium'>{file.name}</span>
                    <span className='text-muted-foreground text-xs'>
                      {t(getAssetTypeLabel(fileType))} ·{' '}
                      {formatUploadSize(file.size)}
                    </span>
                  </div>
                ) : (
                  <div className='text-muted-foreground flex flex-col items-center gap-2 px-4 text-center'>
                    <FileUp className='size-8' />
                    <span className='text-sm font-medium'>
                      {props.uploadConfig.enabled
                        ? t('Click, drag, or paste a file here')
                        : t('Configure Tencent COS in System Settings first')}
                    </span>
                    <span className='text-xs'>
                      {t(
                        'Images up to 30 MB, videos up to 50 MB, audio up to 15 MB'
                      )}
                    </span>
                  </div>
                )}
              </button>
              <div className='grid gap-3 md:grid-cols-[1fr_auto] md:items-end'>
                <div className='grid gap-2'>
                  <Label htmlFor='cos-asset-name'>{t('Name')}</Label>
                  <Input
                    id='cos-asset-name'
                    value={fileName}
                    onChange={(event) => setFileName(event.target.value)}
                    maxLength={80}
                    disabled={!file || submitting}
                    placeholder={t('A short name visible only to you')}
                  />
                </div>
                <Button
                  type='submit'
                  disabled={
                    !props.uploadConfig.enabled ||
                    !file ||
                    !fileName.trim() ||
                    submitting
                  }
                >
                  {submitting ? t('Uploading...') : t('Upload and create')}
                </Button>
              </div>
              {(uploadStage || uploadProgress > 0) && (
                <Progress value={uploadProgress}>
                  <ProgressLabel>{uploadStage}</ProgressLabel>
                  <span className='text-muted-foreground ml-auto text-sm tabular-nums'>
                    {uploadProgress}%
                  </span>
                </Progress>
              )}
            </form>
          </TabsContent>
          <TabsContent value='url' className='pt-3'>
            <form
              className='grid gap-3 md:grid-cols-[12rem_1fr_1.35fr_auto] md:items-start'
              onSubmit={submitURL}
            >
              <div className='grid gap-2'>
                <Label className='h-4'>{t('Type')}</Label>
                <Select
                  value={urlType}
                  onValueChange={(value) => setURLType(value as AssetType)}
                >
                  <SelectTrigger className='h-9 w-full'>
                    <SelectValue>{t(getAssetTypeLabel(urlType))}</SelectValue>
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      <SelectItem value='image'>{t('Image')}</SelectItem>
                      <SelectItem value='video'>{t('Video')}</SelectItem>
                      <SelectItem value='audio'>{t('Audio')}</SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </div>
              <div className='grid gap-2'>
                <Label className='h-4'>{t('Name')}</Label>
                <Input
                  className='h-9'
                  value={urlName}
                  onChange={(event) => setURLName(event.target.value)}
                  maxLength={80}
                  required
                  placeholder={t('A short name visible only to you')}
                />
              </div>
              <div className='grid gap-2'>
                <Label className='h-4'>{t('Public media URL')}</Label>
                <Input
                  className='h-9'
                  value={url}
                  onChange={(event) => setURL(event.target.value)}
                  required
                  type='url'
                  placeholder='https://...'
                />
              </div>
              <Button
                className='h-9 md:mt-6'
                type='submit'
                disabled={submitting}
              >
                {submitting ? t('Creating...') : t('Create')}
              </Button>
            </form>
          </TabsContent>
        </Tabs>
      </CardContent>
    </Card>
  )
}
