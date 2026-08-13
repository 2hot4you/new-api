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
import { ArrowUpRight, Braces, KeyRound, Search } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { AnimateInView } from '@/components/animate-in-view'
import { cn } from '@/lib/utils'

interface CTAProps {
  isAuthenticated?: boolean
}

const CURL_SAMPLE = `curl https://aigc.claudeye.com/v1/chat/completions \\
  -H "Authorization: Bearer $MOLII_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "deepseek-v4-flash-202605",
    "messages": [{"role": "user", "content": "Hello, Molii"}],
    "stream": true
  }'`

const PYTHON_SAMPLE = `import os
from openai import OpenAI

client = OpenAI(
    api_key=os.environ["MOLII_API_KEY"],
    base_url="https://aigc.claudeye.com/v1",
)

response = client.chat.completions.create(
    model="deepseek-v4-flash-202605",
    messages=[{"role": "user", "content": "Hello, Molii"}],
)
print(response.choices[0].message.content)`

export function CTA(props: CTAProps) {
  const { t } = useTranslation()
  const [language, setLanguage] = useState<'curl' | 'python'>('curl')

  return (
    <section className='border-border/50 border-t px-6 py-24 md:py-32'>
      <div className='mx-auto max-w-7xl'>
        <AnimateInView className='mb-12 max-w-3xl'>
          <p className='text-muted-foreground mb-3 text-xs font-semibold tracking-[0.18em] uppercase'>
            {t('Get started')}
          </p>
          <h2 className='text-3xl leading-tight font-semibold tracking-tight md:text-5xl'>
            {t('Explore first. Integrate when you are ready.')}
          </h2>
        </AnimateInView>

        <div className='grid gap-5 lg:grid-cols-2'>
          <AnimateInView animation='fade-right'>
            <article
              data-home-api-example
              className='border-border/70 relative flex min-h-[34rem] flex-col overflow-hidden rounded-3xl border bg-[#111] text-[#f4f4f4] shadow-[0_24px_80px_rgb(0_0_0/0.14)]'
            >
              <div className='flex items-center justify-between border-b border-white/10 px-5 py-4'>
                <div className='flex items-center gap-2 text-sm font-medium'>
                  <Braces className='size-4' />
                  {t('Use Molii through API')}
                </div>
                <div className='flex rounded-lg bg-white/8 p-1'>
                  {(['curl', 'python'] as const).map((item) => (
                    <button
                      key={item}
                      type='button'
                      onClick={() => setLanguage(item)}
                      className={cn(
                        'rounded-md px-3 py-1.5 text-xs transition-colors',
                        language === item
                          ? 'bg-white text-black'
                          : 'text-white/55 hover:text-white'
                      )}
                    >
                      {item === 'curl' ? 'cURL' : 'Python'}
                    </button>
                  ))}
                </div>
              </div>
              <pre className='min-h-0 flex-1 overflow-auto p-6 font-mono text-[12px] leading-6 text-white/78 md:text-[13px]'>
                <code>{language === 'curl' ? CURL_SAMPLE : PYTHON_SAMPLE}</code>
              </pre>
              <div className='flex flex-col gap-3 border-t border-white/10 px-5 py-4 sm:flex-row sm:items-center sm:justify-between'>
                <p className='text-xs text-white/45'>
                  {t('This example is display-only and never sends a request.')}
                </p>
                <a
                  href={props.isAuthenticated ? '/keys' : '/sign-up'}
                  className='inline-flex items-center gap-1 text-sm font-medium text-white'
                >
                  <KeyRound className='size-4' />
                  {props.isAuthenticated
                    ? t('Manage API Keys')
                    : t('Create account')}
                  <ArrowUpRight className='size-4' />
                </a>
              </div>
            </article>
          </AnimateInView>

          <AnimateInView animation='fade-left' delay={90}>
            <article className='border-border/70 bg-muted/15 home-dot-grid relative flex min-h-[34rem] flex-col overflow-hidden rounded-3xl border p-8 md:p-10'>
              <div className='relative'>
                <span className='border-border/60 bg-background flex size-12 items-center justify-center rounded-2xl border shadow-sm'>
                  <Search className='size-5' />
                </span>
                <p className='text-muted-foreground mt-10 text-xs font-semibold tracking-[0.16em] uppercase'>
                  {t('Model marketplace')}
                </p>
                <h3 className='mt-4 max-w-md text-3xl font-semibold tracking-tight md:text-4xl'>
                  {t('Find the right model before writing the first request.')}
                </h3>
                <p className='text-muted-foreground mt-5 max-w-lg text-sm leading-7'>
                  {t(
                    'Search by provider and capability, compare pricing, review supported parameters, and open complete API examples for each model.'
                  )}
                </p>
              </div>

              <div className='relative mt-auto pt-14'>
                <div className='border-border/70 bg-background/80 grid grid-cols-2 gap-px overflow-hidden rounded-2xl border shadow-sm backdrop-blur sm:grid-cols-3'>
                  {[
                    t('LLM models'),
                    t('Image generation'),
                    t('Video generation'),
                    t('Price comparison'),
                    t('Model capabilities'),
                    t('API examples'),
                  ].map((item) => (
                    <div
                      key={item}
                      className='bg-background/75 px-4 py-5 text-center text-xs font-medium'
                    >
                      {item}
                    </div>
                  ))}
                </div>
                <a
                  href='/pricing'
                  className='bg-foreground text-background mt-5 inline-flex h-11 items-center gap-2 rounded-xl px-5 text-sm font-medium'
                >
                  {t('Explore model marketplace')}
                  <ArrowUpRight className='size-4' />
                </a>
              </div>
            </article>
          </AnimateInView>
        </div>
      </div>
    </section>
  )
}
