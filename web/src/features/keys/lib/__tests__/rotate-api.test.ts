import assert from 'node:assert/strict'
import { after, test } from 'node:test'

const { api } = await import('@/lib/api')
const { rotateApiKey } = await import('../../api')

type ApiClient = {
  post: (url: string, data?: unknown) => Promise<{ data: unknown }>
}

const apiClient = api as unknown as ApiClient
const originalPost = apiClient.post

after(() => {
  apiClient.post = originalPost
})

test('rotate API explicitly confirms the key rotation', async () => {
  let url = ''
  let body: unknown
  apiClient.post = async (nextURL, nextBody) => {
    url = nextURL
    body = nextBody
    return { data: { success: true, data: { key: 'new-key' } } }
  }

  const result = await rotateApiKey(42)

  assert.equal(url, '/api/token/42/rotate')
  assert.deepEqual(body, { confirm: true })
  assert.deepEqual(result, { success: true, data: { key: 'new-key' } })
})
