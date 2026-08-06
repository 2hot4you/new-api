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

export type GenerationLogSection = 'drawing' | 'task'
export type UsageLogSource = 'grok-image' | 'grok-video' | 'seedance'
export type VideoLogSource = Extract<UsageLogSource, 'grok-video' | 'seedance'>

export const GENERATION_LOG_META = {
  titleKey: 'Generation Records',
  sections: {
    drawing: { labelKey: 'Image Generation' },
    task: { labelKey: 'Video Generation' },
  },
} as const

export const GENERATION_LOG_SOURCES = {
  drawing: [{ id: 'grok-image', labelKey: 'Grok Image' }],
  task: [
    { id: 'grok-video', labelKey: 'Grok Video', platform: '62' },
    { id: 'seedance', labelKey: 'Seedance', platform: '61' },
  ],
} as const

const DEFAULT_SOURCE: Record<GenerationLogSection, UsageLogSource> = {
  drawing: 'grok-image',
  task: 'grok-video',
}

export function resolveUsageLogSource(
  section: GenerationLogSection,
  source?: string
): UsageLogSource {
  const sources = GENERATION_LOG_SOURCES[section] as readonly {
    id: UsageLogSource
  }[]
  return sources.some((item) => item.id === source)
    ? (source as UsageLogSource)
    : DEFAULT_SOURCE[section]
}

export function resolveVideoLogSource(source?: string): VideoLogSource {
  const resolved = resolveUsageLogSource('task', source)
  return resolved === 'seedance' ? 'seedance' : 'grok-video'
}

export function getVideoPlatformForSource(
  source: UsageLogSource
): string | undefined {
  return GENERATION_LOG_SOURCES.task.find((item) => item.id === source)
    ?.platform
}
