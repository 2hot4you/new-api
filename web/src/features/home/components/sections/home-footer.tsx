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
import { MoliiWordmark } from '@/components/layout/components/molii-wordmark'
import { useStatus } from '@/hooks/use-status'
import { useSystemConfig } from '@/hooks/use-system-config'
import { DEFAULT_LOGO } from '@/lib/constants'
import { getLobeIcon } from '@/lib/lobe-icon'

import { buildHomeDocsUrl, getHomeFooterVariant } from '../../lib/home-footer'
import type { HomeVendor } from '../../lib/home-model-catalog'

interface HomeFooterContentProps {
  displayName: string
  displayLogo: string
  docsLink?: string
  userAgreementEnabled: boolean
  privacyPolicyEnabled: boolean
  vendors: HomeVendor[]
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

function FooterDeveloperColumn(props: {
  title: string
  links: HomeFooterLink[]
  protocolLabel: string
}) {
  return (
    <div className='min-w-0'>
      <FooterColumn title={props.title} links={props.links} />
      <div
        aria-label={props.protocolLabel}
        className='mt-5 flex flex-wrap gap-1.5'
      >
        {['OpenAI', 'Anthropic', 'Gemini'].map((protocol) => (
          <span
            key={protocol}
            data-home-footer-protocol={protocol}
            className='rounded-md border border-white/10 bg-white/5 px-2 py-1 text-[11px] font-medium text-white/46'
          >
            {protocol}
          </span>
        ))}
      </div>
    </div>
  )
}

function FooterVendorColumn(props: { vendors: HomeVendor[] }) {
  const { t } = useTranslation()
  const title = t('Vendors')

  return (
    <div className='min-w-0'>
      <h3 className='text-xs font-semibold tracking-[0.16em] text-white/38 uppercase'>
        {title}
      </h3>
      <nav aria-label={title} className='mt-4 flex flex-col items-start gap-1'>
        {props.vendors.map((vendor) => (
          <a
            key={vendor.id ?? vendor.name}
            href={`/pricing?vendor=${encodeURIComponent(vendor.name)}`}
            data-home-footer-vendor={vendor.name}
            className='group/vendor flex min-h-8 max-w-full items-center gap-2 text-sm text-white/58 transition-[color,transform] hover:text-white motion-safe:hover:translate-x-0.5'
          >
            <span
              data-icon-key={vendor.icon || vendor.name}
              className='flex size-4 shrink-0 items-center justify-center'
            >
              {getLobeIcon(vendor.icon || vendor.name, 16)}
            </span>
            <span className='truncate'>{vendor.name}</span>
          </a>
        ))}
      </nav>
    </div>
  )
}

export function HomeFooterContent(props: HomeFooterContentProps) {
  const { t } = useTranslation()
  const currentYear = new Date().getFullYear()
  const useMoliiWordmark = props.displayLogo === DEFAULT_LOGO
  const quickStartLink = buildHomeDocsUrl(
    props.docsLink,
    '/getting-started/quickstart'
  )
  const apiReferenceLink = buildHomeDocsUrl(props.docsLink, '/api-reference')
  const authenticationLink = buildHomeDocsUrl(
    props.docsLink,
    '/api-basics/authentication'
  )
  const baseUrlLink = buildHomeDocsUrl(props.docsLink, '/api-basics/base-url')
  const errorsAndRetriesLink = buildHomeDocsUrl(
    props.docsLink,
    '/api-basics/errors-retries'
  )
  const changelogLink = buildHomeDocsUrl(props.docsLink, '/changelog')
  const helpCenterLink = buildHomeDocsUrl(props.docsLink, '/help')
  const troubleshootingLink = buildHomeDocsUrl(
    props.docsLink,
    '/help/troubleshooting'
  )
  const contactSupportLink = buildHomeDocsUrl(
    props.docsLink,
    '/help/contact-support'
  )

  const productLinks: HomeFooterLink[] = [
    { href: '/pricing', label: t('Model marketplace') },
    { href: '/playground', label: t('footer.home.onlinePlayground') },
    { href: '/keys', label: t('API Keys') },
    { href: '/temporary-assets', label: t('footer.home.temporaryAssets') },
    { href: '/usage-logs/task', label: t('Generation Records') },
    { href: '/usage-logs/common', label: t('Usage Logs') },
    { href: '/wallet', label: t('footer.home.walletAndBilling') },
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
    ...(authenticationLink
      ? [
          {
            href: authenticationLink,
            label: t('footer.home.authentication'),
            external: true,
          },
        ]
      : []),
    ...(baseUrlLink
      ? [
          {
            href: baseUrlLink,
            label: t('footer.home.baseUrl'),
            external: true,
          },
        ]
      : []),
    ...(errorsAndRetriesLink
      ? [
          {
            href: errorsAndRetriesLink,
            label: t('footer.home.errorsAndRetries'),
            external: true,
          },
        ]
      : []),
    ...(changelogLink
      ? [
          {
            href: changelogLink,
            label: t('footer.home.changelog'),
            external: true,
          },
        ]
      : []),
  ]
  const supportLinks: HomeFooterLink[] = [
    ...(helpCenterLink
      ? [
          {
            href: helpCenterLink,
            label: t('footer.home.helpCenter'),
            external: true,
          },
        ]
      : []),
    ...(troubleshootingLink
      ? [
          {
            href: troubleshootingLink,
            label: t('footer.home.troubleshooting'),
            external: true,
          },
        ]
      : []),
    ...(contactSupportLink
      ? [
          {
            href: contactSupportLink,
            label: t('footer.home.contactSupport'),
            external: true,
          },
        ]
      : []),
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
        <div className='grid min-w-0 gap-10 sm:grid-cols-2 lg:grid-cols-[1.4fr_repeat(4,1fr)] lg:gap-14'>
          <div className='min-w-0 sm:col-span-2 lg:col-span-1'>
            <a href='/' className='inline-flex items-center gap-3'>
              {useMoliiWordmark ? (
                <MoliiWordmark
                  data-home-footer-wordmark
                  alt={props.displayName}
                  className='h-12 max-w-[7.5rem]'
                />
              ) : (
                <>
                  <img
                    src={props.displayLogo}
                    alt={props.displayName}
                    className='size-9 rounded-xl bg-white/8 object-contain p-1'
                  />
                  <span className='text-lg font-semibold tracking-tight'>
                    {props.displayName}
                  </span>
                </>
              )}
            </a>
            <p className='mt-5 max-w-md text-sm leading-7 text-white/52'>
              {t('footer.home.brandDescription')}
            </p>
            <p className='mt-4 inline-flex rounded-full border border-white/10 bg-white/5 px-3 py-1.5 text-xs leading-5 text-white/46'>
              {t('footer.home.capabilitySummary')}
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
          <FooterDeveloperColumn
            title={t('footer.home.developers')}
            links={developerLinks}
            protocolLabel={t('footer.home.supportedProtocols')}
          />
          <FooterVendorColumn vendors={props.vendors} />
          <FooterColumn title={t('footer.home.support')} links={supportLinks} />
        </div>

        <div className='mt-14 flex min-w-0 flex-col gap-3 border-t border-white/10 pt-6 text-xs text-white/34 sm:flex-row sm:items-center sm:justify-between'>
          <span>
            &copy; {currentYear} {props.displayName}.{' '}
            {t('footer.defaultCopyright')}
          </span>
          <span className='flex flex-wrap items-center gap-x-2 gap-y-1'>
            <span>{t('footer.home.compatibleApis')}</span>
            <span aria-hidden className='text-white/18'>
              ·
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
          </span>
        </div>
      </div>
    </footer>
  )
}

export function HomeFooter(props: { vendors: HomeVendor[] }) {
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
      vendors={props.vendors}
    />
  )
}
