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
import assert from 'node:assert/strict'
import { after, describe, test } from 'node:test'

import { Window } from 'happy-dom'

import { TASK_PLATFORMS, TASK_STATUS } from '../../constants'
import { taskActionMapper, taskPlatformMapper } from '../mappers'
import {
  canPreviewVideoTask,
  isGeneratedVideoTask,
  shouldShowGrokVideoTemporaryLinkWarning,
} from '../task-video-preview'

const domWindow = new Window()
domWindow.document.write('<!doctype html><html><body></body></html>')
for (const key of [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MutationObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
  'matchMedia',
  'customElements',
] as const) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const { act, createElement } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { VideoPreviewDialog } =
  await import('../../components/dialogs/video-preview-dialog')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en', resources: {} })

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

async function renderVideoPreview(platform: string) {
  const host = document.createElement('div')
  document.body.append(host)
  const root = createRoot(host)
  await act(async () => {
    root.render(
      createElement(
        I18nextProvider,
        { i18n },
        createElement(VideoPreviewDialog, {
          open: true,
          onOpenChange: () => undefined,
          log: {
            id: 1,
            user_id: 1,
            platform,
            task_id: 'task_public_preview',
            action: 'generate',
            channel_id: 1,
            submit_time: 1,
            result_url: '/v1/videos/task_public_preview/content?signature=test',
            video_params: { has_video: false },
            status: TASK_STATUS.SUCCESS,
          },
        })
      )
    )
  })
  return { host, root }
}

describe('task video preview', () => {
  after(() => domWindow.close())

  test('maps Grok video edit tasks to their user-facing labels', () => {
    assert.equal(taskActionMapper.getLabel('video_edit'), 'Video Editing')
    assert.equal(taskPlatformMapper.getLabel('62'), 'Grok')
  })

  test('allows successful Grok video tasks with a signed result URL to preview', () => {
    const task = {
      platform: TASK_PLATFORMS.MOLII_GROK,
      status: TASK_STATUS.SUCCESS,
      result_url:
        '/v1/videos/task_rR90qPDjBcnNcJP7cOGvcu2NhADWecJw/content?expires=1&signature=test',
    }

    assert.equal(isGeneratedVideoTask(task), true)
    assert.equal(canPreviewVideoTask(task), true)
  })

  test('does not expose a preview action without a usable result URL', () => {
    assert.equal(
      canPreviewVideoTask({
        platform: TASK_PLATFORMS.MOLII_GROK,
        status: TASK_STATUS.SUCCESS,
        result_url: '   ',
      }),
      false
    )
  })

  test('keeps unfinished Grok tasks in the progress branch', () => {
    assert.equal(
      isGeneratedVideoTask({
        platform: TASK_PLATFORMS.MOLII_GROK,
        status: TASK_STATUS.IN_PROGRESS,
      }),
      false
    )
  })

  test('preserves Seedance generated video preview behavior', () => {
    assert.equal(
      canPreviewVideoTask({
        platform: TASK_PLATFORMS.STARAI,
        status: TASK_STATUS.SUCCESS,
        result_url: '/v1/videos/seedance-task/content?signature=test',
      }),
      true
    )
  })

  test('shows the temporary-link warning only for Grok video previews', () => {
    assert.equal(
      shouldShowGrokVideoTemporaryLinkWarning({
        platform: TASK_PLATFORMS.MOLII_GROK,
      }),
      true
    )
    assert.equal(
      shouldShowGrokVideoTemporaryLinkWarning({
        platform: TASK_PLATFORMS.STARAI,
      }),
      false
    )
    assert.equal(
      shouldShowGrokVideoTemporaryLinkWarning({ platform: 'runway' }),
      false
    )
  })

  test('renders the Grok warning before the player in a bounded two-row layout', async () => {
    const grok = await renderVideoPreview(TASK_PLATFORMS.MOLII_GROK)
    const alert = document.querySelector('[role="alert"]')
    const player = document.querySelector('video')

    assert.ok(alert)
    assert.ok(player)
    assert.ok(
      alert.compareDocumentPosition(player) & Node.DOCUMENT_POSITION_FOLLOWING,
      'the temporary-link warning must precede the player'
    )
    assert.ok(
      alert.parentElement?.className.includes(
        'lg:grid-rows-[auto_minmax(0,1fr)]'
      ),
      'the alert row must not make the full-height player overflow the dialog'
    )

    await act(async () => grok.root.unmount())
    grok.host.remove()

    const seedance = await renderVideoPreview(TASK_PLATFORMS.STARAI)
    assert.equal(document.querySelector('[role="alert"]'), null)
    await act(async () => seedance.root.unmount())
    seedance.host.remove()
  })
})
