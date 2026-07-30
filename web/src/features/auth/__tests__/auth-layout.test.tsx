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
import { renderToStaticMarkup } from 'react-dom/server'

import { AuthLayoutFrame } from '../auth-layout'

const domWindow = new Window()

describe('Molii authentication layout', () => {
  after(() => {
    domWindow.close()
  })

  test('provides semantic workspace and content structure with dynamic branding', () => {
    const markup = renderToStaticMarkup(
      <AuthLayoutFrame
        systemName='Molii'
        logo='/molii.svg'
        loading={false}
        labels={{
          logo: 'Brand logo',
          workspace: 'Creative workspace',
          tagline: 'Create without limits',
          image: 'Image',
          video: 'Video',
          audio: 'Audio',
        }}
      >
        <form aria-label='Authentication form' />
      </AuthLayoutFrame>
    )

    domWindow.document.body.innerHTML = markup

    const main = domWindow.document.querySelector('main')
    const workspace = domWindow.document.querySelector(
      'aside[aria-label="Creative workspace"]'
    )
    const content = domWindow.document.querySelector('main > section')
    const homeLink = domWindow.document.querySelector(
      'header a[href="/"]'
    ) as HTMLAnchorElement | null
    const logo = domWindow.document.querySelector(
      'img[alt="Brand logo"]'
    ) as HTMLImageElement | null

    assert.ok(main)
    assert.ok(workspace)
    assert.ok(content)
    assert.equal(
      domWindow.document.querySelectorAll('[aria-label="Creative workspace"]')
        .length,
      1
    )
    assert.ok(homeLink)
    assert.equal(homeLink.textContent?.includes('Molii'), true)
    assert.equal(logo?.getAttribute('src'), '/molii.svg')
    assert.ok(content.querySelector('form[aria-label="Authentication form"]'))
  })

  test('keeps the authentication content usable while brand data is loading', () => {
    const markup = renderToStaticMarkup(
      <AuthLayoutFrame
        systemName='Molii'
        logo='/molii.svg'
        loading
        labels={{
          logo: 'Brand logo',
          workspace: 'Creative workspace',
          tagline: 'Create without limits',
          image: 'Image',
          video: 'Video',
          audio: 'Audio',
        }}
      >
        <button type='submit'>Continue</button>
      </AuthLayoutFrame>
    )

    domWindow.document.body.innerHTML = markup

    assert.equal(domWindow.document.querySelector('header img'), null)
    assert.ok(
      domWindow.document.querySelector('main > section button[type="submit"]')
    )
    assert.equal(
      domWindow.document.querySelector('header a')?.getAttribute('aria-label'),
      'Molii'
    )
    assert.equal(
      domWindow.document.querySelector('aside')?.textContent?.includes('Image'),
      true
    )
  })
})
