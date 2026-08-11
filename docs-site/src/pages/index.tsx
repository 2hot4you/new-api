import Link from '@docusaurus/Link';
import Heading from '@theme/Heading';
import Layout from '@theme/Layout';
import type { ReactNode } from 'react';

import styles from './index.module.css';

type PortalCard = {
  eyebrow: string;
  title: string;
  description: string;
  href: string;
  linkLabel: string;
};

const paths: PortalCard[] = [
  {
    eyebrow: '01 · 第一次使用',
    title: '发送第一个请求',
    description: '创建 API Key，完成图片生成请求，并正确读取响应与费用。',
    href: '/quick-start',
    linkLabel: '开始教程',
  },
  {
    eyebrow: '02 · 视频工作流',
    title: '提交并跟踪异步任务',
    description: '掌握 Seedance 与 Grok 视频创建、轮询、下载和最终结算。',
    href: '/getting-started/video-workflow',
    linkLabel: '查看工作流',
  },
  {
    eyebrow: '03 · 已有系统',
    title: '直接对接 API',
    description: '查阅完整端点、参数范围、错误码、媒体输入与重试边界。',
    href: '/api-reference',
    linkLabel: '打开 API 参考',
  },
];

const taskSteps = [
  ['STEP 1', '提交一次', '付费 POST 不自动重试，保存任务 ID。'],
  ['STEP 2', '安全轮询', '按建议间隔查询，处理超时与 Retry-After。'],
  ['STEP 3', '获取结果', '任务成功后读取可播放的媒体地址。'],
  ['STEP 4', '核对费用', '区分预计价格、实际 Token 与最终结算。'],
] as const;

const resources = [
  ['多模态媒体输入规范', '/api-basics/media-inputs'],
  ['临时素材生命周期', '/guides/temporary-assets'],
  ['错误处理与 Request ID', '/api-basics/errors-retries'],
  ['完整 curl、Python 与 TypeScript 示例', '/examples'],
] as const;

export default function Home(): ReactNode {
  return (
    <Layout
      title="Molii 开发者文档"
      description="使用 Molii API 构建可靠的 AI 图片与视频创作体验"
    >
      <main className={styles.portal}>
        <header className={styles.hero}>
          <div className={styles.heroCopy}>
            <p className={styles.eyebrow}>Molii Developer Platform</p>
            <Heading as="h1">把 AI 图片与视频能力，可靠地接入你的产品</Heading>
            <p className={styles.lead}>
              从第一个请求到生产上线，完整了解模型选择、媒体输入、异步任务、费用结算和错误恢复。
            </p>
            <div className={styles.actions}>
              <Link className="button button--primary" to="/quick-start">5 分钟快速开始</Link>
              <Link className="button button--secondary" to="/api-reference">浏览 API 参考</Link>
            </div>
            <ul className={styles.proofList}>
              <li><strong>统一鉴权</strong><span>Bearer API Key</span></li>
              <li><strong>多模态</strong><span>图片 · 视频 · 音频</span></li>
              <li><strong>任务可追踪</strong><span>状态 · 用量 · 计费</span></li>
            </ul>
          </div>
          <div className={styles.codeCard} aria-label="Seedance 请求示例">
            <div className={styles.codeHeader}>创建 Seedance 视频任务</div>
            <pre><code>{`curl -X POST https://api.molii.co/v1/video/generations \\
  -H "Authorization: Bearer $MOLII_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "doubao-seedance-2-0-260128",
    "prompt": "清晨的竹林，电影感运镜",
    "resolution": "1080p",
    "ratio": "16:9",
    "duration": 5
  }'`}</code></pre>
          </div>
        </header>

        <section className={styles.section}>
          <Heading as="h2">选择你的接入路径</Heading>
          <p className={styles.sectionLead}>根据目标选择最短阅读路径，不必先理解全部文档结构。</p>
          <div className={styles.cardGrid}>
            {paths.map((path) => (
              <article className={styles.pathCard} key={path.href}>
                <p className={styles.cardEyebrow}>{path.eyebrow}</p>
                <Heading as="h3">{path.title}</Heading>
                <p>{path.description}</p>
                <Link to={path.href}>{path.linkLabel} →</Link>
              </article>
            ))}
          </div>
        </section>

        <section className={styles.section}>
          <Heading as="h2">模型与能力一目了然</Heading>
          <div className={styles.modelGrid}>
            <article className={styles.featuredModel}>
              <span>推荐 · 视频生成</span>
              <Heading as="h3">Seedance 2.0</Heading>
              <p>支持文生视频、首尾帧、多参考图片、参考视频和参考音频组合。</p>
              <Link to="/models/seedance-2">查看模型能力 →</Link>
            </article>
            <article className={styles.modelCard}>
              <Heading as="h3">Seedance 2.0 Fast</Heading>
              <p>面向 480p 与 720p 的快速视频生成工作流。</p>
              <Link to="/models/seedance-2">查看 Fast 版本 →</Link>
            </article>
            <article className={styles.modelCard}>
              <Heading as="h3">Grok Imagine</Heading>
              <p>覆盖图片生成、图片编辑和异步视频任务。</p>
              <Link to="/models">查看 Grok 模型 →</Link>
            </article>
          </div>
        </section>

        <section className={styles.section}>
          <Heading as="h2">从请求到结算的完整链路</Heading>
          <div className={styles.stepGrid}>
            {taskSteps.map(([number, title, description]) => (
              <article key={number}>
                <small>{number}</small>
                <Heading as="h3">{title}</Heading>
                <p>{description}</p>
              </article>
            ))}
          </div>
        </section>

        <section className={styles.section}>
          <Heading as="h2">生产环境接入清单</Heading>
          <p className={styles.sectionLead}>上线前检查密钥管理、超时、重试、媒体输入、安全下载和费用结算。</p>
          <Link className="button button--secondary" to="/api-basics">查看完整清单</Link>
        </section>

        <section className={styles.section}>
          <Heading as="h2">继续深入</Heading>
          <div className={styles.resourceGrid}>
            {resources.map(([label, href]) => <Link key={href} to={href}>{label}<span>→</span></Link>)}
          </div>
        </section>
      </main>
    </Layout>
  );
}
