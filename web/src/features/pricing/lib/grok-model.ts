import type { PricingModel } from '../types'

export const GROK_IMAGE_MODELS = [
  'grok-imagine-image',
  'grok-imagine-image-quality',
  'grok-imagine-image-2.0',
] as const

export const GROK_VIDEO_MODELS = [
  'grok-imagine-video',
  'grok-imagine-video-1.5',
] as const

export function isGrokImagineModel(model: PricingModel | string): boolean {
  const name = typeof model === 'string' ? model : model.model_name
  return [...GROK_IMAGE_MODELS, ...GROK_VIDEO_MODELS].includes(name as never)
}

export function isGrokImageModel(model: PricingModel | string): boolean {
  const name = typeof model === 'string' ? model : model.model_name
  return GROK_IMAGE_MODELS.includes(name as never)
}

export function isGrokVideoModel(model: PricingModel | string): boolean {
  const name = typeof model === 'string' ? model : model.model_name
  return GROK_VIDEO_MODELS.includes(name as never)
}

export function getGrokModelCapabilities(modelName: string) {
  switch (modelName) {
    case 'grok-imagine-image':
    case 'grok-imagine-image-quality':
    case 'grok-imagine-image-2.0':
      return {
        input: ['Text', 'Image'],
        output: 'Image',
        resolutions: ['1K', '2K'],
        operations: ['Image generation', 'Image editing'],
      }
    case 'grok-imagine-video-1.5':
      return {
        input: ['Text', 'Image'],
        output: 'Video',
        resolutions: ['480p', '720p', '1080p'],
        operations: ['Image-to-video', 'Reference-to-video'],
      }
    default:
      return {
        input: ['Text', 'Image', 'Video'],
        output: 'Video',
        resolutions: ['480p', '720p'],
        operations: ['Video generation', 'Video editing', 'Video extension'],
      }
  }
}
