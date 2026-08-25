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
import { after, afterEach, describe, test } from 'node:test'

import { Window } from 'happy-dom'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'HTMLInputElement',
  'HTMLFormElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'KeyboardEvent',
  'PointerEvent',
  'MouseEvent',
  'FocusEvent',
  'CustomEvent',
  'customElements',
  'MutationObserver',
  'ResizeObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
] as const

for (const key of domGlobals) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}
Object.defineProperty(globalThis, 'matchMedia', {
  configurable: true,
  value: domWindow.matchMedia.bind(domWindow),
})

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { api } = await import('@/lib/api')
const { ApiKeysProvider } = await import('../api-keys-provider')
const { ApiKeysMutateDrawer } = await import('../api-keys-mutate-drawer')
const { apiKeySchema } = await import('../../types')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

type ApiMethod = (url: string, data?: unknown) => Promise<{ data: unknown }>
type MockableApi = {
  get: ApiMethod
  post: ApiMethod
  put: ApiMethod
}
type RenderedDrawer = {
  host: HTMLDivElement
  queryClient: InstanceType<typeof QueryClient>
  root: ReturnType<typeof createRoot>
}

const apiClient = api as unknown as MockableApi
const originalGet = apiClient.get
const originalPost = apiClient.post
const originalPut = apiClient.put
let renderedDrawer: RenderedDrawer | null = null

function installApiFixtures(
  createdPayloads: Array<Record<string, unknown>>,
  options: {
    apiKey?: ReturnType<typeof apiKeySchema.parse>
    updatedPayloads?: Array<Record<string, unknown>>
  } = {}
) {
  apiClient.get = async (url) => {
    if (options.apiKey && url === `/api/token/${options.apiKey.id}`) {
      return { data: { success: true, data: options.apiKey } }
    }
    switch (url) {
      case '/api/status':
        return { data: { data: { default_use_auto_group: true } } }
      case '/api/user/models':
        return { data: { success: true, data: [] } }
      case '/api/user/self/groups':
        return {
          data: {
            success: true,
            data: {
              auto: {
                desc: 'Automatic routing',
                ratio: 'auto',
                display_order: 2,
              },
              default: {
                desc: 'Standard access',
                ratio: 1,
                display_order: 1,
              },
              vip: {
                desc: 'Priority access',
                ratio: 2,
                display_order: 0,
              },
            },
          },
        }
      case '/api/token/auto-groups':
        return {
          data: {
            success: true,
            data: { groups: ['vip', 'default'], max_count: 3 },
          },
        }
      default:
        throw new Error(`Unexpected GET ${url}`)
    }
  }
  apiClient.post = async (url, data) => {
    assert.equal(url, '/api/token/')
    assert.ok(data && typeof data === 'object')
    createdPayloads.push(data as Record<string, unknown>)
    return { data: { success: true, data: {} } }
  }
  apiClient.put = async (url, data) => {
    assert.equal(url, '/api/token/')
    assert.ok(data && typeof data === 'object')
    options.updatedPayloads?.push(data as Record<string, unknown>)
    return { data: { success: true, data: {} } }
  }
}

async function waitForCondition(
  condition: () => boolean,
  failureMessage: string
): Promise<void> {
  if (condition()) return

  await new Promise<void>((resolve, reject) => {
    const observer = new MutationObserver(() => {
      if (!condition()) return
      clearTimeout(timeoutId)
      observer.disconnect()
      resolve()
    })
    const timeoutId = setTimeout(() => {
      observer.disconnect()
      reject(new Error(`${failureMessage}: ${document.body.textContent}`))
    }, 1500)

    observer.observe(document, {
      attributes: true,
      childList: true,
      characterData: true,
      subtree: true,
    })
  })
}

