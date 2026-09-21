/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { Window } from 'happy-dom'
import {
  afterAll,
  afterEach,
  beforeEach,
  describe,
  expect,
  test,
  vi,
} from 'vitest'

const domWindow = new Window({ url: 'https://claudeye.test/' })
for (const key of [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'HTMLFormElement',
  'HTMLImageElement',
  'HTMLInputElement',
  'HTMLLabelElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MutationObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
] as const) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', {
  configurable: true,
  value: () => undefined,
})

const { useState } = await import('react')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { act, cleanup, fireEvent, render, waitFor } =
  await import('@testing-library/react')
const { createMemoryHistory, createRootRoute, createRouter, RouterProvider } =
  await import('@tanstack/react-router')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const systemApi = await import('../../api')
const { SettingsPageProvider } =
  await import('../../components/settings-page-context')
const { getOptionValue, useSystemOptions } =
  await import('../../hooks/use-system-options')
const { ClaudeyeBrandAppearanceSection } =
  await import('../claudeye-brand-appearance-section')
const { getBrandAppearanceSections } = await import('../section-registry')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  fallbackLng: 'en',
  resources: { en: { translation: {} } },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

let updateOption: ReturnType<typeof vi.spyOn>
let getOptions: ReturnType<typeof vi.spyOn> | undefined

const customColors = {
  'brand_setting.claudeye_light_mark_color': '#111111',
  'brand_setting.claudeye_light_text_color': '#222222',
  'brand_setting.claudeye_dark_mark_color': '#EEEEEE',
  'brand_setting.claudeye_dark_text_color': '#DDDDDD',
}

const refreshedColors = {
  ...customColors,
  'brand_setting.claudeye_light_mark_color': '#FFFFFF',
}

function SectionHarness() {
  const [actionsContainer, setActionsContainer] =
    useState<HTMLDivElement | null>(null)
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: { mutations: { retry: false } },
      })
  )
  const [defaultValues, setDefaultValues] = useState(customColors)

  return (
    <QueryClientProvider client={queryClient}>
      <button type='button' onClick={() => setDefaultValues(refreshedColors)}>
        Refresh option values
      </button>
      <div ref={setActionsContainer} />
      <SettingsPageProvider
        actionsContainer={actionsContainer}
        suppressSectionHeader={false}
      >
        <ClaudeyeBrandAppearanceSection defaultValues={defaultValues} />
      </SettingsPageProvider>
    </QueryClientProvider>
  )
}

function QueryConnectedSection() {
  const [actionsContainer, setActionsContainer] =
    useState<HTMLDivElement | null>(null)
  const { data, isLoading, refetch } = useSystemOptions()

  if (isLoading) return null
  const settings = getOptionValue(data?.data, customColors)

  return (
    <>
      <button type='button' onClick={() => void refetch()}>
        Refetch system options
      </button>
      <output aria-label='Queried light mark'>
        {settings['brand_setting.claudeye_light_mark_color']}
      </output>
      <output aria-label='Queried dark text'>
        {settings['brand_setting.claudeye_dark_text_color']}
      </output>
      <div ref={setActionsContainer} />
      <SettingsPageProvider
        actionsContainer={actionsContainer}
        suppressSectionHeader={false}
      >
        <ClaudeyeBrandAppearanceSection defaultValues={settings} />
      </SettingsPageProvider>
    </>
  )
}

function QueryConnectedHarness() {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          mutations: { retry: false },
          queries: { retry: false },
        },
      })
  )

  return (
    <QueryClientProvider client={queryClient}>
      <QueryConnectedSection />
    </QueryClientProvider>
  )
}

async function renderSection(rootComponent = SectionHarness) {
  const rootRoute = createRootRoute({ component: rootComponent })
  const router = createRouter({
    routeTree: rootRoute,
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })

  await act(async () => {
    await router.load()
  })
  const result = render(
    <I18nextProvider i18n={i18n}>
      <RouterProvider router={router} />
    </I18nextProvider>
  )
  await act(async () => undefined)
  return result
}

