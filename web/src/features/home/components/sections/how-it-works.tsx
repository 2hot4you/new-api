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
import { ArrowRight, Boxes, Code2, Eye, WandSparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { AnimateInView } from '@/components/animate-in-view'

export function HowItWorks() {
  const { t } = useTranslation()
  const principles = [
    {
      id: 'models',
      icon: Boxes,
      title: t('Model-rich by design'),
      description: t(
        'Compare language, image, and video models through a single searchable catalog.'
      ),
    },
    {
      id: 'developer',
      icon: Code2,
      title: t('Developer-friendly access'),
      description: t(
        'Use familiar request formats, one API Key, and clear examples to move from exploration to integration.'
      ),
    },
    {
      id: 'billing',
      icon: Eye,
      title: t('Transparent by default'),
      description: t(
        'See prices before calling and review request-level billing details after completion.'
      ),
    },
    {
      id: 'creation',
      icon: WandSparkles,
      title: t('Creation comes first'),
      description: t(
        'Treat generated images and videos as trackable work, with status, preview, records, and downloads.'
      ),
    },
  ]

  return (
    <section className='border-border/50 border-t px-6 py-24 md:py-32'>
      <div className='mx-auto max-w-7xl'>
        <AnimateInView className='mb-10'>
          <p className='text-muted-foreground mb-3 text-xs font-semibold tracking-[0.18em] uppercase'>
            {t('The Molii way')}
          </p>
          <h2 className='text-3xl font-semibold tracking-tight md:text-5xl'>
            {t('A clearer way to use AI models')}
          </h2>
        </AnimateInView>

        <div className='border-border/60 border-t'>
          {principles.map((principle, index) => {
            const Icon = principle.icon
            return (
              <AnimateInView
                key={principle.id}
                delay={index * 70}
                animation={index % 2 === 0 ? 'fade-right' : 'fade-left'}
              >
                <article
                  data-home-principle={principle.id}
                  className='group border-border/60 grid gap-6 border-b py-8 md:grid-cols-[5rem_1fr_1fr_3rem] md:items-center md:py-10'
                >
                  <span className='text-muted-foreground text-sm tabular-nums'>
                    {String(index + 1).padStart(2, '0')}
                  </span>
                  <div className='flex items-center gap-4'>
                    <span className='border-border/60 bg-muted/30 flex size-11 items-center justify-center rounded-xl border'>
                      <Icon className='size-5 transition-transform duration-300 group-hover:scale-110 group-hover:-rotate-3' />
                    </span>
                    <h3 className='text-2xl font-semibold tracking-tight md:text-3xl'>
                      {principle.title}
                    </h3>
                  </div>
                  <p className='text-muted-foreground max-w-xl text-sm leading-6 md:text-base'>
                    {principle.description}
                  </p>
                  <ArrowRight className='text-muted-foreground hidden size-5 transition-transform duration-300 group-hover:translate-x-1 md:block' />
                </article>
              </AnimateInView>
            )
          })}
        </div>
      </div>
    </section>
  )
}
