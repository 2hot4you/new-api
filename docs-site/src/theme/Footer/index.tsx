import useBaseUrl from '@docusaurus/useBaseUrl';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import type { ReactNode } from 'react';

import { ProviderIcon, type ProviderName } from './provider-icons';
import styles from './styles.module.css';

interface FooterLink {
  href: string;
  label: string;
  external?: boolean;
}

interface FooterColumnProps {
  links: FooterLink[];
  title: string;
}

interface Vendor {
  name: ProviderName;
}

const vendors: Vendor[] = [
  { name: 'OpenAI' },
  { name: 'Anthropic' },
  { name: 'Google' },
  { name: 'xAI' },
  { name: 'DeepSeek' },
  { name: 'Zhipu' },
  { name: 'MoonShot' },
  { name: 'MiniMax' },
  { name: 'Alibaba' },
  { name: 'ByteDance' },
];

function ArrowUpRight() {
  return (
    <svg
      aria-hidden='true'
      className={styles.arrow}
      viewBox='0 0 24 24'
      fill='none'
      stroke='currentColor'
      strokeWidth='2'
      strokeLinecap='round'
      strokeLinejoin='round'
    >
      <path d='M7 17 17 7' />
      <path d='M7 7h10v10' />
    </svg>
  );
}

function FooterLinkItem({ external, href, label }: FooterLink) {
  return (
    <a
      href={href}
      className={styles.link}
      rel={external ? 'noopener noreferrer' : undefined}
      target={external ? '_blank' : undefined}
    >
      <span>{label}</span>
      {external ? <ArrowUpRight /> : null}
    </a>
  );
}

function FooterColumn({ links, title }: FooterColumnProps) {
  return (
    <div className={styles.column}>
      <h2 className={styles.columnTitle}>{title}</h2>
      <nav aria-label={title} className={styles.linkList}>
        {links.map((link) => (
          <FooterLinkItem key={`${link.href}:${link.label}`} {...link} />
        ))}
      </nav>
    </div>
  );
}

function DeveloperColumn({ links }: { links: FooterLink[] }) {
  return (
    <div className={styles.column}>
      <FooterColumn title='开发者' links={links} />
      <div aria-label='兼容协议' className={styles.protocols}>
        {['OpenAI', 'Anthropic', 'Gemini'].map((protocol) => (
          <span key={protocol} className={styles.protocol}>
            {protocol}
          </span>
        ))}
      </div>
    </div>
  );
}

function VendorColumn({ platformUrl }: { platformUrl: (path: string) => string }) {
  return (
    <div className={styles.column}>
      <h2 className={styles.columnTitle}>厂商</h2>
      <nav aria-label='厂商' className={styles.linkList}>
        {vendors.map((vendor) => (
          <a
            key={vendor.name}
            href={platformUrl(`/pricing?vendor=${encodeURIComponent(vendor.name)}`)}
            className={styles.vendorLink}
          >
            <span
              aria-hidden='true'
              className={styles.vendorIcon}
              data-footer-vendor-icon={vendor.name}
            >
              <ProviderIcon name={vendor.name} />
            </span>
            <span>{vendor.name}</span>
          </a>
        ))}
      </nav>
    </div>
  );
}

function BrandLink({ children, href }: { children: ReactNode; href: string }) {
  return (
    <a href={href} className={styles.brandLink} aria-label='返回 Molii 首页'>
      {children}
    </a>
  );
}

export default function Footer() {
  const { siteConfig } = useDocusaurusContext();
  const logoUrl = useBaseUrl('/img/molii-wordmark.png');
  const quickStartUrl = useBaseUrl('/quick-start');
  const apiReferenceUrl = useBaseUrl('/api-reference');
  const authenticationUrl = useBaseUrl('/api-basics/authentication');
  const baseUrlGuideUrl = useBaseUrl('/api-basics/base-url');
  const errorsUrl = useBaseUrl('/api-basics/errors-retries');
  const changelogUrl = useBaseUrl('/changelog');
  const helpUrl = useBaseUrl('/help');
  const troubleshootingUrl = useBaseUrl('/help/troubleshooting');
  const contactSupportUrl = useBaseUrl('/help/contact-support');
  const platformUrl = (path: string) => new URL(path, `${siteConfig.url}/`).href;

  const productLinks: FooterLink[] = [
    { href: platformUrl('/pricing'), label: '模型广场' },
    { href: platformUrl('/playground'), label: '在线操练场' },
    { href: platformUrl('/keys'), label: 'API 密钥' },
    { href: platformUrl('/temporary-assets'), label: '临时素材' },
    { href: platformUrl('/usage-logs/task'), label: '生成记录' },
    { href: platformUrl('/usage-logs/common'), label: '使用日志' },
    { href: platformUrl('/wallet'), label: '钱包与账单' },
  ];
  const developerLinks: FooterLink[] = [
    { href: quickStartUrl, label: '快速开始' },
    { href: apiReferenceUrl, label: 'API 文档' },
    { href: platformUrl('/pricing'), label: '模型与价格' },
    { href: authenticationUrl, label: '身份验证' },
    { href: baseUrlGuideUrl, label: 'Base URL' },
    { href: errorsUrl, label: '错误与重试' },
    { href: changelogUrl, label: '更新日志' },
  ];
  const supportLinks: FooterLink[] = [
    { href: helpUrl, label: '帮助中心' },
    { href: troubleshootingUrl, label: '故障排查' },
    { href: contactSupportUrl, label: '联系支持' },
    { href: platformUrl('/about'), label: '关于' },
  ];

  return (
    <footer className={styles.footer}>
      <div className={styles.topRule} aria-hidden='true' />
      <div className={styles.container}>
        <div className={styles.grid}>
          <div className={styles.brand}>
            <BrandLink href={platformUrl('/')}>
              <img src={logoUrl} alt='Molii' className={styles.wordmark} />
            </BrandLink>
            <p className={styles.description}>
              通过统一 API Key 连接语言、图片与视频模型，并提供透明计费和完整生成记录。
            </p>
            <p className={styles.capabilities}>统一接入 · 透明计费 · 完整记录</p>
            <a href={platformUrl('/pricing')} className={styles.action}>
              探索模型广场
              <ArrowUpRight />
            </a>
          </div>

          <FooterColumn title='产品' links={productLinks} />
          <DeveloperColumn links={developerLinks} />
          <VendorColumn platformUrl={platformUrl} />
          <FooterColumn title='支持' links={supportLinks} />
        </div>

        <div className={styles.bottom}>
          <span>© {new Date().getFullYear()} Molii. 保留所有权利。</span>
          <span className={styles.attribution}>
            <span>OpenAI、Anthropic 与 Gemini 兼容 API</span>
            <span aria-hidden='true' className={styles.dot}>
              ·
            </span>
            <span>
              基于{' '}
              <a href='https://github.com/QuantumNous/new-api' rel='noopener noreferrer' target='_blank'>
                New API（QuantumNous）
              </a>
              构建
            </span>
          </span>
        </div>
      </div>
    </footer>
  );
}
