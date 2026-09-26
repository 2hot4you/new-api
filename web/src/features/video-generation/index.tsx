/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { RefreshCw, Send, Video } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import type { COSUploadConfig } from '@/features/temporary-assets/components/create-asset-card'
import type { TemporaryAsset } from '@/features/temporary-assets/lib/asset-utils'
import { api } from '@/lib/api'
import { formatQuotaWithCurrency } from '@/lib/currency'

import {
  estimateVideoStudioTask,
  getVideoStudioOptions,
  getVideoStudioTask,
  getVideoStudioTasks,
  submitVideoStudioTask,
} from './api'
import { VideoStudioCurrentPreview } from './components/current-video-preview'
import { VideoStudioMediaPicker } from './components/media-picker'
import { VideoStudioPaidRequestDialog } from './components/paid-request-dialog'
import { VideoStudioPromptComposer } from './components/prompt-composer'
import { VideoStudioTaskHistory } from './components/task-history'
import {
  chooseDefaultVideoStudioTokenID,
  findLatestActiveVideoStudioTaskID,
  hasUnavailableSelectedAsset,
  stripVideoStudioAssetMentions,
} from './lib/authoring'
import { videoStudioFormSchema } from './lib/form-schema'
import { buildSeedancePayload, validateMediaSelection } from './lib/payload'
import type {
  VideoStudioFormValues,
  VideoStudioEstimate,
  VideoStudioMedia,
  VideoStudioMode,
  SeedancePayload,
  VideoStudioTask,
} from './types'

const optionsQueryKey = ['video-studio', 'options'] as const
const tasksQueryKey = ['video-studio', 'tasks'] as const
const assetsQueryKey = ['video-studio', 'assets'] as const

const videoStudioModeLabels: Record<VideoStudioMode, string> = {
  text: 'Text to video',
  frames: 'First / last frame',
  references: 'Multimodal references',
}

function requestErrorMessage(error: unknown): string | undefined {
  if (!error || typeof error !== 'object' || !('response' in error)) return
  const response = error.response
  if (!response || typeof response !== 'object' || !('data' in response)) return
  const data = response.data
  if (!data || typeof data !== 'object') return
  if (
    'error' in data &&
    data.error &&
    typeof data.error === 'object' &&
    'message' in data.error
  ) {
    return typeof data.error.message === 'string'
      ? data.error.message
      : undefined
  }
  return 'message' in data && typeof data.message === 'string'
    ? data.message
    : undefined
}

function mediaValidationMessage(reason: string): string {
  const messages: Record<string, string> = {
    frame_required: 'Frame mode requires exactly one first-frame image.',
    frame_count:
      'Frame mode supports one first frame and one optional last frame.',
    text_media_not_allowed: 'Text mode does not include reference media.',
    reference_required:
      'Add at least one reference image, video, or audio file.',
    audio_requires_visual:
      'Reference audio requires at least one image or video.',
    image_limit: 'Too many reference images for this model.',
    video_limit: 'Too many reference videos for this model.',
    audio_limit: 'Too many reference audio files for this model.',
  }
  return messages[reason] ?? 'Invalid media selection.'
}

function requestMode(task: VideoStudioTask): VideoStudioMode {
  const media = task.request?.media ?? []
  if (
    media.some((entry) => ['first_frame', 'last_frame'].includes(entry.role))
  ) {
    return 'frames'
  }
  return media.length > 0 ? 'references' : 'text'
}

