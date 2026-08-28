/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import type { AssetType } from './asset-utils'

const extensionTypes: Record<string, AssetType> = {
  jpg: 'image',
  jpeg: 'image',
  png: 'image',
  webp: 'image',
  bmp: 'image',
  tif: 'image',
  tiff: 'image',
  gif: 'image',
  mp4: 'video',
  mov: 'video',
  wav: 'audio',
  mp3: 'audio',
}

export function inferAssetType(
  fileName: string,
  contentType: string
): AssetType | undefined {
  const mimeGroup = contentType.toLowerCase().split('/')[0]
  if (mimeGroup === 'image' || mimeGroup === 'video' || mimeGroup === 'audio') {
    return mimeGroup
  }
  const extension = fileName.toLowerCase().split('.').pop() ?? ''
  return extensionTypes[extension]
}

export function fileNameWithoutExtension(fileName: string): string {
  const index = fileName.lastIndexOf('.')
  return index > 0 ? fileName.slice(0, index) : fileName
}

export function formatUploadSize(bytes: number): string {
  if (bytes < 1024 * 1024) return `${Math.max(1, Math.round(bytes / 1024))} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}
