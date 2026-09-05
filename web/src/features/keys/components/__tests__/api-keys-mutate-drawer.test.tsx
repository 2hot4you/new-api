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

import { Window } from 'happy-dom'
import { afterAll as after, afterEach, describe, test } from 'vitest'

import type { PricingData } from '@/features/pricing/types'

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
  'NodeFilter',
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

const { act, Component, useState } = await import('react')
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

const emptyPricing: PricingData = {
  success: true,
  data: [],
  vendors: [],
  group_ratio: {},
  usable_group: {},
  supported_endpoint: {},
  auto_groups: [],
}

const apiClient = api as unknown as MockableApi
const originalGet = apiClient.get
const originalPost = apiClient.post
const originalPut = apiClient.put
let renderedDrawer: RenderedDrawer | null = null
let capturedDrawerError: Error | null = null

class DrawerErrorBoundary extends Component<
  { children: React.ReactNode },
  { hasError: boolean }
> {
  state = { hasError: false }

  static getDerivedStateFromError() {
    return { hasError: true }
  }

  componentDidCatch(error: Error) {
    capturedDrawerError = error
  }

  render() {
    return this.state.hasError ? <div>drawer crashed</div> : this.props.children
  }
}

function ControlledDrawer(props: {
  closeOnChange: boolean
  currentRow?: ReturnType<typeof apiKeySchema.parse>
}) {
  const [open, setOpen] = useState(true)

  return (
    <ApiKeysMutateDrawer
      open={open}
      onOpenChange={props.closeOnChange ? setOpen : () => undefined}
      currentRow={props.currentRow}
    />
  )
}

