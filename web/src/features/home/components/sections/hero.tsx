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
import { ArrowUpRight, BookOpen, Layers3, Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import type { PricingModel } from '@/features/pricing/types'
import { useStatus } from '@/hooks/use-status'

import { ModelSearch } from '../model-search'

interface HeroProps {
  models: PricingModel[]
  isLoading?: boolean
  isAuthenticated?: boolean
}

export function Hero(props: HeroProps) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const docsUrl =
    (status?.docs_link as string | undefined) || 'https://docs.newapi.pro'

  return (
    <section className='relative isolate overflow-hidden px-6 pt-24 pb-20 md:pt-36 md:pb-28'>
      <div
        aria-hidden
        className='pointer-events-none absolute inset-0 -z-20 bg-[radial-gradient(circle_at_50%_-10%,color-mix(in_oklab,var(--foreground)_10%,transparent),transparent_58%)]'
      />
      <div
        aria-hidden
        className='home-dot-grid pointer-events-none absolute inset-0 -z-10 [mask-image:linear-gradient(to_bottom,black,transparent_82%)] opacity-45'
      />

      <div className='mx-auto flex max-w-7xl flex-col items-center text-center'>
        <div
          className='landing-animate-fade-up border-border/70 bg-background/75 mb-7 inline-flex items-center gap-2 rounded-full border px-3.5 py-1.5 text-xs font-medium opacity-0 shadow-sm backdrop-blur'
          style={{ animationDelay: '0ms' }}
        >
          <Sparkles className='size-3.5' />
          {t('One platform for language, image, and video models')}
        </div>

        <h1
          className='landing-animate-fade-up max-w-6xl text-[clamp(2.6rem,6.5vw,6rem)] leading-[0.94] font-semibold tracking-[-0.055em] opacity-0'
          style={{ animationDelay: '70ms' }}
        >
          <span className='block text-balance sm:whitespace-nowrap'>
            {t('Build with every kind of AI.')}
          </span>
          <span className='text-muted-foreground mt-2 block text-balance sm:whitespace-nowrap'>
            {t('Create with Molii.')}
          </span>
        </h1>

        <p
          className='landing-animate-fade-up text-muted-foreground mt-8 max-w-3xl text-base leading-7 opacity-0 md:text-lg md:leading-8'
          style={{ animationDelay: '140ms' }}
        >
          {t(
            'Explore DeepSeek, GLM, Qwen, Kimi, MiniMax, MiMo, Seedance, and Grok Imagine through one model platform, one API key, and transparent billing.'
          )}
        </p>

        <div
          className='landing-animate-fade-up mt-10 w-full max-w-3xl opacity-0'
          style={{ animationDelay: '210ms' }}
        >
          <ModelSearch models={props.models} isLoading={props.isLoading} />
        </div>

        <div
          className='landing-animate-fade-up mt-6 flex flex-wrap items-center justify-center gap-3 opacity-0'
          style={{ animationDelay: '280ms' }}
        >
          <Button
            className='group h-11 rounded-xl px-5'
            render={<a href='/pricing' />}
          >
            <Layers3 className='size-4' />
            {t('Explore model marketplace')}
            <ArrowUpRight className='size-4 transition-transform group-hover:translate-x-0.5 group-hover:-translate-y-0.5' />
          </Button>
          <Button
            variant='outline'
            className='h-11 rounded-xl px-5'
            render={
              <a href={docsUrl} target='_blank' rel='noopener noreferrer' />
            }
          >
            <BookOpen className='size-4' />
            {t('Read API documentation')}
          </Button>
          <a
            href={props.isAuthenticated ? '/dashboard' : '/sign-up'}
            className='text-muted-foreground hover:text-foreground px-3 py-2 text-sm transition-colors'
          >
            {props.isAuthenticated ? t('Open dashboard') : t('Create account')}
          </a>
        </div>

        <div
          className='landing-animate-fade-up border-border/55 text-muted-foreground mt-12 flex flex-wrap justify-center gap-x-6 gap-y-2 border-t pt-5 text-xs opacity-0 md:text-sm'
          style={{ animationDelay: '350ms' }}
        >
          <span>{t('OpenAI-compatible API')}</span>
          <span>{t('Asynchronous generation tasks')}</span>
          <span>{t('Detailed usage and billing records')}</span>
        </div>
      </div>
    </section>
  )
}
