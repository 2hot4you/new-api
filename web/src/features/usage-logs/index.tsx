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
import { getRouteApi, useNavigate } from '@tanstack/react-router'
import { useCallback, useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import type { NavGroup } from '@/components/layout/types'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { CacheStatsDialog } from '@/features/system-settings/general/channel-affinity/cache-stats-dialog'
import { useSidebarConfig } from '@/hooks/use-sidebar-config'

import { UserInfoDialog } from './components/dialogs/user-info-dialog'
import {
  type LogsViewScope,
  UsageLogsProvider,
  useLogsViewScope,
  useUsageLogsContext,
} from './components/usage-logs-provider'
import { UsageLogsTable } from './components/usage-logs-table'
import {
  isUsageLogsSectionId,
  USAGE_LOGS_DEFAULT_SECTION,
  type UsageLogsSectionId,
} from './section-registry'
import {
  GENERATION_LOG_META,
  GENERATION_LOG_SOURCES,
  resolveUsageLogSource,
  type GenerationLogSection,
} from './source-registry'

const route = getRouteApi('/_authenticated/usage-logs/$section')
const GENERATION_LOG_SECTIONS = ['drawing', 'task'] as const

const SECTION_META: Record<UsageLogsSectionId, { titleKey: string }> = {
  common: {
    titleKey: 'Common Logs',
  },
  drawing: {
    titleKey: 'Image Generation',
  },
  task: {
    titleKey: 'Video Generation',
  },
}

function UsageLogsContent() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const params = route.useParams()
  const search = route.useSearch()
  const activeCategory: UsageLogsSectionId =
    params.section && isUsageLogsSectionId(params.section)
      ? params.section
      : USAGE_LOGS_DEFAULT_SECTION
  const {
    selectedUserId,
    userInfoDialogOpen,
    setUserInfoDialogOpen,
    affinityTarget,
    affinityDialogOpen,
    setAffinityDialogOpen,
  } = useUsageLogsContext()
  const { canManageScope, viewScope, setViewScope } = useLogsViewScope()
  const tabNavGroups = useMemo<NavGroup[]>(
    () => [
      {
        title: GENERATION_LOG_META.titleKey,
        items: GENERATION_LOG_SECTIONS.map((section) => ({
          title: SECTION_META[section].titleKey,
          url: `/usage-logs/${section}`,
        })),
      },
    ],
    []
  )
  const filteredTabGroups = useSidebarConfig(tabNavGroups)
  const visibleSections = useMemo(
    () =>
      (filteredTabGroups[0]?.items ?? [])
        .map((item) => {
          if (!('url' in item) || typeof item.url !== 'string') return null
          return item.url.split('/').pop() ?? null
        })
        .filter((section): section is UsageLogsSectionId =>
          Boolean(section && isUsageLogsSectionId(section))
        ),
    [filteredTabGroups]
  )

  const handleSectionChange = useCallback(
    (section: string) => {
      if (section !== 'drawing' && section !== 'task') return
      const nextSection = section as GenerationLogSection
      void navigate({
        to: '/usage-logs/$section',
        params: { section: nextSection },
        search: {
          ...search,
          source: resolveUsageLogSource(nextSection),
          page: 1,
          filter: undefined,
          ...(nextSection === 'task'
            ? {
                type: undefined,
                model: undefined,
                token: undefined,
                group: undefined,
                requestId: undefined,
                upstreamRequestId: undefined,
              }
            : {}),
        },
      })
    },
    [navigate, search]
  )

  const handleViewScopeChange = useCallback(
    (scope: string) => {
      if (scope === 'all' || scope === 'self') {
        setViewScope(scope as LogsViewScope)
      }
    },
    [setViewScope]
  )

  const activeGenerationSection: GenerationLogSection | null =
    activeCategory === 'drawing' || activeCategory === 'task'
      ? activeCategory
      : null
  const activeSource = activeGenerationSection
    ? resolveUsageLogSource(activeGenerationSection, search.source)
    : undefined
  const handleSourceChange = useCallback(
    (source: string) => {
      if (!activeGenerationSection) return
      const nextSource = resolveUsageLogSource(activeGenerationSection, source)
      if (nextSource !== source) return
      void navigate({
        to: '/usage-logs/$section',
        params: { section: activeGenerationSection },
        search: {
          ...search,
          source: nextSource,
          page: 1,
        },
        replace: true,
      })
    },
    [activeGenerationSection, navigate, search]
  )

  const pageMeta =
    activeCategory === 'common'
      ? SECTION_META.common
      : { titleKey: GENERATION_LOG_META.titleKey }
  const showTaskSwitcher =
    activeCategory !== 'common' && visibleSections.length > 1
  const tableCategory = activeCategory === 'drawing' ? 'image' : activeCategory

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>
          {t(pageMeta.titleKey)}
        </SectionPageLayout.Title>
        {canManageScope && (
          <SectionPageLayout.Actions>
            <Tabs value={viewScope} onValueChange={handleViewScopeChange}>
              <TabsList>
                <TabsTrigger value='all'>{t('All')}</TabsTrigger>
                <TabsTrigger value='self'>{t('Only Mine')}</TabsTrigger>
              </TabsList>
            </Tabs>
          </SectionPageLayout.Actions>
        )}
        <SectionPageLayout.Content>
          <div className='flex h-full min-h-0 flex-col gap-4'>
            {showTaskSwitcher && (
              <Tabs value={activeCategory} onValueChange={handleSectionChange}>
                <TabsList className='max-w-full flex-wrap justify-start group-data-horizontal/tabs:h-auto'>
                  {visibleSections.map((section) => (
                    <TabsTrigger key={section} value={section}>
                      {t(SECTION_META[section].titleKey)}
                    </TabsTrigger>
                  ))}
                </TabsList>
              </Tabs>
            )}
            {activeGenerationSection && activeSource && (
              <Tabs value={activeSource} onValueChange={handleSourceChange}>
                <TabsList className='max-w-full flex-wrap justify-start group-data-horizontal/tabs:h-auto'>
                  {GENERATION_LOG_SOURCES[activeGenerationSection].map(
                    (source) => (
                      <TabsTrigger key={source.id} value={source.id}>
                        {t(source.labelKey)}
                      </TabsTrigger>
                    )
                  )}
                </TabsList>
              </Tabs>
            )}
            <div className='min-h-0 flex-1'>
              <UsageLogsTable
                logCategory={tableCategory}
                source={activeSource}
              />
            </div>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <UserInfoDialog
        userId={selectedUserId}
        open={userInfoDialogOpen}
        onOpenChange={setUserInfoDialogOpen}
      />

      <CacheStatsDialog
        open={affinityDialogOpen}
        onOpenChange={setAffinityDialogOpen}
        target={
          affinityTarget
            ? {
                rule_name: affinityTarget.rule_name || '',
                using_group:
                  affinityTarget.using_group ||
                  affinityTarget.selected_group ||
                  '',
                key_hint: affinityTarget.key_hint || '',
                key_fp: affinityTarget.key_fp || '',
              }
            : null
        }
      />
    </>
  )
}

export function UsageLogs() {
  return (
    <UsageLogsProvider>
      <UsageLogsContent />
    </UsageLogsProvider>
  )
}
