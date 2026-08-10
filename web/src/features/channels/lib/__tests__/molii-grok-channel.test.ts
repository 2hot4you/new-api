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
import { channelFormSchema } from '../channel-form'
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
    }
    assert.equal(channelFormSchema.safeParse(valid).success, true)
    const invalid = channelFormSchema.safeParse({ ...valid, key: '   ' })
    assert.equal(invalid.success, false)
    assert.equal(
      invalid.error?.issues.some((issue) => issue.path[0] === 'key'),
      true
    )
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
