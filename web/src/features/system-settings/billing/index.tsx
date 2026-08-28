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
import { SettingsPage } from '../components/settings-page'
import type { BillingSettings } from '../types'
import {
  BILLING_DEFAULT_SECTION,
  getBillingSectionContent,
  getBillingSectionMeta,
} from './section-registry.tsx'

const defaultBillingSettings: BillingSettings = {
  QuotaForNewUser: 0,
  PreConsumedQuota: 0,
  QuotaForInviter: 0,
  QuotaForInvitee: 0,
  TopUpLink: '',
  'general_setting.docs_link': '',
  'quota_setting.enable_free_model_pre_consume': true,
  QuotaPerUnit: 500000,
  USDExchangeRate: 7,
  'general_setting.quota_display_type': 'USD',
  'general_setting.custom_currency_symbol': '¤',
  'general_setting.custom_currency_exchange_rate': 1,
  DisplayInCurrencyEnabled: true,
  DisplayTokenStatEnabled: true,
  ModelPrice: '',
  ModelRatio: '',
  CacheRatio: '',
  CreateCacheRatio: '',
  CompletionRatio: '',
  ImageRatio: '',
  AudioRatio: '',
  AudioCompletionRatio: '',
  ExposeRatioEnabled: false,
  'billing_setting.billing_mode': '{}',
  'billing_setting.billing_expr': '{}',
  'tool_price_setting.prices': '{}',
  'starai_video_price.standard_720p': 46,
  'starai_video_price.standard_720p_video': 28,
  'starai_video_price.standard_1080p': 51,
  'starai_video_price.standard_1080p_video': 31,
  'starai_video_price.standard_4k': 26,
  'starai_video_price.standard_4k_video': 16,
  'starai_video_price.fast_720p': 37,
  'starai_video_price.fast_720p_video': 22,
  'starai_video_price.mini_720p': 23,
  'starai_video_price.mini_720p_video': 14,
  'starai_video_price.seedance_25_720p': 70,
  'starai_video_price.seedance_25_720p_video': 42,
  'starai_video_price.seedance_25_1080p': 77,
  'starai_video_price.seedance_25_1080p_video': 46,
  'molii_grok_price.image_standard_input': 0.002,
  'molii_grok_price.image_standard_1k': 0.02,
  'molii_grok_price.image_standard_2k': 0.02,
  'molii_grok_price.image_quality_input': 0.01,
  'molii_grok_price.image_quality_1k': 0.05,
  'molii_grok_price.image_quality_2k': 0.07,
  'molii_grok_price.image_20_input': 0.01,
  'molii_grok_price.image_20_low_1k': 0.04,
  'molii_grok_price.image_20_low_2k': 0.06,
  'molii_grok_price.image_20_medium_1k': 0.06,
  'molii_grok_price.image_20_medium_2k': 0.08,
  'molii_grok_price.video_15_image_input': 0.01,
  'molii_grok_price.video_15_480p': 0.08,
  'molii_grok_price.video_15_720p': 0.14,
  'molii_grok_price.video_15_1080p': 0.25,
  'molii_grok_price.video_image_input': 0.002,
  'molii_grok_price.video_video_input': 0.01,
  'molii_grok_price.video_480p': 0.05,
  'molii_grok_price.video_720p': 0.07,
  'molii_grok_tool_price.web_search': 5,
  'molii_grok_tool_price.x_search': 5,
  'molii_grok_tool_price.code_execution': 5,
  'molii_grok_tool_price.attachment_search': 10,
  'molii_grok_tool_price.collections_search': 2.5,
  'molii_grok_tool_price.image_generation': 0.05,
  TopupGroupRatio: '',
  GroupRatio: '',
  UserUsableGroups: '',
  GroupGroupRatio: '',
  AutoGroups: '',
  MaxTokenAutoGroups: 5,
  DefaultUseAutoGroup: false,
  'group_ratio_setting.group_metadata': '[]',
  'group_ratio_setting.group_special_usable_group': '{}',
  PayAddress: '',
  EpayId: '',
  EpayKey: '',
  Price: 7.3,
  MinTopUp: 1,
  CustomCallbackAddress: '',
  PayMethods: '',
  'payment_setting.amount_options': '',
  'payment_setting.amount_discount': '',
  'payment_setting.compliance_confirmed': false,
  'payment_setting.compliance_terms_version': '',
  'payment_setting.compliance_confirmed_at': 0,
  'payment_setting.compliance_confirmed_by': 0,
  'payment_setting.compliance_confirmed_ip': '',
  StripeApiSecret: '',
  StripeWebhookSecret: '',
  StripePriceId: '',
  StripeUnitPrice: 8.0,
  StripeMinTopUp: 1,
  StripePromotionCodesEnabled: false,
  CreemApiKey: '',
  CreemWebhookSecret: '',
  CreemTestMode: false,
  CreemProducts: '[]',
  WaffoEnabled: false,
  WaffoApiKey: '',
  WaffoPrivateKey: '',
  WaffoPublicCert: '',
  WaffoSandboxPublicCert: '',
  WaffoSandboxApiKey: '',
  WaffoSandboxPrivateKey: '',
  WaffoSandbox: false,
  WaffoMerchantId: '',
  WaffoCurrency: 'USD',
  WaffoUnitPrice: 1,
  WaffoMinTopUp: 1,
  WaffoNotifyUrl: '',
  WaffoReturnUrl: '',
  WaffoPayMethods: '[]',
  WaffoPancakeMerchantID: '',
  WaffoPancakePrivateKey: '',
  WaffoPancakeReturnURL: '',
  WaffoPancakeStoreID: '',
  WaffoPancakeProductID: '',
  'checkin_setting.enabled': false,
  'checkin_setting.min_quota': 1000,
  'checkin_setting.max_quota': 10000,
}

export function BillingSettings() {
  return (
    <SettingsPage
      routePath='/_authenticated/system-settings/billing/$section'
      defaultSettings={defaultBillingSettings}
      defaultSection={BILLING_DEFAULT_SECTION}
      getSectionContent={getBillingSectionContent}
      getSectionMeta={getBillingSectionMeta}
    />
  )
}