async function renderCreateDrawer(
  groupsData?: {
    success: boolean
    data: Record<
      string,
      { desc: string; ratio: number | string; display_order?: number }
    >
  },
  currentRow?: ReturnType<typeof apiKeySchema.parse>
): Promise<void> {
  const host = document.createElement('div')
  document.body.append(host)
  const root = createRoot(host)
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const freshAt = Date.now() + 60_000
  queryClient.setQueryData(
    ['status'],
    { default_use_auto_group: true },
    { updatedAt: freshAt }
  )
  queryClient.setQueryData(
    ['user-models'],
    { success: true, data: [] },
    { updatedAt: freshAt }
  )
  queryClient.setQueryData(
    ['user-groups'],
    groupsData ?? {
      success: true,
      data: {
        auto: {
          desc: 'Automatic routing',
          ratio: 'auto',
          display_order: 2,
        },
        default: {
          desc: 'Standard access',
          ratio: 1,
          display_order: 1,
        },
        vip: {
          desc: 'Priority access',
          ratio: 2,
          display_order: 0,
        },
      },
    },
    { updatedAt: freshAt }
  )
  queryClient.setQueryData(
    ['token-auto-groups'],
    {
      success: true,
      data: { groups: ['vip', 'default'], max_count: 3 },
    },
    { updatedAt: freshAt }
  )
  if (currentRow) {
    queryClient.setQueryData(
      ['api-key', currentRow.id],
      { success: true, data: currentRow },
      { updatedAt: freshAt }
    )
  }
  renderedDrawer = { host, queryClient, root }

  await act(async () =>
    root.render(
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>
          <ApiKeysProvider>
            <ApiKeysMutateDrawer
              open
              onOpenChange={() => undefined}
              currentRow={currentRow}
            />
          </ApiKeysProvider>
        </I18nextProvider>
      </QueryClientProvider>
    )
  )
  await act(async () =>
    waitForCondition(() => {
      const saveButton = findButton('Save changes', false)
      return saveButton !== null && !saveButton.disabled
    }, 'API key drawer did not finish initializing')
  )
}

function findButton(text: string, required: true): HTMLButtonElement
function findButton(text: string, required: false): HTMLButtonElement | null
function findButton(text: string, required = true): HTMLButtonElement | null {
  const button = [
    ...document.querySelectorAll<HTMLButtonElement>('button'),
  ].find((candidate) => candidate.textContent?.includes(text))
  if (required) assert.ok(button, `Expected button containing "${text}"`)
  return button ?? null
}

function getControlByLabel<T extends HTMLElement>(labelText: string): T {
  const label = [...document.querySelectorAll<HTMLLabelElement>('label')].find(
    (candidate) => candidate.textContent?.trim() === labelText
  )
  assert.ok(label, `Expected label "${labelText}"`)
  assert.ok(label.htmlFor)
  const control =
    label.control ??
    label
      .closest('[data-slot="form-item"]')
      ?.querySelector<HTMLElement>(
        '[data-slot="form-control"], input, textarea, button[role="combobox"], [role="group"]'
      )
  assert.ok(control)
  return control as T
}

async function changeInput(input: HTMLInputElement, value: string) {
  await act(async () => {
    const valueSetter = Object.getOwnPropertyDescriptor(
      domWindow.HTMLInputElement.prototype,
      'value'
    )?.set
    assert.ok(valueSetter)
    valueSetter.call(input, value)
    input.dispatchEvent(
      new domWindow.Event('input', { bubbles: true }) as unknown as Event
    )
  })
}

async function selectComboboxOption(
  trigger: HTMLButtonElement,
  optionDescription: string
) {
  await act(async () => trigger.click())
  const option = [
    ...document.querySelectorAll<HTMLElement>(
      '[data-group-selection-checkbox]'
    ),
  ].find((candidate) => candidate.textContent?.includes(optionDescription))
  assert.ok(option, `Expected option containing "${optionDescription}"`)
  await act(async () => option.click())
}

afterEach(async () => {
  apiClient.get = originalGet
  apiClient.post = originalPost
  apiClient.put = originalPut
  domWindow.localStorage.clear()
  if (renderedDrawer) {
    await act(async () => renderedDrawer?.root.unmount())
    renderedDrawer.queryClient.clear()
    renderedDrawer.host.remove()
    renderedDrawer = null
  }
  document.body.replaceChildren()
})

after(() => {
  domWindow.close()
})

function makeLegacyApiKey(overrides: Record<string, unknown> = {}) {
  return apiKeySchema.parse({
    id: 7,
    name: 'legacy-key',
    key: 'sk-test',
    status: 1,
    remain_quota: 0,
    used_quota: 0,
    unlimited_quota: true,
    expired_time: -1,
    created_time: 1,
    accessed_time: 0,
    group: 'default',
    auto_groups: null,
    cross_group_retry: false,
    model_limits_enabled: false,
    model_limits: '',
    allow_ips: '',
    ...overrides,
  })
}

