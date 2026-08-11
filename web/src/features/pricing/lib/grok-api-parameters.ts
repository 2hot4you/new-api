/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import type { GrokOperation } from './grok-api-sample'
import type { SupportedParameter } from './mock-stats'

const IMAGE_ASPECT_RATIOS = [
  '1:1',
  '16:9',
  '9:16',
  '4:3',
  '3:4',
  '3:2',
  '2:3',
  '2:1',
  '1:2',
  '19.5:9',
  '9:19.5',
  '20:9',
  '9:20',
  'auto',
]

const VIDEO_ASPECT_RATIOS = ['1:1', '16:9', '9:16', '4:3', '3:4', '3:2', '2:3']

function modelParameter(modelName: string): SupportedParameter {
  return {
    name: 'model',
    type: 'enum',
    enumValues: [modelName],
    descriptionKey: 'Grok model ID selected for this request',
    required: true,
  }
}

const requiredImagePrompt: SupportedParameter = {
  name: 'prompt',
  type: 'string',
  range: '1–10,000 characters',
  descriptionKey: 'Text instruction for image generation or editing',
  required: true,
}

const requiredVideoPrompt: SupportedParameter = {
  name: 'prompt',
  type: 'string',
  range: '1–10,000 characters',
  descriptionKey: 'Text instruction for video generation or editing',
  required: true,
}

const imageAspectRatio: SupportedParameter = {
  name: 'aspect_ratio',
  type: 'enum',
  defaultValue: '16:9',
  enumValues: IMAGE_ASPECT_RATIOS,
  descriptionKey: 'Aspect ratio of each generated image',
}

const imageResolution: SupportedParameter = {
  name: 'resolution',
  type: 'enum',
  defaultValue: '1k',
  enumValues: ['1k', '2k'],
  descriptionKey: 'Resolution of each generated image',
}

const imageOutputCount: SupportedParameter = {
  name: 'n',
  type: 'integer',
  defaultValue: 1,
  range: '1–4',
  descriptionKey: 'Number of images to generate',
}

function imageParameters(
  modelName: string,
  operation: GrokOperation
): SupportedParameter[] {
  const mediaParameters: SupportedParameter[] =
    operation === 'edit'
      ? [
          {
            name: 'image',
            type: 'object',
            range: '1–3 input images in total',
            descriptionKey:
              'Single input image URL or file_id; provide image or images',
          },
          {
            name: 'images',
            type: 'array',
            range: '1–3 input images in total',
            descriptionKey:
              'Multiple input image URLs or file_id values; provide image or images',
          },
        ]
      : []

  return [
    modelParameter(modelName),
    requiredImagePrompt,
    ...mediaParameters,
    imageAspectRatio,
    imageResolution,
    imageOutputCount,
  ]
}

function videoGenerationParameters(modelName: string): SupportedParameter[] {
  const imageRequired = modelName === 'grok-imagine-video-1.5'
  const resolutions = imageRequired
    ? ['480p', '720p', '1080p']
    : ['480p', '720p']

  return [
    modelParameter(modelName),
    requiredVideoPrompt,
    {
      name: 'image',
      type: 'object',
      descriptionKey: imageRequired
        ? 'Input image URL or file_id; required by grok-imagine-video-1.5'
        : 'Optional input image URL or file_id for image-to-video generation',
      required: imageRequired,
    },
    {
      name: 'duration',
      type: 'integer',
      defaultValue: 5,
      range: '1–15 seconds',
      descriptionKey: 'Requested output video duration in seconds',
    },
    {
      name: 'aspect_ratio',
      type: 'enum',
      defaultValue: '16:9',
      enumValues: VIDEO_ASPECT_RATIOS,
      descriptionKey: 'Requested output video aspect ratio',
    },
    {
      name: 'resolution',
      type: 'enum',
      defaultValue: '480p',
      enumValues: resolutions,
      descriptionKey: 'Requested output video resolution',
    },
  ]
}

function videoEditParameters(modelName: string): SupportedParameter[] {
  return [
    modelParameter(modelName),
    requiredVideoPrompt,
    {
      name: 'video',
      type: 'object',
      descriptionKey: 'Input video URL or file_id to edit',
      required: true,
    },
  ]
}

function videoExtensionParameters(modelName: string): SupportedParameter[] {
  return [
    modelParameter(modelName),
    requiredVideoPrompt,
    {
      name: 'video',
      type: 'object',
      descriptionKey: 'Input video URL or file_id to extend; input must be 2–15 seconds',
      required: true,
    },
    {
      name: 'duration',
      type: 'integer',
      defaultValue: 6,
      range: '2–10 seconds',
      descriptionKey: 'Additional output duration in seconds',
    },
  ]
}

function referenceVideoParameters(modelName: string): SupportedParameter[] {
  return [
    modelParameter(modelName),
    requiredVideoPrompt,
    {
      name: 'reference_images',
      type: 'array',
      range: '1–7 images',
      descriptionKey: 'Reference image URLs or file_id values; cannot combine with image or video',
      required: true,
    },
    {
      name: 'duration',
      type: 'integer',
      defaultValue: 5,
      range: '1–15 seconds',
      descriptionKey: 'Requested output video duration in seconds',
    },
    {
      name: 'aspect_ratio',
      type: 'enum',
      defaultValue: '16:9',
      enumValues: VIDEO_ASPECT_RATIOS,
      descriptionKey: 'Requested output video aspect ratio',
    },
    {
      name: 'resolution',
      type: 'enum',
      defaultValue: '480p',
      enumValues: ['480p', '720p'],
      descriptionKey: 'Reference-to-video output resolution',
    },
  ]
}

const taskIdParameter: SupportedParameter = {
  name: 'task_id',
  type: 'string',
  descriptionKey: 'Molii public task ID returned by the creation request',
  required: true,
}

export function buildGrokApiParameters(
  modelName: string,
  operation: GrokOperation
): SupportedParameter[] {
  if (operation === 'status' || operation === 'download') {
    return [taskIdParameter]
  }
  if (modelName.includes('-image')) {
    return imageParameters(modelName, operation)
  }
  if (operation === 'edit') {
    return videoEditParameters(modelName)
  }
  if (operation === 'extend') {
    return videoExtensionParameters(modelName)
  }
  if (operation === 'reference') {
    return referenceVideoParameters(modelName)
  }
  return videoGenerationParameters(modelName)
}
