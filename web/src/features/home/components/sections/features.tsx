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
import {
  BrainCircuit,
  Clapperboard,
  KeyRound,
  ListVideo,
  ReceiptText,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { AnimateInView } from '@/components/animate-in-view'

export function Features() {
  const { t } = useTranslation()
  const capabilities = [
    {
      id: 'llm',
      icon: BrainCircuit,
      eyebrow: t('Language models'),
      title: t('One API for your LLM stack'),
      description: t(
        'Use DeepSeek, GLM, Qwen, Kimi, MiniMax, MiMo, and other enabled language models through familiar OpenAI-compatible endpoints.'
      ),
      meta: [t('Chat Completions'), t('Streaming'), t('Tools and reasoning')],
      className: 'md:col-span-2',
    },
    {
      id: 'creation',
      icon: Clapperboard,
      eyebrow: t('Image and video creation'),
      title: t('Create beyond text'),
      description: t(
        'Generate and edit images with Grok Imagine, then create videos with Grok Imagine Video and Seedance.'
      ),
      meta: [t('Image generation'), t('Video generation'), t('Media inputs')],
      className: 'md:col-span-1',
    },
    {
      id: 'key',
      icon: KeyRound,
      eyebrow: t('Unified access'),
      title: t('One API Key, every enabled model'),
      description: t(
        'Create, restrict, rotate, and monitor a single Molii API Key without managing a separate credential for every model family.'
      ),
      meta: [
        t('Model restrictions'),
        t('Quota controls'),
        t('IP restrictions'),
      ],
      className: 'md:col-span-1',
    },
    {
      id: 'tasks',
      icon: ListVideo,
      eyebrow: t('Asynchronous workflows'),
      title: t('Tasks that remain observable'),
      description: t(
        'Track asynchronous image and video tasks from submission to completion, including progress, preview, download, and generation records.'
      ),
      meta: [t('Polling'), t('Generation records'), t('Result download')],
      className: 'md:col-span-1',
    },
    {
      id: 'billing',
      icon: ReceiptText,
      eyebrow: t('Transparent billing'),
      title: t('Understand every charge'),
      description: t(
        'Inspect request parameters, pricing dimensions, calculation formulas, estimated cost, and final settled cost in detailed usage records.'
      ),
      meta: [t('Pricing formulas'), t('Usage logs'), t('Final settlement')],
      className: 'md:col-span-1',
    },
  ]

  return (
    <section className='border-border/50 border-t px-6 py-24 md:py-32'>
      <div className='mx-auto max-w-7xl'>
        <AnimateInView className='mb-12 max-w-3xl'>
          <p className='text-muted-foreground mb-3 text-xs font-semibold tracking-[0.18em] uppercase'>
            {t('Molii capabilities')}
          </p>
          <h2 className='text-3xl leading-tight font-semibold tracking-tight md:text-5xl'>
            {t('From language models to creative generation, in one place.')}
          </h2>
        </AnimateInView>

        <div className='grid gap-4 md:grid-cols-3'>
          {capabilities.map((capability, index) => {
            const Icon = capability.icon
            return (
              <AnimateInView
                key={capability.id}
                delay={index * 70}
                animation='scale-in'
                className={capability.className}
              >
                <article
                  data-home-capability={capability.id}
                  className='group border-border/70 bg-background hover:border-foreground/25 relative flex h-full min-h-80 flex-col overflow-hidden rounded-3xl border p-7 transition-[border-color,transform,box-shadow] duration-300 hover:-translate-y-1 hover:shadow-[0_20px_60px_rgb(0_0_0/0.07)] md:p-9'
                >
                  <div className='home-dot-grid absolute inset-0 [mask-image:linear-gradient(to_bottom_right,black,transparent_70%)] opacity-20' />
                  <div className='relative flex items-start justify-between'>
                    <span className='border-border/60 bg-muted/35 flex size-12 items-center justify-center rounded-2xl border'>
                      <Icon className='size-5 transition-transform duration-300 group-hover:scale-110 group-hover:rotate-3' />
                    </span>
                    <span className='text-muted-foreground text-xs font-medium tabular-nums'>
                      {String(index + 1).padStart(2, '0')}
                    </span>
                  </div>
                  <div className='relative mt-auto pt-16'>
                    <p className='text-muted-foreground text-xs font-semibold tracking-[0.14em] uppercase'>
                      {capability.eyebrow}
                    </p>
                    <h3 className='mt-3 text-2xl font-semibold tracking-tight'>
                      {capability.title}
                    </h3>
                    <p className='text-muted-foreground mt-4 max-w-2xl text-sm leading-6'>
                      {capability.description}
                    </p>
                    <div className='mt-6 flex flex-wrap gap-2'>
                      {capability.meta.map((item) => (
                        <span
                          key={item}
                          className='border-border/60 bg-muted/25 rounded-full border px-2.5 py-1 text-[11px]'
                        >
                          {item}
                        </span>
                      ))}
                    </div>
                  </div>
                </article>
              </AnimateInView>
            )
          })}
        </div>
      </div>
    </section>
  )
}
