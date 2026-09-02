import assert from 'node:assert/strict'
import { describe, test } from 'vitest'

import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider, initReactI18next } from 'react-i18next'

import { ModelPublicationStatus } from '../model-publication-status'

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en' })

function renderPublicationStatus(
  overrides: Partial<React.ComponentProps<typeof ModelPublicationStatus>> = {}
) {
  return renderToStaticMarkup(
    <I18nextProvider i18n={i18n}>
      <ModelPublicationStatus
        enabled
        onEnabledChange={() => {}}
        modelEnabled
        complete
        missingFields={[]}
        visible
        blockers={[]}
        withdrawn={false}
        {...overrides}
      />
    </I18nextProvider>
  )
}

describe('model marketplace publication status', () => {
  test('renders the enabled publication state for a complete visible model', () => {
    const markup = renderPublicationStatus()

    assert.match(markup, /data-marketplace-publication="true"/)
    assert.match(markup, /aria-checked="true"/)
    assert.match(markup, /Published in model marketplace/)
    assert.doesNotMatch(markup, /Missing required metadata/)
  })

  test('lists every missing field and disables only a new publication request', () => {
    const markup = renderPublicationStatus({
      enabled: false,
      complete: false,
      visible: false,
      missingFields: ['description', 'supported_resolutions'],
    })

    assert.match(markup, /Missing required metadata/)
    assert.match(markup, /Chinese description/)
    assert.match(markup, /Supported resolutions/)
    assert.match(markup, /disabled=""/)
  })

  test('shows runtime blockers without disabling an existing publication intent', () => {
    const markup = renderPublicationStatus({
      enabled: true,
      visible: false,
      blockers: ['pricing_missing', 'endpoint_unavailable'],
    })

    assert.match(markup, /Pricing is not configured/)
    assert.match(markup, /No available endpoint/)
    assert.doesNotMatch(markup, /disabled=""/)
  })

  test('shows the automatic withdrawal notice returned by the backend', () => {
    const markup = renderPublicationStatus({
      enabled: false,
      complete: false,
      visible: false,
      missingFields: ['description'],
      withdrawn: true,
    })

    assert.match(markup, /automatically withdrawn/i)
  })
})
