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
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { SettingsPageTitleStatusPortal } from '../components/settings-page-context'

type AigcPricingTab =
  | 'model-pricing'
  | 'seedance'
  | 'grok-imagine'
  | 'catalog-sync'

type MoliiAigcPricingTabsProps = {
  modelPricing: ReactNode
  seedance: ReactNode
  grokImagine: ReactNode
  catalogSync: ReactNode
}

export function MoliiAigcPricingTabs(props: MoliiAigcPricingTabsProps) {
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState<AigcPricingTab>('model-pricing')
  const isRoot = useAuthStore(
    (state) => state.auth.user?.role === ROLE.SUPER_ADMIN
  )

  return (
    <Tabs
      value={activeTab}
      onValueChange={(value) => {
        if (value !== null) setActiveTab(value as AigcPricingTab)
      }}
      className='h-full min-h-0 gap-6'
    >
      <SettingsPageTitleStatusPortal>
        <TabsList className='flex w-fit max-w-full flex-wrap'>
          <TabsTrigger value='model-pricing'>{t('General models')}</TabsTrigger>
          <TabsTrigger value='seedance'>{t('Seedance 2.0')}</TabsTrigger>
          <TabsTrigger value='grok-imagine'>{t('Grok Imagine')}</TabsTrigger>
          {isRoot && (
            <TabsTrigger value='catalog-sync'>
              {t('Environment sync')}
            </TabsTrigger>
          )}
        </TabsList>
      </SettingsPageTitleStatusPortal>

      {activeTab === 'model-pricing' && (
        <TabsContent value='model-pricing' className='min-h-0'>
          {props.modelPricing}
        </TabsContent>
      )}
      {activeTab === 'seedance' && (
        <TabsContent value='seedance' className='min-h-0'>
          {props.seedance}
        </TabsContent>
      )}
      {activeTab === 'grok-imagine' && (
        <TabsContent value='grok-imagine' className='min-h-0'>
          {props.grokImagine}
        </TabsContent>
      )}
      {isRoot && activeTab === 'catalog-sync' && (
        <TabsContent value='catalog-sync' className='min-h-0'>
          {props.catalogSync}
        </TabsContent>
      )}
    </Tabs>
  )
}
