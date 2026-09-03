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
import { Activity, KeyRound, Route } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { PublicLayout } from '@/components/layout'
import { Footer } from '@/components/layout/components/footer'

type AuthLayoutProps = {
  children: React.ReactNode
}

export function AuthLayout({ children }: AuthLayoutProps) {
  const { t } = useTranslation()

  return (
    <PublicLayout showMainContainer={false}>
      <main className='relative isolate flex min-h-svh items-center overflow-hidden px-4 pt-20 pb-10 sm:px-6 sm:pt-24 sm:pb-14 lg:px-8'>
        <div
          aria-hidden='true'
          className='pointer-events-none absolute inset-0 -z-10 bg-[radial-gradient(circle_at_8%_12%,color-mix(in_oklab,var(--primary)_10%,transparent),transparent_34%),linear-gradient(to_bottom,var(--background),color-mix(in_oklab,var(--muted)_36%,var(--background)))]'
        />

        <section className='border-border/70 bg-background/95 mx-auto grid w-full max-w-5xl overflow-hidden rounded-2xl border shadow-[0_24px_70px_-36px_rgba(15,23,42,0.35)] backdrop-blur-sm md:grid-cols-[0.86fr_1.14fr]'>
          <aside className='relative isolate overflow-hidden border-b px-6 py-7 md:flex md:min-h-[620px] md:flex-col md:justify-between md:border-r md:border-b-0 md:px-10 md:py-11'>
            <div
              aria-hidden='true'
              className='pointer-events-none absolute inset-0 -z-10 bg-[radial-gradient(circle_at_12%_8%,color-mix(in_oklab,var(--primary)_24%,transparent),transparent_33%),radial-gradient(circle_at_94%_90%,color-mix(in_oklab,var(--chart-2)_24%,transparent),transparent_38%),linear-gradient(145deg,color-mix(in_oklab,var(--muted)_52%,var(--background)),var(--background))]'
            />

            <div className='max-w-md'>
              <p className='text-primary text-xs font-semibold tracking-[0.16em] uppercase'>
                Molii Developer Platform
              </p>
              <h1 className='mt-3 text-2xl leading-tight font-semibold tracking-tight text-balance sm:text-3xl md:mt-5 md:text-4xl'>
                {t('One key. Every model you can rely on.')}
              </h1>
              <p className='text-muted-foreground mt-3 max-w-sm text-sm leading-6 md:mt-4'>
                {t(
                  'Connect leading AI models through one reliable gateway, with routing and usage visibility built in.'
                )}
              </p>
            </div>

            <div className='mt-6 hidden space-y-3 md:block'>
              <StoryPoint
                icon={Route}
                title={t('Unified routing')}
                description={t(
                  'Switch providers without changing your client.'
                )}
              />
              <StoryPoint
                icon={KeyRound}
                title={t('Controlled access')}
                description={t(
                  'Keep model permissions and quotas in one place.'
                )}
              />
              <StoryPoint
                icon={Activity}
                title={t('Observable usage')}
                description={t(
                  'See requests, costs, and service health clearly.'
                )}
              />
            </div>
          </aside>

          <div className='flex min-w-0 items-center px-5 py-8 sm:px-9 sm:py-10 md:px-12 md:py-12'>
            <div className='mx-auto w-full max-w-[28rem]'>{children}</div>
          </div>
        </section>
      </main>
      <Footer />
    </PublicLayout>
  )
}

type StoryPointProps = {
  icon: React.ComponentType<{ className?: string }>
  title: string
  description: string
}

function StoryPoint({ icon: Icon, title, description }: StoryPointProps) {
  return (
    <div className='border-border/60 bg-background/55 flex items-start gap-3 rounded-xl border px-4 py-3 backdrop-blur-sm'>
      <span className='bg-background text-foreground grid size-8 shrink-0 place-items-center rounded-lg shadow-sm'>
        <Icon className='size-4' />
      </span>
      <div className='min-w-0'>
        <p className='text-sm font-medium'>{title}</p>
        <p className='text-muted-foreground mt-0.5 text-xs leading-5'>
          {description}
        </p>
      </div>
    </div>
  )
}
