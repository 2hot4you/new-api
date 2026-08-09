import Link from '@docusaurus/Link';
import Layout from '@theme/Layout';

import styles from './index.module.css';

const guides = [
  ['快速开始', '在几分钟内完成首个请求。', '/quick-start'],
  ['平台', '了解 Molii 平台的核心能力。', '/platform'],
  ['API 基础', '认证、请求与错误处理约定。', '/api-basics'],
  ['模型', '选择适合创作任务的模型。', '/models'],
  ['API 参考', '查阅完整接口定义。', '/api-reference'],
  ['示例', '从可运行的调用模式开始。', '/examples'],
  ['帮助', '获取常见问题和支持渠道。', '/help'],
  ['更新日志', '跟进产品与 API 的变化。', '/changelog'],
] as const;

export default function Home(): JSX.Element {
  return (
    <Layout title="开发者文档" description="Molii 开发者文档">
      <main>
        <section className={styles.hero}>
          <div className={styles.heroContent}>
            <p className={styles.eyebrow}>MOLII DEVELOPER</p>
            <h1>从想法到创作，只需一次清晰的调用。</h1>
            <p>
              面向开发者的 Molii 产品、模型与 API 文档。使用简洁的接口，将 AI 创作能力融入你的产品。
            </p>
            <div className={styles.actions}>
              <Link className="button button--primary button--lg" to="/quick-start">
                开始使用
              </Link>
              <Link className="button button--secondary button--lg" to="/api-reference">
                查看 API 参考
              </Link>
            </div>
          </div>
        </section>
        <section className={styles.guides} aria-label="文档导航">
          {guides.map(([title, description, to]) => (
            <Link className={styles.guideCard} key={to} to={to}>
              <h2>{title}</h2>
              <p>{description}</p>
              <span aria-hidden="true">探索 →</span>
            </Link>
          ))}
        </section>
      </main>
    </Layout>
  );
}
