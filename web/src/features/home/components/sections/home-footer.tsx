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
import { ArrowUpRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Footer } from '@/components/layout/components/footer'
import { useStatus } from '@/hooks/use-status'
import { useSystemConfig } from '@/hooks/use-system-config'

import { buildHomeDocsUrl, getHomeFooterVariant } from '../../lib/home-footer'

interface HomeFooterContentProps {
  displayName: string
  displayLogo: string
  docsLink?: string
  userAgreementEnabled: boolean
  privacyPolicyEnabled: boolean
}

interface HomeFooterLink {
  href: string
  label: string
  external?: boolean
}

function FooterLink(props: HomeFooterLink) {
  return (
    <a
      href={props.href}
      target={props.external ? '_blank' : undefined}
      rel={props.external ? 'noopener noreferrer' : undefined}
      className='group/link flex min-h-8 items-center gap-1 text-sm text-white/58 transition-[color,transform] hover:text-white motion-safe:hover:translate-x-0.5'
    >
      {props.label}
      {props.external && (
        <ArrowUpRight className='size-3.5 opacity-0 transition-opacity group-hover/link:opacity-100' />
      )}
    </a>
  )
}

function FooterColumn(props: { title: string; links: HomeFooterLink[] }) {
  return (
    <div className='min-w-0'>
      <h3 className='text-xs font-semibold tracking-[0.16em] text-white/38 uppercase'>
        {props.title}
      </h3>
      <nav
        aria-label={props.title}
        className='mt-4 flex flex-col items-start gap-1'
      >
        {props.links.map((link) => (
          <FooterLink key={`${link.href}:${link.label}`} {...link} />
        ))}
      </nav>
    </div>
  )
}

export function HomeFooterContent(props: HomeFooterContentProps) {
  const { t } = useTranslation()
  const currentYear = new Date().getFullYear()
  const quickStartLink = buildHomeDocsUrl(
    props.docsLink,
    '/getting-started/quickstart'
  )
  const apiReferenceLink = buildHomeDocsUrl(props.docsLink, '/api-reference')

  const productLinks: HomeFooterLink[] = [
    { href: '/pricing', label: t('Model marketplace') },
    { href: '/keys', label: t('API Keys') },
    { href: '/usage-logs/task', label: t('Generation Records') },
    { href: '/usage-logs/common', label: t('Usage Logs') },
  ]
  const developerLinks: HomeFooterLink[] = [
    ...(quickStartLink
      ? [
          {
            href: quickStartLink,
            label: t('footer.home.quickStart'),
            external: true,
          },
        ]
      : []),
    ...(apiReferenceLink
      ? [
          {
            href: apiReferenceLink,
            label: t('footer.home.apiDocumentation'),
            external: true,
          },
        ]
      : []),
    { href: '/pricing', label: t('footer.home.modelsAndPricing') },
  ]
  const supportLinks: HomeFooterLink[] = [
    { href: '/about', label: t('About') },
    ...(props.userAgreementEnabled
      ? [{ href: '/user-agreement', label: t('User Agreement') }]
      : []),
    ...(props.privacyPolicyEnabled
      ? [{ href: '/privacy-policy', label: t('Privacy Policy') }]
      : []),
  ]

  return (
    <footer
      data-home-footer
      className='relative z-10 overflow-hidden bg-[#171717] px-6 text-white'
    >
      <div
        aria-hidden
        className='pointer-events-none absolute inset-x-0 top-0 h-px bg-white/12'
      />
      <div className='mx-auto max-w-7xl py-16 md:py-20'>
        <div className='grid min-w-0 gap-10 sm:grid-cols-2 lg:grid-cols-[1.4fr_repeat(3,1fr)] lg:gap-14'>
          <div className='min-w-0 sm:col-span-2 lg:col-span-1'>
            <a href='/' className='inline-flex items-center gap-3'>
              <img
                src={props.displayLogo}
                alt={props.displayName}
                className='size-9 rounded-xl bg-white/8 object-contain p-1'
              />
              <span className='text-lg font-semibold tracking-tight'>
                {props.displayName}
              </span>
            </a>
            <p className='mt-5 max-w-md text-sm leading-7 text-white/52'>
              {t('footer.home.brandDescription')}
            </p>
            <a
              href='/pricing'
              className='mt-7 inline-flex min-h-10 items-center gap-2 rounded-xl border border-white/14 bg-white/7 px-4 text-sm font-medium transition-colors hover:bg-white/12'
            >
              {t('Explore model marketplace')}
              <ArrowUpRight className='size-4' />
            </a>
          </div>

          <FooterColumn
            title={t('footer.home.products')}
            links={productLinks}
          />
          <FooterColumn
            title={t('footer.home.developers')}
            links={developerLinks}
          />
          <FooterColumn title={t('footer.home.support')} links={supportLinks} />
        </div>

        <div className='mt-14 flex min-w-0 flex-col gap-3 border-t border-white/10 pt-6 text-xs text-white/34 sm:flex-row sm:items-center sm:justify-between'>
          <span>
            &copy; {currentYear} {props.displayName}.{' '}
            {t('footer.defaultCopyright')}
          </span>
          <span>
            {t('footer.home.builtOn')}{' '}
            <a
              href='https://github.com/QuantumNous/new-api'
              target='_blank'
              rel='noopener noreferrer'
              className='font-medium text-white/60 transition-colors hover:text-white'
            >
              {t('New API')}
            </a>
          </span>
        </div>
      </div>
    </footer>
  )
}

export function HomeFooter() {
  const { systemName, logo, footerHtml } = useSystemConfig()
  const { status } = useStatus()

  if (getHomeFooterVariant(footerHtml) === 'custom') return <Footer />

  const docsLink =
    typeof status?.docs_link === 'string' ? status.docs_link : undefined

  return (
    <HomeFooterContent
      displayName={systemName || 'Molii'}
      displayLogo={logo || '/logo.png'}
      docsLink={docsLink}
      userAgreementEnabled={Boolean(status?.user_agreement_enabled)}
      privacyPolicyEnabled={Boolean(status?.privacy_policy_enabled)}
    />
  )
}
