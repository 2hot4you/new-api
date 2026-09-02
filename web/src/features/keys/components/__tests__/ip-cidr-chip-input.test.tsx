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
import { afterAll as after, afterEach, describe, test } from 'vitest'

import { Window } from 'happy-dom'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'HTMLInputElement',
  'Node',
  'Event',
  'KeyboardEvent',
  'MouseEvent',
  'FocusEvent',
  'customElements',
  'MutationObserver',
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

const { act, useState } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { IpCidrChipInput } = await import('../ip-cidr-chip-input')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

let host: HTMLDivElement | null = null
let root: ReturnType<typeof createRoot> | null = null

function findButton(name: string): HTMLButtonElement {
  const button = [
    ...document.querySelectorAll<HTMLButtonElement>('button'),
  ].find(
    (candidate) =>
      candidate.getAttribute('aria-label') === name ||
      candidate.textContent === name
  )
  assert.ok(button, `Expected button named "${name}"`)
  return button
}

function getInput(): HTMLInputElement {
  const input = document.querySelector<HTMLInputElement>(
    'input[aria-label="IP address or CIDR"]'
  )
  assert.ok(input, 'Expected IP address input')
  return input
}

async function changeInput(value: string): Promise<void> {
  const input = getInput()
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

async function addWithButton(value: string): Promise<void> {
  await changeInput(value)
  await act(async () => findButton('Add IP address').click())
}

async function pasteIntoInput(value: string): Promise<void> {
  await act(async () => {
    const event = new domWindow.Event('paste', {
      bubbles: true,
      cancelable: true,
    }) as unknown as Event & {
      clipboardData: { getData: (type: string) => string }
    }
    Object.defineProperty(event, 'clipboardData', {
      value: { getData: () => value },
    })
    getInput().dispatchEvent(event)
  })
}

function ControlledIpCidrChipInput(props: {
  onChange: (value: string) => void
  onValidityChange?: (valid: boolean) => void
}) {
  const [value, setValue] = useState('')

  return (
    <IpCidrChipInput
      value={value}
      onChange={(nextValue) => {
        setValue(nextValue)
        props.onChange(nextValue)
      }}
      onValidityChange={props.onValidityChange}
    />
  )
}

async function renderInput(
  onChange: (value: string) => void,
  onValidityChange?: (valid: boolean) => void
): Promise<void> {
  host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)
  await act(async () =>
    root?.render(
      <I18nextProvider i18n={i18n}>
        <ControlledIpCidrChipInput
          onChange={onChange}
          onValidityChange={onValidityChange}
        />
      </I18nextProvider>
    )
  )
}

async function renderPreloadedInput(
  value: string,
  onChange: (value: string) => void,
  resetKey = 'first-record',
  onDraftStateChange?: (state: { hasDraft: boolean; isValid: boolean }) => void
): Promise<void> {
  if (!host) {
    host = document.createElement('div')
    document.body.append(host)
    root = createRoot(host)
  }
  await act(async () =>
    root?.render(
      <I18nextProvider i18n={i18n}>
        <IpCidrChipInput
          value={value}
          onChange={onChange}
          resetKey={resetKey}
          onDraftStateChange={onDraftStateChange}
        />
      </I18nextProvider>
    )
  )
}

afterEach(async () => {
  if (root) {
    await act(async () => root?.unmount())
  }
  host?.remove()
  host = null
  root = null
})

after(() => domWindow.close())