describe('API keys mutate drawer direct group selection', () => {
  test('shows real groups only and copies the configured order for batch-created keys', async () => {
    const createdPayloads: Array<Record<string, unknown>> = []
    installApiFixtures(createdPayloads)
    await renderCreateDrawer()

    const sheetContent = document.querySelector<HTMLElement>(
      '[data-slot="sheet-content"]'
    )
    assert.ok(sheetContent)
    assert.equal(sheetContent.classList.contains('overflow-x-hidden'), true)

    const groupControl = getControlByLabel<HTMLElement>('Group')
    const groupTrigger = groupControl.querySelector<HTMLButtonElement>(
      'button[role="combobox"]'
    )
    assert.ok(groupTrigger)
    assert.equal(groupTrigger.textContent?.includes('vip → default'), true)
    assert.equal(
      document.body.textContent?.includes('2 / 3 groups selected'),
      true
    )

    await act(async () => groupTrigger.click())
    assert.deepEqual(
      [
        ...document.querySelectorAll<HTMLElement>(
          '[data-group-selection-checkbox]'
        ),
      ].map((checkbox) => checkbox.dataset.groupSelectionCheckbox),
      ['vip', 'default']
    )
    assert.equal(
      document.body.textContent?.includes('Automatic routing'),
      false
    )

    await changeInput(getControlByLabel<HTMLInputElement>('Name'), 'batch')
    await changeInput(getControlByLabel<HTMLInputElement>('Quantity'), '2')
    await act(async () => findButton('Save changes', true).click())
    await act(async () =>
      waitForCondition(
        () => createdPayloads.length === 2,
        'batch API keys were not created'
      )
    )

    assert.equal(createdPayloads.length, 2)
    assert.equal(createdPayloads[0]?.name, 'batch')
    for (const payload of createdPayloads) {
      assert.equal(payload.group, 'auto')
      assert.deepEqual(payload.auto_groups, ['vip', 'default'])
      assert.equal(payload.cross_group_retry, true)
    }
  })

  test('maps one selected group to fixed routing', async () => {
    const createdPayloads: Array<Record<string, unknown>> = []
    installApiFixtures(createdPayloads)
    await renderCreateDrawer()

    const groupControl = getControlByLabel<HTMLElement>('Group')
    const groupTrigger = groupControl.querySelector<HTMLButtonElement>(
      'button[role="combobox"]'
    )
    assert.ok(groupTrigger)
    await selectComboboxOption(groupTrigger, 'Priority access')

    assert.equal(groupTrigger.textContent?.includes('default'), true)
    assert.equal(groupTrigger.textContent?.includes('vip'), false)
    assert.equal(
      document.body.textContent?.includes('One group uses fixed routing'),
      true
    )

    await changeInput(getControlByLabel<HTMLInputElement>('Name'), 'fixed')
    await act(async () => findButton('Save changes', true).click())
    await act(async () =>
      waitForCondition(
        () => createdPayloads.length === 1,
        'fixed-group API key was not created'
      )
    )
    assert.equal(createdPayloads[0]?.group, 'default')
    assert.deepEqual(createdPayloads[0]?.auto_groups, [])
    assert.equal(createdPayloads[0]?.cross_group_retry, false)
  })

  test('keeps direct selections open and submits their ordered fallback', async () => {
    const createdPayloads: Array<Record<string, unknown>> = []
    installApiFixtures(createdPayloads)
    await renderCreateDrawer()

    const groupControl = getControlByLabel<HTMLElement>('Group')
    const groupTrigger = groupControl.querySelector<HTMLButtonElement>(
      'button[role="combobox"]'
    )
    assert.ok(groupTrigger)
    await selectComboboxOption(groupTrigger, 'Priority access')
    const priorityOption = [
      ...document.querySelectorAll<HTMLElement>(
        '[data-group-selection-checkbox]'
      ),
    ].find((option) => option.textContent?.includes('Priority access'))
    assert.ok(priorityOption)
    await act(async () => priorityOption.click())

    assert.equal(groupTrigger.getAttribute('aria-expanded'), 'true')
    assert.equal(groupTrigger.textContent?.includes('default → vip'), true)
    await changeInput(getControlByLabel<HTMLInputElement>('Name'), 'fallback')
    await act(async () => findButton('Save changes', true).click())
    await act(async () =>
      waitForCondition(
        () => createdPayloads.length === 1,
        'ordered-fallback API key was not created'
      )
    )

    assert.equal(createdPayloads[0]?.group, 'auto')
    assert.deepEqual(createdPayloads[0]?.auto_groups, ['default', 'vip'])
    assert.equal(createdPayloads[0]?.cross_group_retry, true)
  })

  test('sorts group choices by display order before legacy name fallback', async () => {
    const createdPayloads: Array<Record<string, unknown>> = []
    installApiFixtures(createdPayloads)
    await renderCreateDrawer()

    const groupControl = getControlByLabel<HTMLElement>('Group')
    const groupTrigger = groupControl.querySelector<HTMLButtonElement>(
      'button[role="combobox"]'
    )
    assert.ok(groupTrigger)
    await act(async () => groupTrigger.click())

    assert.deepEqual(
      [
        ...document.querySelectorAll<HTMLElement>(
          '[data-group-selection-checkbox]'
        ),
      ]
        .map((item) => item.textContent)
        .filter(
          (text): text is string =>
            text?.includes('Priority access') ||
            text?.includes('Standard access')
        )
        .map((text) => {
          if (text.includes('Priority access')) return 'vip'
          return 'default'
        }),
      ['vip', 'default']
    )
  })

  test('uses codepoint order for legacy groups without display order', async () => {
    const createdPayloads: Array<Record<string, unknown>> = []
    installApiFixtures(createdPayloads)
    await renderCreateDrawer({
      success: true,
      data: {
        auto: { desc: 'Automatic routing', ratio: 'auto', display_order: 0 },
        z: { desc: 'Legacy Z', ratio: 1 },
        á: { desc: 'Legacy A acute', ratio: 1 },
      },
    })

    const groupControl = getControlByLabel<HTMLElement>('Group')
    const groupTrigger = groupControl.querySelector<HTMLButtonElement>(
      'button[role="combobox"]'
    )
    assert.ok(groupTrigger)
    await act(async () => groupTrigger.click())

    assert.deepEqual(
      [
        ...document.querySelectorAll<HTMLElement>(
          '[data-group-selection-checkbox]'
        ),
      ]
        .map((item) => item.textContent)
        .filter(
          (text): text is string =>
            text?.includes('Legacy Z') || text?.includes('Legacy A acute')
        )
        .map((text) => {
          if (text.includes('Legacy Z')) return 'z'
          return 'á'
        }),
      ['z', 'á']
    )
  })

  test('preserves a legacy fixed group when updating a key', async () => {
    const apiKey = makeLegacyApiKey()
    const updatedPayloads: Array<Record<string, unknown>> = []
    installApiFixtures([], { apiKey, updatedPayloads })
    await renderCreateDrawer(undefined, apiKey)

    const groupControl = getControlByLabel<HTMLElement>('Group')
    const groupTrigger = groupControl.querySelector<HTMLButtonElement>(
      'button[role="combobox"]'
    )
    assert.ok(groupTrigger)
    assert.equal(groupTrigger.textContent?.includes('default'), true)
    assert.equal(
      document.body.textContent?.includes('One group uses fixed routing'),
      true
    )

    await act(async () => findButton('Save changes', true).click())
    await act(async () =>
      waitForCondition(
        () => updatedPayloads.length === 1,
        'legacy fixed API key was not updated'
      )
    )

    assert.equal(updatedPayloads[0]?.id, apiKey.id)
    assert.equal(updatedPayloads[0]?.group, 'default')
    assert.deepEqual(updatedPayloads[0]?.auto_groups, [])
    assert.equal(updatedPayloads[0]?.cross_group_retry, false)
  })

  test('materializes the current ordered groups for a legacy Auto key', async () => {
    const apiKey = makeLegacyApiKey({
      group: 'auto',
      auto_groups: null,
      cross_group_retry: true,
    })
    const updatedPayloads: Array<Record<string, unknown>> = []
    installApiFixtures([], { apiKey, updatedPayloads })
    await renderCreateDrawer(undefined, apiKey)

    const groupControl = getControlByLabel<HTMLElement>('Group')
    const groupTrigger = groupControl.querySelector<HTMLButtonElement>(
      'button[role="combobox"]'
    )
    assert.ok(groupTrigger)
    assert.equal(groupTrigger.textContent?.includes('vip → default'), true)

    await act(async () => findButton('Save changes', true).click())
    await act(async () =>
      waitForCondition(
        () => updatedPayloads.length === 1,
        'legacy Auto API key was not updated'
      )
    )

    assert.equal(updatedPayloads[0]?.id, apiKey.id)
    assert.equal(updatedPayloads[0]?.group, 'auto')
    assert.deepEqual(updatedPayloads[0]?.auto_groups, ['vip', 'default'])
    assert.equal(updatedPayloads[0]?.cross_group_retry, true)
  })
})