describe('ClaudeyeBrandAppearanceSection', () => {
  afterAll(() => domWindow.close())

  beforeEach(() => {
    updateOption = vi
      .spyOn(systemApi, 'updateSystemOption')
      .mockResolvedValue({ success: true, message: '' })
  })

  afterEach(() => {
    cleanup()
    updateOption.mockRestore()
    getOptions?.mockRestore()
    getOptions = undefined
  })

  test('renders four native color pickers, HEX inputs, and visible swatches', async () => {
    const view = await renderSection()
    const labels = [
      'Light surface mark color',
      'Light surface wordmark color',
      'Dark surface mark color',
      'Dark surface wordmark color',
    ]

    for (const label of labels) {
      expect(view.getByLabelText(label).getAttribute('type')).toBe('text')
      expect(view.getByLabelText(`${label} picker`).getAttribute('type')).toBe(
        'color'
      )
    }
    expect(
      view.container.querySelectorAll('[data-brand-color-swatch]')
    ).toHaveLength(4)
  })

  test('keeps each picker and editable HEX input synchronized', async () => {
    const view = await renderSection()
    const textInput = view.getByLabelText('Light surface mark color')
    const picker = view.getByLabelText('Light surface mark color picker')

    await act(async () => {
      fireEvent.change(textInput, { target: { value: '#a1b2c3' } })
    })
    expect((picker as HTMLInputElement).value).toBe('#a1b2c3')

    await act(async () => {
      fireEvent.change(picker, { target: { value: '#abcdef' } })
    })
    expect((textInput as HTMLInputElement).value).toBe('#ABCDEF')
  })

  test('updates a surface preview only when both colors are valid and falls back on error', async () => {
    const view = await renderSection()
    const markInput = view.getByLabelText('Light surface mark color')
    const textInput = view.getByLabelText('Light surface wordmark color')
    let preview = view.getByRole('img', { name: 'Light header preview' })

    expect(preview.getAttribute('src')).toBe(
      '/api/branding/claudeye/wordmark.svg?surface=light&mark=%23111111&text=%23222222'
    )

    await act(async () => {
      fireEvent.change(markInput, { target: { value: '#123' } })
    })
    await act(async () => {
      fireEvent.change(textInput, { target: { value: '#abcdef' } })
    })
    expect(preview.getAttribute('src')).toBe(
      '/api/branding/claudeye/wordmark.svg?surface=light&mark=%23111111&text=%23222222'
    )

    await act(async () => {
      fireEvent.change(markInput, { target: { value: '#123456' } })
    })
    preview = view.getByRole('img', { name: 'Light header preview' })
    expect(preview.getAttribute('src')).toBe(
      '/api/branding/claudeye/wordmark.svg?surface=light&mark=%23123456&text=%23ABCDEF'
    )

    await act(async () => {
      fireEvent.error(preview)
    })
    expect(preview.getAttribute('src')).toBe('/claudeye-wordmark-neutral.png')
  })

  test('shows format validation and blocks saving partial HEX input', async () => {
    const view = await renderSection()
    const input = view.getByLabelText('Light surface mark color')

    fireEvent.change(input, { target: { value: '#123' } })

    expect(
      (await view.findByText('Color must use #RRGGBB format')).isConnected
    ).toBe(true)
    expect(
      (
        view.getByRole('button', {
          name: 'Save Changes',
        }) as HTMLButtonElement
      ).disabled
    ).toBe(true)
    expect(updateOption).not.toHaveBeenCalled()
  })

  test('warns about low contrast without blocking a valid save', async () => {
    const view = await renderSection()
    const input = view.getByLabelText('Light surface mark color')

    fireEvent.change(input, { target: { value: '#ffffff' } })

    expect((await view.findByText('Low contrast')).isConnected).toBe(true)
    const save = view.getByRole('button', {
      name: 'Save Changes',
    }) as HTMLButtonElement
    await waitFor(() => expect(save.disabled).toBe(false))
    fireEvent.click(save)

    await waitFor(() => {
      expect(updateOption).toHaveBeenCalledWith({
        key: 'brand_setting.claudeye_light_mark_color',
        value: '#FFFFFF',
      })
    })
  })

  test('restores all defaults as dirty values without saving until Save Changes', async () => {
    const view = await renderSection()

    fireEvent.click(
      view.getByRole('button', { name: 'Restore default colors' })
    )

    expect(
      (view.getByLabelText('Light surface mark color') as HTMLInputElement)
        .value
    ).toBe('#242424')
    expect(
      (view.getByLabelText('Light surface wordmark color') as HTMLInputElement)
        .value
    ).toBe('#6A6A6A')
    expect(
      (view.getByLabelText('Dark surface mark color') as HTMLInputElement).value
    ).toBe('#FFFFFF')
    expect(
      (view.getByLabelText('Dark surface wordmark color') as HTMLInputElement)
        .value
    ).toBe('#B8B8B8')
    expect(updateOption).not.toHaveBeenCalled()

    const save = view.getByRole('button', {
      name: 'Save Changes',
    }) as HTMLButtonElement
    await waitFor(() => expect(save.disabled).toBe(false))
    fireEvent.click(save)
    await waitFor(() => expect(updateOption).toHaveBeenCalledTimes(4))
  })

  test('submits changed values with normalized uppercase HEX', async () => {
    const view = await renderSection()

    fireEvent.change(view.getByLabelText('Dark surface wordmark color'), {
      target: { value: '#a1b2c3' },
    })
    const save = view.getByRole('button', {
      name: 'Save Changes',
    }) as HTMLButtonElement
    await waitFor(() => expect(save.disabled).toBe(false))
    fireEvent.click(save)

    await waitFor(() => {
      expect(updateOption).toHaveBeenCalledTimes(1)
      expect(updateOption).toHaveBeenCalledWith({
        key: 'brand_setting.claudeye_dark_text_color',
        value: '#A1B2C3',
      })
    })
  })

  test('retains attempted values for retry when the first option response is unsuccessful', async () => {
    updateOption.mockResolvedValueOnce({
      success: false,
      message: 'Database write failed',
    })
    const view = await renderSection()
    const markInput = view.getByLabelText('Light surface mark color')

    fireEvent.change(markInput, { target: { value: '#abcdef' } })
    const save = view.getByRole('button', {
      name: 'Save Changes',
    }) as HTMLButtonElement
    await waitFor(() => expect(save.disabled).toBe(false))
    fireEvent.click(save)

    await waitFor(() => expect(updateOption).toHaveBeenCalledTimes(1))
    expect((markInput as HTMLInputElement).value).toBe('#abcdef')
    await waitFor(() => expect(save.disabled).toBe(false))

    fireEvent.click(save)
    await waitFor(() => expect(updateOption).toHaveBeenCalledTimes(2))
    expect(updateOption).toHaveBeenLastCalledWith({
      key: 'brand_setting.claudeye_light_mark_color',
      value: '#ABCDEF',
    })
  })

  test('stops sequential saves after an unsuccessful later option and retains every edit', async () => {
    updateOption
      .mockResolvedValueOnce({ success: true, message: '' })
      .mockResolvedValueOnce({
        success: false,
        message: 'Database write failed',
      })
    const view = await renderSection()
    const lightMark = view.getByLabelText('Light surface mark color')
    const lightText = view.getByLabelText('Light surface wordmark color')
    const darkMark = view.getByLabelText('Dark surface mark color')

    fireEvent.change(lightMark, { target: { value: '#abcdef' } })
    fireEvent.change(lightText, { target: { value: '#123456' } })
    fireEvent.change(darkMark, { target: { value: '#654321' } })
    const save = view.getByRole('button', {
      name: 'Save Changes',
    }) as HTMLButtonElement
    await waitFor(() => expect(save.disabled).toBe(false))
    fireEvent.click(save)

    await waitFor(() => expect(updateOption).toHaveBeenCalledTimes(2))
    expect(updateOption.mock.calls).toEqual([
      [
        {
          key: 'brand_setting.claudeye_light_mark_color',
          value: '#ABCDEF',
        },
      ],
      [
        {
          key: 'brand_setting.claudeye_light_text_color',
          value: '#123456',
        },
      ],
    ])
    expect((lightMark as HTMLInputElement).value).toBe('#abcdef')
    expect((lightText as HTMLInputElement).value).toBe('#123456')
    expect((darkMark as HTMLInputElement).value).toBe('#654321')
    await waitFor(() => expect(save.disabled).toBe(false))
  })

  test('retains failed and unsent edits when an earlier save refetches system options', async () => {
    const serverColors = { ...customColors }
    getOptions = vi
      .spyOn(systemApi, 'getSystemOptions')
      .mockImplementation(async () => ({
        success: true,
        message: '',
        data: Object.entries(serverColors).map(([key, value]) => ({
          key,
          value,
        })),
      }))

    let updateCount = 0
    let resolveSecondUpdate:
      | ((response: { success: boolean; message: string }) => void)
      | undefined
    updateOption.mockImplementation(
      async (request: Parameters<typeof systemApi.updateSystemOption>[0]) => {
        updateCount += 1
        if (updateCount === 2) {
          return new Promise((resolve) => {
            resolveSecondUpdate = resolve
          })
        }

        serverColors[request.key as keyof typeof serverColors] = String(
          request.value
        )
        return { success: true, message: '' }
      }
    )

    const view = await renderSection(QueryConnectedHarness)
    const lightMark = (await view.findByLabelText(
      'Light surface mark color'
    )) as HTMLInputElement
    const lightText = view.getByLabelText(
      'Light surface wordmark color'
    ) as HTMLInputElement
    const darkMark = view.getByLabelText(
      'Dark surface mark color'
    ) as HTMLInputElement
    const darkText = view.getByLabelText(
      'Dark surface wordmark color'
    ) as HTMLInputElement
    const save = view.getByRole('button', {
      name: 'Save Changes',
    }) as HTMLButtonElement

    fireEvent.change(lightMark, { target: { value: '#abcdef' } })
    fireEvent.change(lightText, { target: { value: '#123456' } })
    fireEvent.change(darkMark, { target: { value: '#654321' } })
    await waitFor(() => expect(save.disabled).toBe(false))
    fireEvent.click(save)

    await waitFor(() => expect(updateOption).toHaveBeenCalledTimes(2))
    await waitFor(() => expect(getOptions).toHaveBeenCalledTimes(2))
    await act(async () => {
      resolveSecondUpdate?.({
        success: false,
        message: 'Database write failed',
      })
    })

    await waitFor(() => expect(save.disabled).toBe(false))
    expect(lightMark.value).toBe('#abcdef')
    expect(lightText.value).toBe('#123456')
    expect(darkMark.value).toBe('#654321')
    expect(updateOption).toHaveBeenCalledTimes(2)

    serverColors['brand_setting.claudeye_dark_text_color'] = '#CCCCCC'
    fireEvent.click(save)

    await waitFor(() => expect(updateOption).toHaveBeenCalledTimes(5))
    await waitFor(() => {
      expect(save.disabled).toBe(true)
      expect(lightMark.value).toBe('#ABCDEF')
      expect(lightText.value).toBe('#123456')
      expect(darkMark.value).toBe('#654321')
      expect(darkText.value).toBe('#CCCCCC')
    })
    expect(serverColors).toEqual({
      ...customColors,
      'brand_setting.claudeye_light_mark_color': '#ABCDEF',
      'brand_setting.claudeye_light_text_color': '#123456',
      'brand_setting.claudeye_dark_mark_color': '#654321',
      'brand_setting.claudeye_dark_text_color': '#CCCCCC',
    })
  })

  test('accepts a later clean refresh when a saved key was superseded before confirmation', async () => {
    const serverColors = { ...customColors }
    getOptions = vi
      .spyOn(systemApi, 'getSystemOptions')
      .mockImplementation(async () => ({
        success: true,
        message: '',
        data: Object.entries(serverColors).map(([key, value]) => ({
          key,
          value,
        })),
      }))
    updateOption.mockImplementation(
      async (request: Parameters<typeof systemApi.updateSystemOption>[0]) => {
        serverColors[request.key as keyof typeof serverColors] = String(
          request.value
        )
        serverColors['brand_setting.claudeye_light_mark_color'] = '#FFFFFF'
        return { success: true, message: '' }
      }
    )

    const view = await renderSection(QueryConnectedHarness)
    const lightMark = (await view.findByLabelText(
      'Light surface mark color'
    )) as HTMLInputElement
    const darkText = view.getByLabelText(
      'Dark surface wordmark color'
    ) as HTMLInputElement
    const save = view.getByRole('button', {
      name: 'Save Changes',
    }) as HTMLButtonElement

    fireEvent.change(lightMark, { target: { value: '#abcdef' } })
    await waitFor(() => expect(save.disabled).toBe(false))
    fireEvent.click(save)

    await waitFor(() => expect(updateOption).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(getOptions).toHaveBeenCalledTimes(2))
    await waitFor(() =>
      expect(view.getByLabelText('Queried light mark').textContent).toBe(
        '#FFFFFF'
      )
    )
    await waitFor(() => expect(save.disabled).toBe(true))
    expect(lightMark.value).toBe('#abcdef')

    serverColors['brand_setting.claudeye_dark_text_color'] = '#CCCCCC'
    fireEvent.click(
      view.getByRole('button', { name: 'Refetch system options' })
    )

    await waitFor(() => expect(getOptions).toHaveBeenCalledTimes(3))
    await waitFor(() =>
      expect(view.getByLabelText('Queried dark text').textContent).toBe(
        '#CCCCCC'
      )
    )
    await waitFor(
      () => {
        expect(lightMark.value).toBe('#FFFFFF')
        expect(darkText.value).toBe('#CCCCCC')
        expect(save.disabled).toBe(true)
      },
      { timeout: 2000 }
    )
  })

  test('reconciles a stable superseding snapshot within a bounded delay', async () => {
    const serverColors = { ...customColors }
    getOptions = vi
      .spyOn(systemApi, 'getSystemOptions')
      .mockImplementation(async () => ({
        success: true,
        message: '',
        data: Object.entries(serverColors).map(([key, value]) => ({
          key,
          value,
        })),
      }))
    updateOption.mockImplementation(
      async (request: Parameters<typeof systemApi.updateSystemOption>[0]) => {
        serverColors[request.key as keyof typeof serverColors] = String(
          request.value
        )
        serverColors['brand_setting.claudeye_light_mark_color'] = '#FFFFFF'
        return { success: true, message: '' }
      }
    )

    const view = await renderSection(QueryConnectedHarness)
    const lightMark = (await view.findByLabelText(
      'Light surface mark color'
    )) as HTMLInputElement
    const save = view.getByRole('button', {
      name: 'Save Changes',
    }) as HTMLButtonElement

    fireEvent.change(lightMark, { target: { value: '#abcdef' } })
    await waitFor(() => expect(save.disabled).toBe(false))
    fireEvent.click(save)

    await waitFor(() => expect(updateOption).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(getOptions).toHaveBeenCalledTimes(2))
    await waitFor(() =>
      expect(view.getByLabelText('Queried light mark').textContent).toBe(
        '#FFFFFF'
      )
    )
    await waitFor(() => expect(save.disabled).toBe(true))
    expect(lightMark.value).toBe('#abcdef')

    fireEvent.click(
      view.getByRole('button', { name: 'Refetch system options' })
    )
    await waitFor(() => expect(getOptions).toHaveBeenCalledTimes(3))
    expect(view.getByLabelText('Queried light mark').textContent).toBe(
      '#FFFFFF'
    )

    await waitFor(() => expect(lightMark.value).toBe('#FFFFFF'), {
      timeout: 2000,
    })
    expect(save.disabled).toBe(true)
  })

  test('reconciles when the authoritative snapshot restores the pre-save palette', async () => {
    const serverColors = { ...customColors }
    getOptions = vi
      .spyOn(systemApi, 'getSystemOptions')
      .mockImplementation(async () => ({
        success: true,
        message: '',
        data: Object.entries(serverColors).map(([key, value]) => ({
          key,
          value,
        })),
      }))
    updateOption.mockImplementation(
      async (request: Parameters<typeof systemApi.updateSystemOption>[0]) => {
        serverColors[request.key as keyof typeof serverColors] = String(
          request.value
        )
        serverColors['brand_setting.claudeye_light_mark_color'] = '#111111'
        return { success: true, message: '' }
      }
    )

    const view = await renderSection(QueryConnectedHarness)
    const lightMark = (await view.findByLabelText(
      'Light surface mark color'
    )) as HTMLInputElement
    const save = view.getByRole('button', {
      name: 'Save Changes',
    }) as HTMLButtonElement

    fireEvent.change(lightMark, { target: { value: '#abcdef' } })
    await waitFor(() => expect(save.disabled).toBe(false))
    fireEvent.click(save)

    await waitFor(() => expect(updateOption).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(getOptions).toHaveBeenCalledTimes(2))
    expect(view.getByLabelText('Queried light mark').textContent).toBe(
      '#111111'
    )
    await waitFor(() => expect(save.disabled).toBe(true))
    expect(lightMark.value).toBe('#abcdef')

    await waitFor(() => expect(lightMark.value).toBe('#111111'), {
      timeout: 2000,
    })
    expect(save.disabled).toBe(true)
  })

  test('synchronizes the last valid palette when option values refresh', async () => {
    const view = await renderSection()
    const markInput = view.getByLabelText('Light surface mark color')
    const picker = view.getByLabelText('Light surface mark color picker')

    await act(async () => {
      fireEvent.click(
        view.getByRole('button', { name: 'Refresh option values' })
      )
    })

    await waitFor(() =>
      expect((markInput as HTMLInputElement).value).toBe('#FFFFFF')
    )
    expect(
      (
        view.getByRole('img', {
          name: 'Light header preview',
        }) as HTMLImageElement
      ).src
    ).toContain('surface=light&mark=%23FFFFFF&text=%23222222')
    expect((picker as HTMLInputElement).value).toBe('#ffffff')
    expect(view.getByText('Low contrast').isConnected).toBe(true)

    await act(async () => {
      fireEvent.change(markInput, { target: { value: '#FFF' } })
    })

    expect(
      (
        view.getByRole('img', {
          name: 'Light header preview',
        }) as HTMLImageElement
      ).src
    ).toContain('surface=light&mark=%23FFFFFF&text=%23222222')
    expect((picker as HTMLInputElement).value).toBe('#ffffff')
    const swatch = markInput
      .closest('[data-slot="form-item"]')
      ?.querySelector<HTMLElement>('[data-brand-color-swatch]')
    expect(swatch?.style.backgroundColor).toBe('#FFFFFF')
    expect(view.getByText('Low contrast').isConnected).toBe(true)
  })

  test('registers brand appearance only for claudeye builds', () => {
    expect(getBrandAppearanceSections('molii')).toHaveLength(0)
    expect(getBrandAppearanceSections('ixiaozu')).toHaveLength(0)
    expect(getBrandAppearanceSections('claudeye').map(({ id }) => id)).toEqual([
      'brand-appearance',
    ])
  })
})
