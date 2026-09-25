/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { api } from '@/lib/api'

import type { AssetType, TemporaryAsset } from './asset-utils'

export type UploadAuthorization = {
  upload_id: string
  upload_url: string
  headers: Record<string, string>
}

export function uploadFileToCOS(
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

export async function createTemporaryAssetFromFile(input: {
  file: File
  assetType: AssetType
  name: string
  onProgress: (progress: number) => void
  onUploadComplete?: () => void
}): Promise<TemporaryAsset> {
  const intentResponse = await api.post('/api/assets/self/upload-intent', {
    file_name: input.file.name,
    content_type: input.file.type || 'application/octet-stream',
    asset_type: input.assetType,
    name: input.name,
    file_size: input.file.size,
  })
  const authorization = intentResponse.data?.data as UploadAuthorization
  await uploadFileToCOS(authorization, input.file, input.onProgress)
  input.onUploadComplete?.()
  const completeResponse = await api.post('/api/assets/self/upload-complete', {
    upload_id: authorization.upload_id,
  })
  return completeResponse.data?.data as TemporaryAsset
}
