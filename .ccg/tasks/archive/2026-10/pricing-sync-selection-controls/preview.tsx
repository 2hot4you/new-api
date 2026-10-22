// Disposable data and real production components; never calls a write API.
import { useEffect, useState } from 'react'
import { createRoot } from 'react-dom/client'
import i18next from 'i18next'
import { initReactI18next } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { UpstreamRatioSyncTable } from '@/features/system-settings/models/upstream-ratio-sync-table'
import type { PricingSourceSelections } from '@/features/system-settings/models/upstream-ratio-sync-helpers'
import type { PricingSyncModels } from '@/features/system-settings/types'
import zh from '@/i18n/locales/zh.json'
import '@/styles/index.css'

await i18next.use(initReactI18next).init({ lng: 'zh', resources: { zh }, interpolation: { escapeValue: false } })

const prices: PricingSyncModels = Object.fromEntries([
  ['gpt-image-2', 5, 30],
  ['claude-sonnet-5', 3, 15],
  ['gemini-3.7-flash', 0.75, 3.75],
].map(([model, input, output]) => [model, {
  current: { billing_mode: 'ratio', model_ratio: 1, completion_ratio: 2 },
  upstreams: { 'dev.molii.co': { billing_mode: 'tiered_expr', billing_expr: `p * ${input} + c * ${output}` } },
}]))

function Preview() {
  const [selected, setSelected] = useState<PricingSourceSelections>({})
  const [dark, setDark] = useState(false)
  useEffect(() => { document.documentElement.classList.toggle('dark', dark) }, [dark])
  return <main className='bg-background text-foreground flex min-h-screen flex-col gap-6 p-8'>
    <div className='flex items-center justify-between gap-4'>
      <h1 className='text-xl font-semibold'>定价同步选择预览（仅本地，无保存请求）</h1>
      <Button variant='outline' onClick={() => setDark(!dark)}>{dark ? '切换浅色' : '切换深色'}</Button>
    </div>
    <div className='h-[620px] min-h-0'>
      <UpstreamRatioSyncTable prices={prices} differences={{}} selectedSources={selected} isDisabled={false} isSyncing={false}
        onSelectPrices={(selections) => setSelected((previous) => ({ ...previous, ...Object.fromEntries(selections.map((item) => [item.model, item.source])) }))}
        onUnselectPrices={(models) => setSelected((previous) => Object.fromEntries(Object.entries(previous).filter(([name]) => !models.includes(name))))} />
    </div>
    <p role='status'>已选择 {Object.keys(selected).length} 个模型；此页面不会修改任何站点价格。</p>
  </main>
}

createRoot(document.getElementById('root')!).render(<Preview />)
