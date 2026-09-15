/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { ExternalLink, Image, Music, Video } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'

import type { TemporaryAsset } from '../lib/asset-utils'

const assetIcon = { image: Image, video: Video, audio: Music }

type RefreshPreview = () => Promise<string | undefined>

function useRefreshablePreviewURL(
  item: TemporaryAsset,
  onRefreshPreview?: RefreshPreview
) {
  const sourceURL = item.preview_url || item.source_url
  const [previewURL, setPreviewURL] = useState(sourceURL)
  const [failed, setFailed] = useState(false)
  const retryingRef = useRef(false)
  const retriedRef = useRef(false)

  useEffect(() => {
    setPreviewURL(sourceURL)
    setFailed(false)
  }, [sourceURL])

  const handleReady = () => {
    retriedRef.current = false
    setFailed(false)
  }

  const handleError = async () => {
    if (retryingRef.current) return
    if (item.source_kind !== 'cos' || retriedRef.current || !onRefreshPreview) {
      setFailed(true)
      return
    }
    retryingRef.current = true
    retriedRef.current = true
    try {
      const refreshedURL = await onRefreshPreview()
      if (!refreshedURL || refreshedURL === previewURL) {
        setFailed(true)
        return
      }
      setPreviewURL(refreshedURL)
      setFailed(false)
    } catch {
      setFailed(true)
    } finally {
      retryingRef.current = false
    }
  }

  return { failed, handleError, handleReady, previewURL }
}

export function AssetPreview(props: {
  item: TemporaryAsset
  alt: string
  missingLabel: string
  failedLabel: string
  onRefreshPreview?: RefreshPreview
}) {
  const { failed, handleError, handleReady, previewURL } =
    useRefreshablePreviewURL(props.item, props.onRefreshPreview)
  const Icon = assetIcon[props.item.asset_type] ?? Image

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
        onLoad={handleReady}
        onError={() => void handleError()}
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
        onLoadedMetadata={handleReady}
        onError={() => void handleError()}
      />
    )
  }
  return <Music className='size-5' />
}

export function AssetPreviewLink(props: {
  item: TemporaryAsset
  label: string
  onRefreshPreview?: RefreshPreview
}) {
  const previewURL = props.item.preview_url || props.item.source_url
  if (!previewURL) return null
  return (
    <a
      href={previewURL}
      target='_blank'
      rel='noreferrer'
      className='text-muted-foreground hover:text-foreground flex min-w-0 items-center gap-1 text-xs'
      onClick={(event) => {
        if (props.item.source_kind !== 'cos' || !props.onRefreshPreview) {
          return
        }
        event.preventDefault()
        const previewWindow = window.open('about:blank', '_blank')
        if (!previewWindow) return
        previewWindow.opener = null
        void props
          .onRefreshPreview()
          .then((refreshedURL) => {
            if (!refreshedURL) {
              previewWindow.close()
              return
            }
            previewWindow.location.replace(refreshedURL)
          })
          .catch(() => previewWindow.close())
      }}
    >
      <ExternalLink className='size-3 shrink-0' />
      <span className='truncate'>{props.label}</span>
    </a>
  )
}

export function AssetAudioPreview(props: {
  item: TemporaryAsset
  onRefreshPreview?: RefreshPreview
}) {
  const { handleError, handleReady, previewURL } = useRefreshablePreviewURL(
    props.item,
    props.onRefreshPreview
  )
  if (!previewURL || props.item.asset_type !== 'audio') return null
  return (
    <audio
      src={previewURL}
      controls
      preload='none'
      className='h-8 w-full'
      onCanPlay={handleReady}
      onError={() => void handleError()}
    />
  )
}
