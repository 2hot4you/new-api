import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { renderToStaticMarkup } from 'react-dom/server'

import { getDiceBearDylanAvatarUrl } from '../../lib/avatar'
import { UserAvatar } from '../user-avatar'

describe('current user avatar', () => {
  test('builds a stable Dylan URL from only the numeric user ID', () => {
    const url = getDiceBearDylanAvatarUrl(2205)

    assert.equal(
      url,
      'https://api.dicebear.com/10.x/dylan/svg?seed=molii-user-2205'
    )
    assert.doesNotMatch(url, /alice|example\.com/i)
  })

  test('keeps the initials fallback while the Dylan image loads', () => {
    const markup = renderToStaticMarkup(
      <UserAvatar userId={2205} name='Alice Example' />
    )

    assert.match(markup, />A<\/span>/)
  })
})
