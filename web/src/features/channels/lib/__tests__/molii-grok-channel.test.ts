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
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { describe, test } from 'node:test'
import { fileURLToPath } from 'node:url'

import {
  CHANNEL_TYPES,
  CHANNEL_TYPE_MOLII_GROK_AIGC,
  CHANNEL_TYPE_OPTIONS,
  MOLII_GROK_AIGC_MODELS,
} from '../../constants'
import {
  channelFormSchema,
  transformChannelToFormDefaults,
  transformFormDataToCreatePayload,
  transformFormDataToUpdatePayload,
} from '../channel-form'
import {
  getChannelTypeConfig,
  shouldShowBaseUrlField,
} from '../channel-type-config'
import {
  getChannelTestAction,
  getChannelTypeIcon,
  getRelatedModelsForChannelType,
} from '../channel-utils'

describe('Molii Grok Imagine API channel', () => {
  test('registers channel 62 without changing channel 61', () => {
    assert.equal(CHANNEL_TYPE_MOLII_GROK_AIGC, 62)
    assert.equal(CHANNEL_TYPES[61], 'Molii Volcengine Imagine API')
    assert.equal(CHANNEL_TYPES[62], 'Molii Grok Imagine API')
    assert.deepEqual(
      CHANNEL_TYPE_OPTIONS.find(
        (item) => item.value === CHANNEL_TYPE_MOLII_GROK_AIGC
      ),
      { value: CHANNEL_TYPE_MOLII_GROK_AIGC, label: 'Molii Grok Imagine API' }
    )
    assert.equal(getChannelTypeIcon(CHANNEL_TYPE_MOLII_GROK_AIGC), 'XAI')
  })

  test('fills the two image models and all supported video model IDs', () => {
    const expectedModels = [
      'grok-imagine-image',
      'grok-imagine-image-quality',
      'grok-imagine-video',
      'grok-imagine-video-1.5',
    ]
    assert.deepEqual(MOLII_GROK_AIGC_MODELS, expectedModels)
    assert.deepEqual(
      getRelatedModelsForChannelType(CHANNEL_TYPE_MOLII_GROK_AIGC, []),
      expectedModels
    )
    assert.deepEqual(getChannelTypeConfig(CHANNEL_TYPE_MOLII_GROK_AIGC), {
      id: CHANNEL_TYPE_MOLII_GROK_AIGC,
      name: 'Molii Grok Imagine API',
      icon: 'XAI',
      supportedModels: [...MOLII_GROK_AIGC_MODELS],
      hints: {
        key: 'Enter API key for this channel',
        models: MOLII_GROK_AIGC_MODELS.join(','),
        other: 'TCP reachability test only; no paid generation request is sent',
      },
    })
  })

  test('hides Base URL and uses a TCP reachability test action', () => {
    assert.equal(shouldShowBaseUrlField(CHANNEL_TYPE_MOLII_GROK_AIGC), false)
    assert.deepEqual(getChannelTestAction(CHANNEL_TYPE_MOLII_GROK_AIGC), {
      direct: true,
      label: 'Reachability Test',
    })
  })

  test('requires Key but does not require Base URL or provider-specific fields', () => {
    const valid = {
      name: 'Grok',
      type: CHANNEL_TYPE_MOLII_GROK_AIGC,
      base_url: '',
      key: 'sk-placeholder',
      models: MOLII_GROK_AIGC_MODELS.join(','),
      group: ['default'],
      status: 1,
      molii_grok_management_access_token: 'management-token-placeholder',
      molii_grok_management_user_id: 2205,
    }
    assert.equal(channelFormSchema.safeParse(valid).success, true)
    const invalid = channelFormSchema.safeParse({ ...valid, key: '   ' })
    assert.equal(invalid.success, false)
    assert.equal(
      invalid.error?.issues.some((issue) => issue.path[0] === 'key'),
      true
    )
  })

  test('preserves the saved API key when editing with the key field left blank', () => {
    const editValues = {
      name: 'Grok',
      type: CHANNEL_TYPE_MOLII_GROK_AIGC,
      base_url: '',
      key: '',
      models: MOLII_GROK_AIGC_MODELS.join(','),
      group: ['default'],
      status: 1,
      is_editing: true,
      molii_grok_management_access_token: '',
      molii_grok_management_user_id: 2205,
      molii_grok_management_access_token_configured: true,
    }

    const parsed = channelFormSchema.safeParse(editValues)
    assert.equal(
      parsed.success,
      true,
      parsed.error?.issues.map((issue) => issue.message).join(', ') || ''
    )
    if (!parsed.success) return

    const payload = transformFormDataToUpdatePayload(parsed.data, 62)
    assert.equal('key' in payload, false)
  })

  test('requires a complete management credential pair when creating a channel', () => {
    const base = {
      name: 'Grok',
      type: CHANNEL_TYPE_MOLII_GROK_AIGC,
      base_url: '',
      key: 'sk-placeholder',
      models: MOLII_GROK_AIGC_MODELS.join(','),
      group: ['default'],
      status: 1,
      is_editing: false,
    }

    const missing = channelFormSchema.safeParse(base)
    assert.equal(missing.success, false)
    assert.equal(
      missing.error?.issues.some(
        (issue) => issue.path[0] === 'molii_grok_management_access_token'
      ),
      true
    )
    assert.equal(
      missing.error?.issues.some(
        (issue) => issue.path[0] === 'molii_grok_management_user_id'
      ),
      true
    )

    assert.equal(
      channelFormSchema.safeParse({
        ...base,
        molii_grok_management_access_token: 'management-token-placeholder',
        molii_grok_management_user_id: 2205,
      }).success,
      true
    )
  })

  test('writes management credentials outside the channel JSON on create', () => {
    const payload = transformFormDataToCreatePayload({
      name: 'Grok',
      type: CHANNEL_TYPE_MOLII_GROK_AIGC,
      base_url: '',
      key: 'sk-placeholder',
      models: MOLII_GROK_AIGC_MODELS.join(','),
      group: ['default'],
      status: 1,
      molii_grok_management_access_token: 'management-token-placeholder',
      molii_grok_management_user_id: 2205,
      clear_molii_grok_management_access_token: false,
      is_editing: false,
    })

    assert.equal(
      payload.molii_grok_management_access_token,
      'management-token-placeholder'
    )
    assert.equal(payload.molii_grok_management_user_id, 2205)
    assert.equal('molii_grok_management_access_token' in payload.channel, false)
  })

  test('keeps the management token write-only when editing', () => {
    const defaults = transformChannelToFormDefaults({
      id: 62,
      type: CHANNEL_TYPE_MOLII_GROK_AIGC,
      key: '',
      status: 1,
      name: 'Grok',
      created_time: 0,
      test_time: 0,
      response_time: 0,
      base_url: '',
      other: '',
      balance: 0,
      balance_updated_time: 0,
      models: MOLII_GROK_AIGC_MODELS.join(','),
      group: 'default',
      used_quota: 0,
      other_info: '',
      remark: '',
      max_input_tokens: 0,
      channel_info: {
        is_multi_key: false,
        multi_key_size: 0,
        multi_key_polling_index: 0,
        multi_key_mode: 'random',
      },
      settings: '{}',
      molii_grok_management_user_id: 2205,
      molii_grok_management_access_token_configured: true,
    })

    assert.equal(defaults.is_editing, true)
    assert.equal(defaults.molii_grok_management_access_token, '')
    assert.equal(defaults.molii_grok_management_user_id, 2205)
    assert.equal(defaults.molii_grok_management_access_token_configured, true)

    const update = transformFormDataToUpdatePayload(defaults, 62)
    assert.equal('molii_grok_management_access_token' in update, false)
    assert.equal(update.molii_grok_management_user_id, 2205)

    const clearUpdate = transformFormDataToUpdatePayload(
      {
        ...defaults,
        clear_molii_grok_management_access_token: true,
      },
      62
    )
    assert.equal(clearUpdate.clear_molii_grok_management_access_token, true)
    assert.equal('molii_grok_management_access_token' in clearUpdate, false)
    assert.equal('molii_grok_management_user_id' in clearUpdate, false)
  })

  test('does not embed the private upstream brand or domain in frontend files', () => {
    const files = [
      '../../constants.ts',
      '../channel-type-config.ts',
      '../channel-utils.ts',
    ]
    const source = files
      .map((relativePath) =>
        readFileSync(
          fileURLToPath(new URL(relativePath, import.meta.url)),
          'utf8'
        )
      )
      .join('\n')
      .toLowerCase()
    const privateBrand = ['wxi', 'ai'].join('')
    const privateDomain = ['api', privateBrand, 'com'].join('.')
    assert.equal(source.includes(privateBrand), false)
    assert.equal(source.includes(privateDomain), false)
  })

  test('maps the exact reachability success message in every locale', () => {
    const messageKey = '可达性测试通过，未发送付费请求'
    const obsoleteKeys = [
      'Configuration Check',
      'Configuration check only; no paid generation request is sent',
      '配置校验通过，未发起付费请求',
      '可达性测试通过，未发起付费请求',
    ]
    const expectedTranslations: Record<string, string> = {
      en: 'Reachability test passed; no paid request was sent',
      fr: 'Test de joignabilité réussi ; aucune requête payante n’a été envoyée',
      ja: '到達性テストに成功しました。有料リクエストは送信されていません',
      ru: 'Проверка доступности пройдена; платный запрос не отправлялся',
      vi: 'Kiểm tra khả năng kết nối thành công; không gửi yêu cầu có tính phí',
      'zh-TW': '可達性測試通過，未發起付費請求',
      zh: '可达性测试通过，未发起付费请求',
    }

    for (const [locale, expected] of Object.entries(expectedTranslations)) {
      const localePath = fileURLToPath(
        new URL(`../../../../i18n/locales/${locale}.json`, import.meta.url)
      )
      const localeData = JSON.parse(readFileSync(localePath, 'utf8')) as {
        translation: Record<string, string>
      }
      assert.equal(localeData.translation[messageKey], expected, locale)
      for (const obsoleteKey of obsoleteKeys) {
        assert.equal(obsoleteKey in localeData.translation, false, locale)
      }
    }

    const staticKeys = readFileSync(
      fileURLToPath(
        new URL('../../../../i18n/static-keys.ts', import.meta.url)
      ),
      'utf8'
    )
    assert.equal(staticKeys.includes(messageKey), true)
    for (const obsoleteKey of obsoleteKeys) {
      assert.equal(staticKeys.includes(obsoleteKey), false)
    }
  })
})