function installApiFixtures(
  createdPayloads: Array<Record<string, unknown>>,
  options: {
    apiKey?: ReturnType<typeof apiKeySchema.parse>
    autoGroups?: string[]
    defaultUseAutoGroup?: boolean
    models?: string[]
    modelsByGroup?: Record<string, string[]>
    pricing?: PricingData
    updatedPayloads?: Array<Record<string, unknown>>
  } = {}
) {
  apiClient.get = async (url) => {
    if (options.apiKey && url === `/api/token/${options.apiKey.id}`) {
      return { data: { success: true, data: options.apiKey } }
    }
    if (url.startsWith('/api/user/models?group=')) {
      const group = decodeURIComponent(url.split('=', 2)[1] ?? '')
      return {
        data: {
          success: true,
          data: options.modelsByGroup?.[group] ?? [],
        },
      }
    }
    switch (url) {
      case '/api/status':
        return {
          data: {
            data: {
              default_use_auto_group: options.defaultUseAutoGroup ?? true,
            },
          },
        }
      case '/api/user/models':
        return { data: { success: true, data: options.models ?? [] } }
      case '/api/pricing':
        return { data: options.pricing ?? emptyPricing }
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
            data: {
              groups: options.autoGroups ?? ['vip', 'default'],
              max_count: 3,
            },
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
  currentRow?: ReturnType<typeof apiKeySchema.parse>,
  models: string[] = [],
  closeOnChange = false,
  options: {
    autoGroups?: string[]
    defaultUseAutoGroup?: boolean
    modelsByGroup?: Record<string, string[]>
    preloadQueries?: boolean
  } = {}
): Promise<void> {
  const host = document.createElement('div')
  document.body.append(host)
  const root = createRoot(host)
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  if (options.preloadQueries !== false) {
    const freshAt = Date.now() + 60_000
    queryClient.setQueryData(
      ['status'],
      { default_use_auto_group: options.defaultUseAutoGroup ?? true },
      { updatedAt: freshAt }
    )
    queryClient.setQueryData(
      ['user-models'],
      { success: true, data: models },
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
        data: {
          groups: options.autoGroups ?? ['vip', 'default'],
          max_count: 3,
        },
      },
      { updatedAt: freshAt }
    )
    for (const [group, groupModels] of Object.entries(
      options.modelsByGroup ?? {}
    )) {
      queryClient.setQueryData(
        ['user-models', group],
        { success: true, data: groupModels },
        { updatedAt: freshAt }
      )
    }
    if (currentRow) {
      queryClient.setQueryData(
        ['api-key', currentRow.id],
        { success: true, data: currentRow },
        { updatedAt: freshAt }
      )
    }
  }
  renderedDrawer = { host, queryClient, root }

  await act(async () =>
    root.render(
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>
          <ApiKeysProvider>
            <DrawerErrorBoundary>
              <ControlledDrawer
                closeOnChange={closeOnChange}
                currentRow={currentRow}
              />
            </DrawerErrorBoundary>
          </ApiKeysProvider>
        </I18nextProvider>
      </QueryClientProvider>
    )
  )
  await act(async () =>
    waitForCondition(() => {
      const saveButton = findButton('Save changes', false)
      return (
        saveButton !== null &&
        document
          .querySelector('form#api-key-form')
          ?.getAttribute('aria-busy') === 'false'
      )
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
    (candidate) =>
      candidate.textContent?.trim().replace(/\s*\*$/, '') === labelText
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

async function openModelLimitOptions(): Promise<HTMLInputElement> {
  const advancedButton = findButton('Advanced Settings', true)
  if (advancedButton.getAttribute('aria-expanded') !== 'true') {
    await act(async () => advancedButton.click())
  }
  const modelControl = getControlByLabel<HTMLElement>('Model Limits')
  const modelInput = modelControl.matches('input')
    ? (modelControl as HTMLInputElement)
    : modelControl.querySelector<HTMLInputElement>('input')
  assert.ok(modelInput)
  await act(async () => {
    modelInput.focus()
    modelInput.dispatchEvent(
      new domWindow.KeyboardEvent('keydown', {
        bubbles: true,
        key: 'ArrowDown',
      }) as unknown as Event
    )
  })
  return modelInput
}

function renderedModelOptions(): string[] {
  return [
    ...document.querySelectorAll<HTMLElement>('[data-slot="combobox-item"]'),
  ].flatMap((item) => {
    const value = item.querySelector('span.truncate')?.textContent?.trim()
    return value ? [value] : []
  })
}

afterEach(async () => {
  apiClient.get = originalGet
  apiClient.post = originalPost
  apiClient.put = originalPut
  domWindow.localStorage.clear()
  capturedDrawerError = null
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
  test('starts without a fallback group when no system default is configured', async () => {
    const createdPayloads: Array<Record<string, unknown>> = []
    installApiFixtures(createdPayloads, {
      autoGroups: [],
      defaultUseAutoGroup: false,
    })
    await renderCreateDrawer(undefined, undefined, [], false, {
      autoGroups: [],
      defaultUseAutoGroup: false,
    })

    const groupControl = getControlByLabel<HTMLElement>('Model routing')
    const groupTrigger = groupControl.querySelector<HTMLButtonElement>(
      'button[role="combobox"]'
    )
    assert.ok(groupTrigger)
    assert.equal(
      groupTrigger.textContent?.includes('Select access points'),
      true
    )
    assert.equal(
      document.body.textContent?.includes('0 / 3 access points selected'),
      true
    )
  })

  test('marks name and group as required fields', async () => {
    const createdPayloads: Array<Record<string, unknown>> = []
    installApiFixtures(createdPayloads)
    await renderCreateDrawer()

    const labels = new Set(
      [...document.querySelectorAll<HTMLLabelElement>('label')]
        .map((label) => label.textContent?.trim())
        .filter(Boolean)
    )

    assert.ok(labels.has('Name *'))
    assert.ok(labels.has('Model routing *'))
  })

  test('shows real groups only and copies the configured order for batch-created keys', async () => {
    const createdPayloads: Array<Record<string, unknown>> = []
    installApiFixtures(createdPayloads)
    await renderCreateDrawer()

    const sheetContent = document.querySelector<HTMLElement>(
      '[data-slot="sheet-content"]'
    )
    assert.ok(sheetContent)
    assert.equal(sheetContent.classList.contains('overflow-x-hidden'), true)

    const groupControl = getControlByLabel<HTMLElement>('Model routing')
    const groupTrigger = groupControl.querySelector<HTMLButtonElement>(
      'button[role="combobox"]'
    )
    assert.ok(groupTrigger)
    assert.equal(
      groupTrigger.textContent?.includes('2 access points selected'),
      true
    )
    assert.equal(
      document.body.textContent?.includes('2 / 3 access points selected'),
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

    const groupControl = getControlByLabel<HTMLElement>('Model routing')
    const groupTrigger = groupControl.querySelector<HTMLButtonElement>(
      'button[role="combobox"]'
    )
    assert.ok(groupTrigger)
    await selectComboboxOption(groupTrigger, 'Priority access')

    assert.ok(document.querySelector('[data-selected-group-item="default"]'))
    assert.equal(
      document.querySelector('[data-selected-group-item="vip"]'),
      null
    )
    assert.equal(
      document.body.textContent?.includes('One access point selected'),
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

    const groupControl = getControlByLabel<HTMLElement>('Model routing')
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
    assert.equal(
      groupTrigger.textContent?.includes('2 access points selected'),
      true
    )
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

    const groupControl = getControlByLabel<HTMLElement>('Model routing')
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

    const groupControl = getControlByLabel<HTMLElement>('Model routing')
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

    const groupControl = getControlByLabel<HTMLElement>('Model routing')
    const groupTrigger = groupControl.querySelector<HTMLButtonElement>(
      'button[role="combobox"]'
    )
    assert.ok(groupTrigger)
    assert.ok(document.querySelector('[data-selected-group-item="default"]'))
    assert.equal(
      document.body.textContent?.includes('One access point selected'),
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

    const groupControl = getControlByLabel<HTMLElement>('Model routing')
    const groupTrigger = groupControl.querySelector<HTMLButtonElement>(
      'button[role="combobox"]'
    )
    assert.ok(groupTrigger)
    assert.equal(
      groupTrigger.textContent?.includes('2 access points selected'),
      true
    )

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

describe('API keys mutate drawer model limits', () => {
  test('disables model selection until a group is selected', async () => {
    const createdPayloads: Array<Record<string, unknown>> = []
    installApiFixtures(createdPayloads, {
      autoGroups: [],
      defaultUseAutoGroup: false,
      models: ['claude-vip', 'gpt-default'],
    })
    await renderCreateDrawer(
      undefined,
      undefined,
      ['claude-vip', 'gpt-default'],
      false,
      {
        autoGroups: [],
        defaultUseAutoGroup: false,
      }
    )

    await act(async () => findButton('Advanced Settings', true).click())
    const modelInput = getControlByLabel<HTMLElement>('Model Limits')
      .closest('[data-slot="form-item"]')
      ?.querySelector<HTMLInputElement>('input')
    assert.ok(modelInput)
    assert.equal(modelInput.disabled, true)
    assert.equal(modelInput.getAttribute('aria-label'), 'Select a group first')
  })

  test('shows only models enabled for the selected group', async () => {
    const createdPayloads: Array<Record<string, unknown>> = []
    installApiFixtures(createdPayloads, {
      autoGroups: [],
      defaultUseAutoGroup: false,
      models: ['claude-vip', 'gpt-vip', 'outside-model'],
      modelsByGroup: {
        vip: ['claude-vip', 'gpt-vip'],
        default: ['outside-model'],
      },
    })
    await renderCreateDrawer(
      undefined,
      undefined,
      ['claude-vip', 'gpt-vip', 'outside-model'],
      false,
      {
        autoGroups: [],
        defaultUseAutoGroup: false,
        modelsByGroup: {
          vip: ['claude-vip', 'gpt-vip'],
          default: ['outside-model'],
        },
      }
    )

    const groupControl = getControlByLabel<HTMLElement>('Model routing')
    const groupTrigger = groupControl.querySelector<HTMLButtonElement>(
      'button[role="combobox"]'
    )
    assert.ok(groupTrigger)
    await selectComboboxOption(groupTrigger, 'Priority access')
    await openModelLimitOptions()
    await act(async () =>
      waitForCondition(
        () => renderedModelOptions().length > 0,
        'group model options did not render'
      )
    )

    assert.deepEqual(renderedModelOptions(), ['claude-vip', 'gpt-vip'])
  })

  test('combines and deduplicates models from multiple selected groups', async () => {
    const createdPayloads: Array<Record<string, unknown>> = []
    installApiFixtures(createdPayloads, {
      autoGroups: [],
      defaultUseAutoGroup: false,
      models: ['shared-model', 'claude-vip', 'gpt-default', 'outside-model'],
      modelsByGroup: {
        vip: ['shared-model', 'claude-vip'],
        default: ['shared-model', 'gpt-default'],
      },
    })
    await renderCreateDrawer(
      undefined,
      undefined,
      ['shared-model', 'claude-vip', 'gpt-default', 'outside-model'],
      false,
      {
        autoGroups: [],
        defaultUseAutoGroup: false,
        modelsByGroup: {
          vip: ['shared-model', 'claude-vip'],
          default: ['shared-model', 'gpt-default'],
        },
      }
    )

    const groupControl = getControlByLabel<HTMLElement>('Model routing')
    const groupTrigger = groupControl.querySelector<HTMLButtonElement>(
      'button[role="combobox"]'
    )
    assert.ok(groupTrigger)
    await selectComboboxOption(groupTrigger, 'Priority access')
    const defaultOption = [
      ...document.querySelectorAll<HTMLElement>(
        '[data-group-selection-checkbox]'
      ),
    ].find((option) => option.textContent?.includes('Standard access'))
    assert.ok(defaultOption)
    await act(async () => defaultOption.click())
    await openModelLimitOptions()
    await act(async () =>
      waitForCondition(
        () => renderedModelOptions().length === 3,
        'combined group model options did not render'
      )
    )

    assert.deepEqual(renderedModelOptions(), [
      'shared-model',
      'claude-vip',
      'gpt-default',
    ])
  })

  test('removes only model limits excluded by a user-initiated group change', async () => {
    const createdPayloads: Array<Record<string, unknown>> = []
    const modelsByGroup = {
      vip: ['shared-model', 'vip-only-model'],
      default: ['shared-model', 'default-only-model'],
    }
    installApiFixtures(createdPayloads, {
      autoGroups: [],
      defaultUseAutoGroup: false,
      models: ['shared-model', 'vip-only-model', 'default-only-model'],
      modelsByGroup,
    })
    await renderCreateDrawer(
      undefined,
      undefined,
      ['shared-model', 'vip-only-model', 'default-only-model'],
      false,
      {
        autoGroups: [],
        defaultUseAutoGroup: false,
        modelsByGroup,
      }
    )

    const groupControl = getControlByLabel<HTMLElement>('Model routing')
    const groupTrigger = groupControl.querySelector<HTMLButtonElement>(
      'button[role="combobox"]'
    )
    assert.ok(groupTrigger)
    await selectComboboxOption(groupTrigger, 'Priority access')
    await openModelLimitOptions()
    await act(async () =>
      waitForCondition(
        () => renderedModelOptions().length === 2,
        'VIP model options did not render'
      )
    )
    for (const model of ['shared-model', 'vip-only-model']) {
      const option = [
        ...document.querySelectorAll<HTMLElement>(
          '[data-slot="combobox-item"]'
        ),
      ].find(
        (item) =>
          item.querySelector('span.truncate')?.textContent?.trim() === model
      )
      assert.ok(option)
      await act(async () => option.click())
    }

    await act(async () => groupTrigger.click())
    const groupOptions = [
      ...document.querySelectorAll<HTMLElement>(
        '[data-group-selection-checkbox]'
      ),
    ]
    const defaultOption = groupOptions.find((option) =>
      option.textContent?.includes('Standard access')
    )
    const vipOption = groupOptions.find((option) =>
      option.textContent?.includes('Priority access')
    )
    assert.ok(defaultOption)
    assert.ok(vipOption)
    await act(async () => defaultOption.click())
    await act(async () => vipOption.click())
    await act(async () =>
      waitForCondition(
        () => document.body.textContent?.includes('vip-only-model') === false,
        'model excluded by the new group remained selected'
      )
    )

    const selectedChipText = [
      ...document.querySelectorAll<HTMLElement>('[data-slot="combobox-chip"]'),
    ].map((chip) =>
      [...chip.querySelectorAll('span')].at(-1)?.textContent?.trim()
    )
    assert.deepEqual(selectedChipText, ['shared-model'])
  })

  test('uses the configured model icon in model choices and selected chips', async () => {
    const createdPayloads: Array<Record<string, unknown>> = []
    const pricing: PricingData = {
      success: true,
      data: [
        {
          id: 1,
          model_name: 'claude-test-model',
          icon: 'Claude.Color',
          vendor_id: 2,
          quota_type: 0,
          model_ratio: 1,
          completion_ratio: 1,
          enable_groups: [],
        },
      ],
      vendors: [{ id: 2, name: 'OpenAI', icon: 'OpenAI.Color' }],
      group_ratio: {},
      usable_group: {},
      supported_endpoint: {},
      auto_groups: [],
    }
    installApiFixtures(createdPayloads, {
      modelsByGroup: {
        vip: ['claude-test-model'],
        default: ['claude-test-model'],
      },
      pricing,
    })
    await renderCreateDrawer(
      undefined,
      undefined,
      ['claude-test-model'],
      false,
      {
        modelsByGroup: {
          vip: ['claude-test-model'],
          default: ['claude-test-model'],
        },
      }
    )

    await act(async () => findButton('Advanced Settings', true).click())
    const modelControl = getControlByLabel<HTMLElement>('Model Limits')
    const modelInput = modelControl.matches('input')
      ? (modelControl as HTMLInputElement)
      : modelControl.querySelector<HTMLInputElement>('input')
    assert.ok(modelInput)
    await act(async () => {
      modelInput.focus()
      modelInput.dispatchEvent(
        new domWindow.KeyboardEvent('keydown', {
          bubbles: true,
          key: 'ArrowDown',
        }) as unknown as Event
      )
    })
    await changeInput(modelInput, 'claude')
    await act(async () =>
      waitForCondition(
        () =>
          [
            ...document.querySelectorAll<HTMLElement>(
              '[data-slot="combobox-item"]'
            ),
          ].some((item) => item.textContent?.includes('claude-test-model')),
        'model restriction option did not render'
      )
    )

    const modelOption = [
      ...document.querySelectorAll<HTMLElement>('[data-slot="combobox-item"]'),
    ].find((item) => item.textContent?.includes('claude-test-model'))
    assert.ok(modelOption)
    assert.ok(
      modelOption.querySelector('[data-model-limit-icon="Claude.Color"]')
    )
    assert.equal(
      modelOption.querySelector('[data-model-limit-icon="OpenAI.Color"]'),
      null
    )

    await act(async () => modelOption.click())
    await act(async () =>
      waitForCondition(
        () =>
          document.querySelector(
            '[data-slot="combobox-chip"] [data-model-limit-icon="Claude.Color"]'
          ) !== null,
        'selected model chip did not render its configured icon'
      )
    )

    await changeInput(getControlByLabel<HTMLInputElement>('Name'), 'limited')
    await act(async () => findButton('Save changes', true).click())
    await act(async () =>
      waitForCondition(
        () => createdPayloads.length === 1,
        'model-limited API key was not created'
      )
    )
    assert.equal(createdPayloads[0]?.model_limits, 'claude-test-model')
  })
})

describe('API keys mutate drawer IP restrictions', () => {
  test('allows empty optional restrictions after asynchronous form initialization', async () => {
    const createdPayloads: Array<Record<string, unknown>> = []
    installApiFixtures(createdPayloads)
    await renderCreateDrawer(undefined, undefined, [], false, {
      preloadQueries: false,
    })

    await changeInput(
      getControlByLabel<HTMLInputElement>('Name'),
      'unrestricted'
    )
    assert.equal(findButton('Save changes', true).disabled, false)
    await act(async () => findButton('Save changes', true).click())
    await act(async () =>
      waitForCondition(
        () => createdPayloads.length === 1,
        'unrestricted API key was not created'
      )
    )

    assert.equal(createdPayloads[0]?.model_limits_enabled, false)
    assert.equal(createdPayloads[0]?.model_limits, '')
    assert.equal(createdPayloads[0]?.allow_ips, '')
  })

  test('closes cleanly after a successful create without a repeated state update', async () => {
    const createdPayloads: Array<Record<string, unknown>> = []
    installApiFixtures(createdPayloads)
    await renderCreateDrawer(undefined, undefined, [], true)
    await changeInput(getControlByLabel<HTMLInputElement>('Name'), 'close-test')
    await act(async () => findButton('Save changes', true).click())
    await act(async () =>
      waitForCondition(
        () => createdPayloads.length === 1,
        'API key was not created'
      )
    )

    assert.equal(capturedDrawerError, null)
    assert.equal(document.body.textContent?.includes('drawer crashed'), false)
  })

  test('closes cleanly after a successful update without a repeated state update', async () => {
    const apiKey = makeLegacyApiKey()
    const updatedPayloads: Array<Record<string, unknown>> = []
    installApiFixtures([], { apiKey, updatedPayloads })
    await renderCreateDrawer(undefined, apiKey, [], true)

    await changeInput(getControlByLabel<HTMLInputElement>('Name'), 'renamed')
    await act(async () => findButton('Save changes', true).click())
    await act(async () =>
      waitForCondition(
        () => updatedPayloads.length === 1,
        'API key was not updated'
      )
    )

    assert.equal(capturedDrawerError, null)
    assert.equal(document.body.textContent?.includes('drawer crashed'), false)
  })

  test('blocks a valid uncommitted IP draft until Add serializes the newline-delimited payload', async () => {
    const createdPayloads: Array<Record<string, unknown>> = []
    installApiFixtures(createdPayloads)
    await renderCreateDrawer()

    await act(async () => findButton('Advanced Settings', true).click())
    const ipInput = document.querySelector<HTMLInputElement>(
      'input[aria-label="IP address or CIDR"]'
    )
    assert.ok(ipInput, 'Expected IP address input')
    assert.equal(ipInput.name, 'allow_ips')
    await changeInput(ipInput, '192.0.2.1')
    assert.equal(findButton('Save changes', true).disabled, true)
    await act(async () => findButton('Add IP address', true).click())
    await changeInput(ipInput, '2001:db8::1/64')
    assert.equal(findButton('Save changes', true).disabled, true)
    await act(async () => findButton('Add IP address', true).click())
    assert.equal(findButton('Save changes', true).disabled, false)
    await changeInput(getControlByLabel<HTMLInputElement>('Name'), 'restricted')
    await act(async () => findButton('Save changes', true).click())
    await act(async () =>
      waitForCondition(
        () => createdPayloads.length === 1,
        'restricted API key was not created'
      )
    )

    assert.equal(createdPayloads[0]?.allow_ips, '192.0.2.1\n2001:db8::/64')
  })

  test('does not submit an invalid uncommitted IP draft', async () => {
    const createdPayloads: Array<Record<string, unknown>> = []
    installApiFixtures(createdPayloads)
    await renderCreateDrawer()

    await act(async () => findButton('Advanced Settings', true).click())
    const ipInput = document.querySelector<HTMLInputElement>(
      'input[aria-label="IP address or CIDR"]'
    )
    assert.ok(ipInput, 'Expected IP address input')
    await changeInput(ipInput, '300.0.0.1')
    await changeInput(getControlByLabel<HTMLInputElement>('Name'), 'blocked')
    assert.equal(findButton('Save changes', true).disabled, true)
    assert.equal(
      document.body.textContent?.includes('Enter a valid IP address or CIDR'),
      true
    )
    await act(async () => findButton('Save changes', true).click())

    assert.equal(createdPayloads.length, 0)
    assert.equal(
      document.body.textContent?.includes('Enter a valid IP address or CIDR'),
      true
    )
  })

  test('blocks legacy IP restrictions before Advanced Settings is opened', async () => {
    const apiKey = makeLegacyApiKey({ allow_ips: 'legacy-address' })
    const updatedPayloads: Array<Record<string, unknown>> = []
    installApiFixtures([], { apiKey, updatedPayloads })
    await renderCreateDrawer(undefined, apiKey)

    await changeInput(getControlByLabel<HTMLInputElement>('Name'), 'renamed')
    assert.equal(findButton('Save changes', true).disabled, true)
    await act(async () => findButton('Save changes', true).click())

    assert.equal(updatedPayloads.length, 0)
  })

  test('blocks legacy invalid restrictions until removal leaves a canonical payload', async () => {
    const apiKey = makeLegacyApiKey({
      allow_ips: '2001:0DB8:0:0::1\n2001:db8::1\nlegacy-address',
    })
    const updatedPayloads: Array<Record<string, unknown>> = []
    installApiFixtures([], { apiKey, updatedPayloads })
    await renderCreateDrawer(undefined, apiKey)

    await act(async () => findButton('Advanced Settings', true).click())
    assert.equal(findButton('Save changes', true).disabled, true)
    await changeInput(getControlByLabel<HTMLInputElement>('Name'), 'renamed')
    assert.equal(findButton('Save changes', true).disabled, true)

    const removeLegacyButton = document.querySelector<HTMLButtonElement>(
      'button[aria-label="Remove legacy-address"]'
    )
    assert.ok(removeLegacyButton, 'Expected legacy IP removal button')
    await act(async () => removeLegacyButton.click())
    assert.equal(findButton('Save changes', true).disabled, false)
    await act(async () => findButton('Save changes', true).click())
    await act(async () =>
      waitForCondition(
        () => updatedPayloads.length === 1,
        'API key was not updated after removing the legacy IP restriction'
      )
    )

    assert.equal(updatedPayloads[0]?.allow_ips, '2001:db8::1')
  })

  test('keeps the configured icon for a selected unavailable model without listing it as selectable', async () => {
    const apiKey = makeLegacyApiKey({
      model_limits_enabled: true,
      model_limits: 'retired-model',
    })
    const pricing: PricingData = {
      ...emptyPricing,
      data: [
        {
          id: 1,
          model_name: 'retired-model',
          icon: 'Claude.Color',
          vendor_id: 2,
          quota_type: 0,
          model_ratio: 1,
          completion_ratio: 1,
          enable_groups: [],
        },
      ],
      vendors: [{ id: 2, name: 'OpenAI', icon: 'OpenAI.Color' }],
    }
    installApiFixtures([], { apiKey, pricing })
    await renderCreateDrawer(undefined, apiKey)

    await act(async () => findButton('Advanced Settings', true).click())
    assert.ok(
      document.querySelector(
        '[data-slot="combobox-chip"] [data-model-limit-icon="Claude.Color"]'
      )
    )
    const modelInput = document.querySelector<HTMLInputElement>(
      '[data-slot="combobox-chip-input"]'
    )
    assert.ok(modelInput)
    await act(async () => modelInput.focus())
    await act(async () =>
      modelInput.dispatchEvent(
        new domWindow.KeyboardEvent('keydown', {
          bubbles: true,
          key: 'ArrowDown',
        }) as unknown as Event
      )
    )

    assert.equal(
      [
        ...document.querySelectorAll<HTMLElement>(
          '[data-slot="combobox-item"]'
        ),
      ].some((item) => item.textContent?.includes('retired-model')),
      false
    )
  })
})
