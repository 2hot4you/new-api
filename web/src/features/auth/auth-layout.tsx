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
import { AudioLines, Image, Sparkles, Video } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Skeleton } from '@/components/ui/skeleton'
import { useSystemConfig } from '@/hooks/use-system-config'

type AuthLayoutProps = {
  children: React.ReactNode
}

interface AuthLayoutFrameProps extends AuthLayoutProps {
  systemName: string
  logo: string
  loading: boolean
  labels: {
    logo: string
    workspace: string
    tagline: string
    image: string
    video: string
    audio: string
  }
}

export function AuthLayoutFrame({
  children,
  systemName,
  logo,
  loading,
  labels,
}: AuthLayoutFrameProps) {
  const creativeTools = [
    { label: labels.image, icon: Image },
    { label: labels.video, icon: Video },
    { label: labels.audio, icon: AudioLines },
  ]

  return (
    <div
      data-auth-layout='molii'
      className='relative min-h-svh overflow-hidden bg-[#050506] text-[#F5F6FA]'
      style={{
        fontFamily: 'Inter, "PingFang SC", "Microsoft YaHei", sans-serif',
      }}
    >
      <header className='absolute inset-x-0 top-0 z-20 flex h-20 items-center px-5 sm:px-8 lg:h-24 lg:px-10'>
        <a
          href='/'
          aria-label={systemName}
          className='flex items-center gap-3 rounded-xl transition-opacity outline-none hover:opacity-80 focus-visible:ring-2 focus-visible:ring-[#7566FF] focus-visible:ring-offset-4 focus-visible:ring-offset-[#050506]'
        >
          <span className='relative block size-9 shrink-0 overflow-hidden rounded-xl border border-[#24252B] bg-[#131417]'>
            {loading ? (
              <Skeleton className='absolute inset-0 rounded-xl bg-[#24252B]' />
            ) : (
              <img
                src={logo}
                alt={labels.logo}
                className='size-full object-cover'
              />
            )}
          </span>
          {loading ? (
            <Skeleton className='h-6 w-24 bg-[#24252B]' />
          ) : (
            <span className='text-base font-semibold tracking-[-0.01em] text-white sm:text-lg'>
              {systemName}
            </span>
          )}
        </a>
      </header>

      <main className='grid min-h-svh lg:grid-cols-[minmax(0,1.08fr)_minmax(480px,0.92fr)]'>
        <aside
          aria-label={labels.workspace}
          className='relative hidden min-h-svh overflow-hidden border-r border-[#24252B] bg-[#0E0F12] lg:flex lg:flex-col lg:justify-center lg:px-10 lg:pt-28 lg:pb-12 xl:px-16'
        >
          <div
            aria-hidden='true'
            className='absolute top-0 right-0 h-full w-px bg-[#7566FF]/25'
          />
          <div className='relative z-10 mx-auto w-full max-w-2xl'>
            <div className='mb-8 inline-flex items-center gap-2 rounded-full border border-[#292B33] bg-[#131417] px-3 py-1.5 text-xs font-medium text-[#B8BAC9]'>
              <Sparkles className='size-3.5 text-[#7566FF]' />
              {labels.workspace}
            </div>

            <h2 className='max-w-xl text-4xl leading-tight font-bold tracking-[-0.035em] text-white xl:text-5xl'>
              {labels.tagline}
            </h2>

            <div className='mt-10 grid grid-cols-3 gap-3'>
              {creativeTools.map(({ label, icon: Icon }, index) => (
                <div
                  key={label}
                  className='rounded-xl border border-[#24252B] bg-[#131417] p-3 xl:p-4'
                >
                  <span
                    className={`mb-6 flex size-9 items-center justify-center rounded-[10px] ${
                      index === 1
                        ? 'bg-[#7566FF] text-white'
                        : 'border border-[#292B33] bg-[#17181D] text-[#9499A8]'
                    }`}
                  >
                    <Icon className='size-4' />
                  </span>
                  <span className='text-sm font-semibold text-[#F5F6FA]'>
                    {label}
                  </span>
                </div>
              ))}
            </div>

            <div
              aria-hidden='true'
              className='relative mt-4 h-48 overflow-hidden rounded-2xl border border-[#24252B] bg-[#050506] p-4 xl:h-56'
            >
              <div className='flex items-center gap-1.5 border-b border-[#24252B] pb-3'>
                <span className='size-1.5 rounded-full bg-[#7566FF]' />
                <span className='size-1.5 rounded-full bg-[#3D3F49]' />
                <span className='size-1.5 rounded-full bg-[#3D3F49]' />
              </div>
              <div className='grid h-[calc(100%-28px)] grid-cols-[0.72fr_1.28fr] gap-3 pt-3'>
                <div className='space-y-2 rounded-xl border border-[#24252B] bg-[#0E0F12] p-3'>
                  <div className='h-2 w-12 rounded-full bg-[#7566FF]' />
                  <div className='h-2 w-full rounded-full bg-[#24252B]' />
                  <div className='h-2 w-4/5 rounded-full bg-[#24252B]' />
                  <div className='mt-5 h-8 rounded-[10px] bg-[#131417]' />
                </div>
                <div className='relative overflow-hidden rounded-xl border border-[#292B33] bg-[#131417]'>
                  <div className='absolute inset-x-4 top-4 h-2 rounded-full bg-[#24252B]' />
                  <div className='absolute inset-x-4 top-10 bottom-4 grid grid-cols-2 gap-2'>
                    <div className='rounded-lg bg-[#17181D]' />
                    <div className='rounded-lg bg-[#7566FF]/20 ring-1 ring-[#7566FF]/50' />
                  </div>
                </div>
              </div>
            </div>
          </div>
        </aside>

        <section className='relative flex min-h-svh items-center justify-center bg-[#050506] px-4 pt-24 pb-8 sm:px-8 sm:pt-28 sm:pb-12 lg:px-10'>
          <div
            aria-hidden='true'
            className='absolute top-28 right-8 size-24 rounded-full border border-[#24252B] opacity-60 lg:hidden'
          />
          <div className='relative z-10 w-full max-w-[480px] rounded-2xl border border-[#24252B] bg-[#0E0F12] p-5 shadow-none sm:p-8 [&_a]:outline-none [&_a]:focus-visible:rounded-sm [&_a]:focus-visible:ring-2 [&_a]:focus-visible:ring-[#7566FF] [&_button]:min-h-11 [&_button]:rounded-[10px] [&_button]:focus-visible:border-[#7566FF] [&_button]:focus-visible:ring-[#7566FF]/25 [&_button[type=submit]]:bg-[#7566FF] [&_button[type=submit]]:text-white [&_button[type=submit]]:hover:bg-[#7566FF]/90 [&_input]:h-11 [&_input]:rounded-[10px] [&_input]:border-[#24252B] [&_input]:bg-[#131417] [&_input]:text-[#F5F6FA] [&_input]:placeholder:text-[#9499A8] [&_input]:focus-visible:border-[#7566FF] [&_input]:focus-visible:ring-[#7566FF]/25 [&_label]:text-[#B8BAC9]'>
            {children}
          </div>
        </section>
      </main>
    </div>
  )
}

export function AuthLayout({ children }: AuthLayoutProps) {
  const { t } = useTranslation()
  const { systemName, logo, loading } = useSystemConfig()

  return (
    <AuthLayoutFrame
      systemName={systemName}
      logo={logo}
      loading={loading}
      labels={{
        logo: t('Logo'),
        workspace: t('AI model testing environment'),
        tagline: t(
          'Power AI applications, manage digital assets, connect the Future'
        ),
        image: t('Image'),
        video: t('Video'),
        audio: t('Audio'),
      }}
    >
      {children}
    </AuthLayoutFrame>
  )
}