describe('IP CIDR chip input', () => {
  test('adds IPv4, IPv6, IPv4 CIDR, and normalized IPv6 CIDR in insertion order', async () => {
    const values: string[] = []
    await renderInput((value) => values.push(value))

    await addWithButton('192.0.2.1')
    await addWithButton('2001:db8::1')
    await addWithButton('198.51.100.0/24')
    await addWithButton('2001:0DB8:0:0::/64')

    assert.equal(
      values.at(-1),
      '192.0.2.1\n2001:db8::1\n198.51.100.0/24\n2001:db8::/64'
    )
  })

  test('splits a comma, space, and newline paste into ordered entries', async () => {
    const values: string[] = []
    await renderInput((value) => values.push(value))

    await pasteIntoInput('192.0.2.1, 2001:db8::1\n198.51.100.0/24')

    assert.equal(values.at(-1), '192.0.2.1\n2001:db8::1\n198.51.100.0/24')
  })

  test('keeps valid batch entries when a pasted candidate is invalid', async () => {
    const values: string[] = []
    await renderInput((value) => values.push(value))

    await pasteIntoInput('192.0.2.1, 300.0.0.1, 2001:db8::1')

    assert.deepEqual(values, ['192.0.2.1\n2001:db8::1'])
    assert.equal(
      document.querySelector<HTMLElement>('[role="alert"]')?.textContent,
      'Enter a valid IP address or CIDR'
    )
    assert.equal(getInput().value, '300.0.0.1')
  })

  test('does not add an entry that already has a chip', async () => {
    const values: string[] = []
    await renderInput((value) => values.push(value))

    await addWithButton('192.0.2.1')
    await addWithButton('192.0.2.1')

    assert.deepEqual(values, ['192.0.2.1'])
  })

  test('rejects leading-zero IPv4 forms and accepts IPv4 and IPv6 prefix boundaries', async () => {
    const values: string[] = []
    await renderInput((value) => values.push(value))

    await addWithButton('192.0.002.1/24')
    await addWithButton('0.0.0.0/0')
    await addWithButton('192.0.2.1/32')
    await addWithButton('::/0')
    await addWithButton('2001:db8::1/128')

    assert.equal(
      values.at(-1),
      '0.0.0.0/0\n192.0.2.1/32\n::/0\n2001:db8::1/128'
    )
  })

  test('suppresses canonical-equivalent IPv6 entries', async () => {
    const values: string[] = []
    await renderInput((value) => values.push(value))

    await addWithButton('2001:0DB8:0:0::1')
    await addWithButton('2001:db8::1')

    assert.deepEqual(values, ['2001:db8::1'])
  })

  test('canonicalizes CIDR host bits before deduplicating equivalent networks', async () => {
    const values: string[] = []
    await renderInput((value) => values.push(value))

    await addWithButton('192.0.2.99/24')
    await addWithButton('192.0.2.0/24')
    await addWithButton('2001:db8::99/64')
    await addWithButton('2001:db8::/64')

    assert.deepEqual(values, ['192.0.2.0/24', '192.0.2.0/24\n2001:db8::/64'])
  })

  test('normalizes and deduplicates preloaded valid entries into the controlled value', async () => {
    const values: string[] = []
    await renderPreloadedInput(
      '2001:0DB8:0:0::1\n2001:db8::1\nlegacy-address',
      (value) => values.push(value)
    )

    assert.deepEqual(
      [...document.querySelectorAll<HTMLElement>('[data-ip-cidr-chip]')].map(
        (chip) => chip.getAttribute('data-ip-cidr-chip')
      ),
      ['2001:db8::1', 'legacy-address']
    )
    assert.deepEqual(values, ['2001:db8::1\nlegacy-address'])
    await renderPreloadedInput('2001:db8::1\nlegacy-address', (value) =>
      values.push(value)
    )
    await addWithButton('192.0.2.1')

    assert.deepEqual(values, ['2001:db8::1\nlegacy-address'])
    assert.equal(
      document.querySelector<HTMLElement>('[role="alert"]')?.textContent,
      'Remove invalid saved entries before adding another IP address'
    )
  })

  test('clears an uncommitted draft when resetKey changes for another record', async () => {
    const values: string[] = []
    await renderPreloadedInput('192.0.2.1', (value) => values.push(value))
    await changeInput('300.0.0.1')
    assert.equal(getInput().value, '300.0.0.1')

    await renderPreloadedInput(
      '192.0.2.1',
      (value) => values.push(value),
      'second-record'
    )

    assert.equal(getInput().value, '')
    assert.equal(document.querySelector('[role="alert"]'), null)
    assert.deepEqual(values, [])
  })

  test('renders the latest controlled entries after a prop update', async () => {
    const values: string[] = []
    await renderPreloadedInput('192.0.2.1', (value) => values.push(value))
    await renderPreloadedInput('2001:0DB8::1\n198.51.100.0/24', (value) =>
      values.push(value)
    )

    assert.deepEqual(
      [...document.querySelectorAll<HTMLElement>('[data-ip-cidr-chip]')].map(
        (chip) => chip.getAttribute('data-ip-cidr-chip')
      ),
      ['2001:db8::1', '198.51.100.0/24']
    )
    assert.deepEqual(values, ['2001:db8::1\n198.51.100.0/24'])
  })

  test('keeps an uncommitted draft after removing an existing chip', async () => {
    const values: string[] = []
    const draftStates: Array<{ hasDraft: boolean; isValid: boolean }> = []
    await renderPreloadedInput(
      '192.0.2.1',
      (value) => values.push(value),
      'first-record',
      (state) => draftStates.push(state)
    )

    await changeInput('2001:db8::1')
    await act(async () => findButton('Remove 192.0.2.1').click())

    assert.equal(getInput().value, '2001:db8::1')
    assert.deepEqual(draftStates.at(-1), { hasDraft: true, isValid: true })
    assert.deepEqual(values, [''])
  })

  test('shows invalid address, prefix, and zone identifier feedback without adding entries', async () => {
    const values: string[] = []
    const validity: boolean[] = []
    await renderInput(
      (value) => values.push(value),
      (valid) => validity.push(valid)
    )

    await addWithButton('300.0.0.1')
    await addWithButton('2001:db8::1/129')
    await addWithButton('fe80::1%en0')

    const feedback = document.querySelector<HTMLElement>('[role="alert"]')
    assert.equal(feedback?.textContent, 'Enter a valid IP address or CIDR')
    assert.deepEqual(values, [])
    assert.equal(validity.at(-1), false)
  })

  test('adds the current entry when Enter is pressed', async () => {
    const values: string[] = []
    await renderInput((value) => values.push(value))

    await changeInput('192.0.2.1')
    await act(async () =>
      getInput().dispatchEvent(
        new domWindow.KeyboardEvent('keydown', {
          bubbles: true,
          key: 'Enter',
        }) as unknown as Event
      )
    )

    assert.deepEqual(values, ['192.0.2.1'])
  })

  test('removes a chip and serializes the remaining entries', async () => {
    const values: string[] = []
    await renderInput((value) => values.push(value))
    await addWithButton('192.0.2.1')
    await addWithButton('2001:db8::1')

    await act(async () => findButton('Remove 192.0.2.1').click())

    assert.equal(values.at(-1), '2001:db8::1')
  })
})
