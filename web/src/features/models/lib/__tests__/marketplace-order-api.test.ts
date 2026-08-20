import assert from 'node:assert/strict'
import { after, test } from 'node:test'

const { api } = await import('@/lib/api')
const { getModelOrder, getVendorOrder, saveModelOrder, saveVendorOrder } =
  await import('../../api')

type ApiClient = {
  get: (url: string) => Promise<{ data: unknown }>
  put: (url: string, body: unknown) => Promise<{ data: unknown }>
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
  const response = { success: true, data: [{ id: 3, display_order: 1 }] }
  apiClient.get = async (nextURL) => {
    url = nextURL
    return { data: response }
  }

  assert.deepEqual(await getModelOrder(), response)
  assert.equal(url, '/api/models/order')
})

test('saves complete model order', async () => {
  let url = ''
  let body: unknown
  apiClient.put = async (nextURL, nextBody) => {
    url = nextURL
    body = nextBody
    return { data: { success: true } }
  }

  await saveModelOrder([3, 1, 2])

  assert.equal(url, '/api/models/order')
  assert.deepEqual(body, { ordered_ids: [3, 1, 2] })
})

test('gets the persisted vendor order', async () => {
  let url = ''
  const response = { success: true, data: [{ id: 2, display_order: 1 }] }
  apiClient.get = async (nextURL) => {
    url = nextURL
    return { data: response }
  }

  assert.deepEqual(await getVendorOrder(), response)
  assert.equal(url, '/api/vendors/order')
})

test('saves complete vendor order', async () => {
  let url = ''
  let body: unknown
  apiClient.put = async (nextURL, nextBody) => {
    url = nextURL
    body = nextBody
    return { data: { success: true } }
  }

  await saveVendorOrder([2, 3, 1])

  assert.equal(url, '/api/vendors/order')
  assert.deepEqual(body, { ordered_ids: [2, 3, 1] })
})
