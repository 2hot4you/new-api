import assert from 'node:assert/strict'
import { afterAll as after, test } from 'vitest'

const { api } = await import('@/lib/api')
const { getModelOrder, getVendorOrder, saveModelOrder, saveVendorOrder } =
  await import('../../api')

type ApiClient = {
  get: (
    url: string,
    config?: { skipBusinessError?: boolean }
  ) => Promise<{ data: unknown }>
  put: (
    url: string,
    body: unknown,
    config?: { skipBusinessError?: boolean }
  ) => Promise<{ data: unknown }>
}

const apiClient = api as unknown as ApiClient
const originalGet = apiClient.get
const originalPut = apiClient.put

after(() => {
  apiClient.get = originalGet
  apiClient.put = originalPut
})

test('gets the persisted model order', async () => {
  let url = ''
  let config: { skipBusinessError?: boolean } | undefined
  const response = { success: true, data: [{ id: 3, display_order: 1 }] }
  apiClient.get = async (nextURL, nextConfig) => {
    url = nextURL
    config = nextConfig
    return { data: response }
  }

  assert.deepEqual(await getModelOrder(), response)
  assert.equal(url, '/api/models/order')
  assert.deepEqual(config, { skipBusinessError: true })
})

test('saves complete model order', async () => {
  let url = ''
  let body: unknown
  let config: { skipBusinessError?: boolean } | undefined
  apiClient.put = async (nextURL, nextBody, nextConfig) => {
    url = nextURL
    body = nextBody
    config = nextConfig
    return { data: { success: true } }
  }

  await saveModelOrder([3, 1, 2])

  assert.equal(url, '/api/models/order')
  assert.deepEqual(body, { ordered_ids: [3, 1, 2] })
  assert.deepEqual(config, { skipBusinessError: true })
})

test('gets the persisted vendor order', async () => {
  let url = ''
  let config: { skipBusinessError?: boolean } | undefined
  const response = { success: true, data: [{ id: 2, display_order: 1 }] }
  apiClient.get = async (nextURL, nextConfig) => {
    url = nextURL
    config = nextConfig
    return { data: response }
  }

  assert.deepEqual(await getVendorOrder(), response)
  assert.equal(url, '/api/vendors/order')
  assert.deepEqual(config, { skipBusinessError: true })
})

test('saves complete vendor order', async () => {
  let url = ''
  let body: unknown
  let config: { skipBusinessError?: boolean } | undefined
  apiClient.put = async (nextURL, nextBody, nextConfig) => {
    url = nextURL
    body = nextBody
    config = nextConfig
    return { data: { success: true } }
  }

  await saveVendorOrder([2, 3, 1])

  assert.equal(url, '/api/vendors/order')
  assert.deepEqual(body, { ordered_ids: [2, 3, 1] })
  assert.deepEqual(config, { skipBusinessError: true })
})