export function VideoGenerationStudio() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [media, setMedia] = useState<VideoStudioMedia[]>([])
  const [pageIndex, setPageIndex] = useState(0)
  const [pageSize, setPageSize] = useState(20)
  const [currentTaskID, setCurrentTaskID] = useState('')
  const [pendingRequest, setPendingRequest] = useState<{
    values: VideoStudioFormValues
    payload: SeedancePayload
    estimate: VideoStudioEstimate
  } | null>(null)
  const optionsQuery = useQuery({
    queryKey: optionsQueryKey,
    queryFn: getVideoStudioOptions,
  })
  const assetsQuery = useQuery({
    queryKey: assetsQueryKey,
    queryFn: async () => {
      const response = await api.get('/api/assets/self')
      return (response.data?.data ?? []) as TemporaryAsset[]
    },
    refetchInterval: (query) =>
      query.state.data?.some(
        (item) =>
          !['ACTIVE', 'SUCCESS', 'FAILED', 'EXPIRED'].includes(
            item.status.toUpperCase()
          )
      )
        ? 8_000
        : false,
  })
  const uploadConfigQuery = useQuery({
    queryKey: ['video-studio', 'upload-config'],
    queryFn: async () => {
      const response = await api.get('/api/assets/self/upload-config')
      return response.data?.data as COSUploadConfig
    },
  })
  const tasksQuery = useQuery({
    queryKey: [...tasksQueryKey, pageIndex, pageSize],
    queryFn: () => getVideoStudioTasks({ page: pageIndex + 1, pageSize }),
    placeholderData: (previousData) => previousData,
  })
  const currentTaskQuery = useQuery({
    queryKey: ['video-studio', 'task', currentTaskID],
    queryFn: () => getVideoStudioTask(currentTaskID),
    enabled: Boolean(currentTaskID),
    refetchInterval: (query) => {
      const status = query.state.data?.task.status
      return status && !['SUCCESS', 'FAILURE'].includes(status) ? 5_000 : false
    },
  })
  const form = useForm<VideoStudioFormValues>({
    resolver: zodResolver(videoStudioFormSchema),
    defaultValues: {
      tokenId: '',
      model: '',
      prompt: '',
      mode: 'text',
      resolution: '720p',
      ratio: '16:9',
      duration: 6,
      generateAudio: true,
      watermark: false,
      webSearch: false,
    },
  })
  const tokenID = form.watch('tokenId')
  const model = form.watch('model')
  const mode = form.watch('mode')
  const selectedToken = optionsQuery.data?.tokens.find(
    (token) => String(token.id) === tokenID
  )
  const capability = model ? optionsQuery.data?.capabilities[model] : undefined
  const durationOptions = useMemo(
    () => [
      ...(capability?.supports_auto_duration ? [-1] : []),
      ...Array.from(
        { length: Math.max(0, (capability?.max_duration ?? 3) - 3) },
        (_, index) => index + 4
      ),
    ],
    [capability?.max_duration, capability?.supports_auto_duration]
  )

  useEffect(() => {
    const nextTokenID = chooseDefaultVideoStudioTokenID(
      form.getValues('tokenId'),
      optionsQuery.data?.tokens ?? []
    )
    if (nextTokenID && nextTokenID !== form.getValues('tokenId')) {
      form.setValue('tokenId', nextTokenID)
    }
  }, [form, optionsQuery.data?.tokens])

  useEffect(() => {
    if (currentTaskID || !tasksQuery.data) return
    const activeTaskID = findLatestActiveVideoStudioTaskID(
      tasksQuery.data.items
    )
    if (activeTaskID) setCurrentTaskID(activeTaskID)
  }, [currentTaskID, tasksQuery.data])

  useEffect(() => {
    const status = currentTaskQuery.data?.task.status
    if (!status || !['SUCCESS', 'FAILURE'].includes(status)) return
    void queryClient.invalidateQueries({ queryKey: tasksQueryKey })
  }, [currentTaskQuery.data?.task.status, queryClient])

  useEffect(() => {
    if (!selectedToken) {
      form.setValue('model', '')
      return
    }
    if (!selectedToken.available_models.includes(form.getValues('model'))) {
      form.setValue('model', selectedToken.available_models[0] ?? '')
    }
  }, [form, selectedToken])

  useEffect(() => {
    if (!capability) return
    if (!capability.resolutions.includes(form.getValues('resolution'))) {
      form.setValue('resolution', capability.resolutions[0] ?? '720p')
    }
    const duration = form.getValues('duration')
    if (duration === -1 && !capability.supports_auto_duration) {
      form.setValue('duration', Math.min(6, capability.max_duration))
    } else if (duration > capability.max_duration) {
      form.setValue('duration', capability.max_duration)
    }
  }, [capability, form])

  const selectedAssetUnavailable = useMemo(() => {
    return hasUnavailableSelectedAsset(media, assetsQuery.data ?? [])
  }, [assetsQuery.data, media])

  const preparePayload = (values: VideoStudioFormValues) => {
    if (!capability) throw new Error(t('Select a supported model'))
    if (!values.prompt.trim() && values.mode === 'text') {
      throw new Error(t('Enter a prompt for text-to-video generation'))
    }
    const mediaResult = validateMediaSelection(values.mode, media, capability)
    if (!mediaResult.valid) {
      throw new Error(t(mediaValidationMessage(mediaResult.reason)))
    }
    if (hasUnavailableSelectedAsset(media, assetsQuery.data ?? [])) {
      throw new Error(
        t('Wait for selected assets to become available or replace them.')
      )
    }
    return buildSeedancePayload({ ...values, media })
  }

  const estimate = useMutation({
    mutationFn: async (values: VideoStudioFormValues) => {
      const payload = preparePayload(values)
      const quote = await estimateVideoStudioTask({
        tokenId: Number(values.tokenId),
        payload,
      })
      return { values, payload, estimate: quote }
    },
    onSuccess: setPendingRequest,
    onError: (error) => {
      toast.error(
        requestErrorMessage(error) ||
          (error instanceof Error
            ? error.message
            : t('Failed to estimate video generation cost'))
      )
    },
  })

  const submit = useMutation({
    mutationFn: async (request: {
      values: VideoStudioFormValues
      payload: SeedancePayload
    }) => {
      return submitVideoStudioTask({
        tokenId: Number(request.values.tokenId),
        requestId: crypto.randomUUID(),
        payload: request.payload,
      })
    },
    onSuccess: async (result) => {
      setPendingRequest(null)
      toast.success(t('Video generation task submitted'))
      const submittedTaskID = result.task_id || result.id
      if (submittedTaskID) setCurrentTaskID(submittedTaskID)
      await queryClient.invalidateQueries({ queryKey: tasksQueryKey })
    },
    onError: (error) => {
      toast.error(
        requestErrorMessage(error) ||
          (error instanceof Error
            ? error.message
            : t('Failed to submit video generation task'))
      )
    },
  })

  const changeMode = (value: VideoStudioMode | null) => {
    if (!value) return
    const previousMode = form.getValues('mode')
    form.setValue('mode', value as VideoStudioMode)
    setMedia([])
    if (previousMode === 'references' && value !== 'references') {
      form.setValue(
        'prompt',
        stripVideoStudioAssetMentions(form.getValues('prompt'))
      )
    }
    if (value === 'frames') form.setValue('ratio', 'adaptive')
  }

  const reuseTask = (item: VideoStudioTask) => {
    const request = item.request
    if (!request) return
    const nextMode = requestMode(item)
    const compatibleToken = optionsQuery.data?.tokens.find((token) =>
      token.available_models.includes(request.model)
    )
    form.reset({
      tokenId: compatibleToken ? String(compatibleToken.id) : '',
      model: request.model,
      prompt: request.prompt ?? '',
      mode: nextMode,
      resolution: request.resolution,
      ratio: request.ratio,
      duration: request.duration,
      generateAudio: request.generate_audio,
      watermark: request.watermark,
      webSearch: request.web_search,
    })
    const mentionCounts = { image: 0, video: 0, audio: 0 }
    setMedia(
      (request.media ?? []).map((entry) => ({
        clientId: crypto.randomUUID(),
        type: entry.type,
        source: entry.source,
        role: entry.role,
        value:
          entry.source === 'asset' ? (entry.asset_id ?? '') : (entry.url ?? ''),
        name: entry.name || entry.asset_id || entry.url || t('Reference media'),
        expiresAt: entry.expires_at,
        mentionIndex: entry.role.startsWith('reference_')
          ? ++mentionCounts[entry.type]
          : undefined,
      }))
    )
    toast.success(t('Generation settings restored'))
  }

  const currentTask =
    currentTaskQuery.data ??
    tasksQuery.data?.items.find((item) => item.task.task_id === currentTaskID)

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('Seedance Video Studio')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          variant='outline'
          disabled={tasksQuery.isFetching || currentTaskQuery.isFetching}
          onClick={() => {
            void tasksQuery.refetch()
            if (currentTaskID) void currentTaskQuery.refetch()
          }}
        >
          <RefreshCw
            className={
              tasksQuery.isFetching || currentTaskQuery.isFetching
                ? 'animate-spin'
                : ''
            }
          />
          {t('Refresh tasks')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='grid items-stretch gap-4 xl:grid-cols-[minmax(420px,0.9fr)_minmax(520px,1.1fr)]'>
          <Card className='h-full'>
            <CardHeader>
              <CardTitle className='flex items-center gap-2'>
                <Video className='size-5' />
                {t('Create Seedance video')}
              </CardTitle>
              <CardDescription>
                {t(
                  'Choose one of your compatible API keys, configure the request, and submit it without exposing the key in the browser.'
                )}
              </CardDescription>
            </CardHeader>
            <CardContent>
              {optionsQuery.isError && (
                <Alert variant='destructive' className='mb-4'>
                  <AlertTitle>
                    {t('Unable to load video generation options')}
                  </AlertTitle>
                  <AlertDescription>
                    {t('Refresh the page and try again.')}
                  </AlertDescription>
                </Alert>
              )}
              {!optionsQuery.isLoading &&
                optionsQuery.data?.tokens.length === 0 && (
                  <Alert className='mb-4'>
                    <AlertTitle>{t('No compatible API key')}</AlertTitle>
                    <AlertDescription>
                      {t(
                        'Create or update an API key with access to a configured Seedance model first.'
                      )}
                    </AlertDescription>
                  </Alert>
                )}
              <form
                className='space-y-5'
                onSubmit={form.handleSubmit((values) =>
                  estimate.mutate(values)
                )}
              >
                <div className='grid gap-4 sm:grid-cols-2'>
                  <Controller
                    control={form.control}
                    name='tokenId'
                    render={({ field }) => (
                      <div className='space-y-1.5'>
                        <Label>{t('API Key')}</Label>
                        <Select
                          value={field.value}
                          onValueChange={field.onChange}
                        >
                          <SelectTrigger
                            className='w-full'
                            aria-label={t('API Key')}
                            aria-invalid={Boolean(
                              form.formState.errors.tokenId
                            )}
                          >
                            <SelectValue placeholder={t('Select an API key')}>
                              {selectedToken?.masked_key}
                            </SelectValue>
                          </SelectTrigger>
                          <SelectContent>
                            {optionsQuery.data?.tokens.map((token) => (
                              <SelectItem
                                key={token.id}
                                value={String(token.id)}
                              >
                                {token.masked_key}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                    )}
                  />
                  <Controller
                    control={form.control}
                    name='model'
                    render={({ field }) => (
                      <div className='space-y-1.5'>
                        <Label>{t('Model')}</Label>
                        <Select
                          value={field.value}
                          onValueChange={field.onChange}
                          disabled={!selectedToken}
                        >
                          <SelectTrigger
                            className='w-full'
                            aria-invalid={Boolean(form.formState.errors.model)}
                          >
                            <SelectValue placeholder={t('Select a model')} />
                          </SelectTrigger>
                          <SelectContent>
                            {selectedToken?.available_models.map((name) => (
                              <SelectItem key={name} value={name}>
                                {name}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                    )}
                  />
                </div>

                <div className='space-y-1.5'>
                  <Label htmlFor='video-studio-prompt'>{t('Prompt')}</Label>
                  <Controller
                    control={form.control}
                    name='prompt'
                    render={({ field }) => (
                      <VideoStudioPromptComposer
                        value={field.value}
                        mode={mode}
                        media={media}
                        assets={assetsQuery.data ?? []}
                        onChange={field.onChange}
                        onMediaChange={setMedia}
                      />
                    )}
                  />
                </div>

                <Controller
                  control={form.control}
                  name='mode'
                  render={({ field }) => (
                    <div className='space-y-1.5'>
                      <Label>{t('Generation mode')}</Label>
                      <Select value={field.value} onValueChange={changeMode}>
                        <SelectTrigger
                          className='w-full'
                          aria-label={t('Generation mode')}
                        >
                          <SelectValue>
                            {t(videoStudioModeLabels[mode])}
                          </SelectValue>
                        </SelectTrigger>
                        <SelectContent>
                          {Object.entries(videoStudioModeLabels).map(
                            ([value, label]) => (
                              <SelectItem key={value} value={value}>
                                {t(label)}
                              </SelectItem>
                            )
                          )}
                        </SelectContent>
                      </Select>
                    </div>
                  )}
                />

                <VideoStudioMediaPicker
                  mode={mode}
                  media={media}
                  assets={assetsQuery.data ?? []}
                  uploadConfig={uploadConfigQuery.data}
                  onChange={setMedia}
                  onAssetsChanged={() =>
                    queryClient.invalidateQueries({ queryKey: assetsQueryKey })
                  }
                />

                <div className='grid gap-4 sm:grid-cols-3'>
                  <Controller
                    control={form.control}
                    name='resolution'
                    render={({ field }) => (
                      <div className='space-y-1.5'>
                        <Label>{t('Resolution')}</Label>
                        <Select
                          value={field.value}
                          onValueChange={field.onChange}
                        >
                          <SelectTrigger className='w-full'>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {capability?.resolutions.map((value) => (
                              <SelectItem key={value} value={value}>
                                {value}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                    )}
                  />
                  <Controller
                    control={form.control}
                    name='ratio'
                    render={({ field }) => (
                      <div className='space-y-1.5'>
                        <Label>{t('Aspect ratio')}</Label>
                        <Select
                          value={mode === 'frames' ? 'adaptive' : field.value}
                          onValueChange={field.onChange}
                          disabled={mode === 'frames'}
                        >
                          <SelectTrigger
                            className='w-full'
                            aria-label={t('Aspect ratio')}
                          >
                            <SelectValue>
                              {field.value === 'adaptive'
                                ? t('Auto')
                                : field.value}
                            </SelectValue>
                          </SelectTrigger>
                          <SelectContent>
                            {capability?.ratios.map((value) => (
                              <SelectItem key={value} value={value}>
                                {value === 'adaptive' ? t('Auto') : value}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                    )}
                  />
                  <Controller
                    control={form.control}
                    name='duration'
                    render={({ field }) => (
                      <div className='space-y-1.5'>
                        <Label>{t('Duration (seconds)')}</Label>
                        <Select
                          value={String(field.value)}
                          onValueChange={(value) =>
                            field.onChange(Number(value))
                          }
                        >
                          <SelectTrigger
                            className='w-full'
                            aria-label={t('Duration (seconds)')}
                          >
                            <SelectValue>
                              {field.value === -1
                                ? t('Auto')
                                : `${field.value} ${t('seconds')}`}
                            </SelectValue>
                          </SelectTrigger>
                          <SelectContent>
                            {durationOptions.map((value) => (
                              <SelectItem key={value} value={String(value)}>
                                {value === -1
                                  ? t('Auto')
                                  : `${value} ${t('seconds')}`}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                    )}
                  />
                </div>

                <div className='grid gap-3 sm:grid-cols-3'>
                  {(
                    [
                      ['generateAudio', 'Generate audio'],
                      ['watermark', 'Watermark'],
                      ['webSearch', 'Web search'],
                    ] as const
                  ).map(([name, label]) => (
                    <Controller
                      key={name}
                      control={form.control}
                      name={name}
                      render={({ field }) => (
                        <Label className='flex items-center justify-between gap-3 rounded-lg border p-3'>
                          <span>{t(label)}</span>
                          <Switch
                            checked={field.value}
                            onCheckedChange={field.onChange}
                          />
                        </Label>
                      )}
                    />
                  ))}
                </div>

                {selectedAssetUnavailable && (
                  <p className='text-destructive text-sm'>
                    {t(
                      'A selected temporary asset is not ready or has expired.'
                    )}
                  </p>
                )}
                <Button
                  className='w-full'
                  type='submit'
                  disabled={
                    estimate.isPending ||
                    submit.isPending ||
                    !selectedToken ||
                    !capability ||
                    selectedAssetUnavailable
                  }
                >
                  <Send />
                  {estimate.isPending
                    ? t('Calculating estimated cost...')
                    : t('Generate video')}
                </Button>
              </form>
            </CardContent>
          </Card>

          <section
            className='h-full'
            aria-label={t('Video generation results')}
          >
            <VideoStudioCurrentPreview
              task={currentTask}
              loading={tasksQuery.isLoading || currentTaskQuery.isLoading}
              onReuse={reuseTask}
            />
          </section>
        </div>

        <section className='mt-4' aria-label={t('Generation history')}>
          <VideoStudioTaskHistory
            items={tasksQuery.data?.items ?? []}
            total={tasksQuery.data?.total ?? 0}
            pageIndex={pageIndex}
            pageSize={pageSize}
            loading={tasksQuery.isLoading}
            onPageChange={setPageIndex}
            onPageSizeChange={(size) => {
              setPageIndex(0)
              setPageSize(size)
            }}
            onPreview={(item) => setCurrentTaskID(item.task.task_id)}
            onReuse={reuseTask}
          />
        </section>
        <VideoStudioPaidRequestDialog
          open={Boolean(pendingRequest)}
          estimatedCost={formatQuotaWithCurrency(
            pendingRequest?.estimate.quota
          )}
          estimatedTokens={pendingRequest?.estimate.estimated_tokens}
          upperBound={
            pendingRequest?.values.duration === -1 &&
            (pendingRequest.estimate.estimated_tokens ?? 0) > 0
          }
          pending={submit.isPending}
          onOpenChange={(open) => {
            if (!open && !submit.isPending) setPendingRequest(null)
          }}
          onConfirm={() => {
            if (!pendingRequest) return
            submit.mutate({
              values: pendingRequest.values,
              payload: pendingRequest.payload,
            })
          }}
        />
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
