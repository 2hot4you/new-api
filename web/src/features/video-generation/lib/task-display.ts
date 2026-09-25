/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import type { VideoStudioTask } from '../types'

export function taskElapsedSeconds(item: VideoStudioTask): number | null {
  const start = item.task.submit_time
  const finish = item.task.finish_time
  if (!start || !finish || finish < start) return null
  return finish - start
}

export function formatElapsedTime(seconds: number | null): string {
  if (seconds === null) return '—'
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.floor(seconds / 60)
  const remaining = seconds % 60
  return remaining === 0 ? `${minutes}m` : `${minutes}m ${remaining}s`
}

export function taskProgress(item: VideoStudioTask): number | null {
  const match = item.task.progress?.match(/(\d+(?:\.\d+)?)%/)
  if (!match) return null
  const value = Number(match[1])
  if (!Number.isFinite(value)) return null
  return Math.max(0, Math.min(100, value))
}

export function taskModel(item: VideoStudioTask): string {
  return (
    item.request?.model ||
    item.task.properties?.origin_model_name ||
    item.task.properties?.upstream_model_name ||
    '—'
  )
}

export function taskMediaCounts(item: VideoStudioTask): string {
  const params = item.task.video_params
  return [
    params?.input_image_count ?? 0,
    params?.input_video_count ?? 0,
    params?.input_audio_count ?? 0,
  ].join(' / ')
}

export function taskDimensions(item: VideoStudioTask): string {
  const { width, height } = item.task.video_params ?? {}
  return width && height ? `${width}×${height}` : '—'
}

export function taskBooleanLabel(value: boolean | undefined): string {
  if (value === undefined) return '—'
  return value ? 'Yes' : 'No'
}
