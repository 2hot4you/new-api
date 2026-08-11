import type { SidebarsConfig } from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docsSidebar: [
    {
      type: 'category',
      label: '开始使用',
      collapsed: false,
      items: ['quick-start', 'getting-started/quickstart', 'getting-started/video-workflow'],
    },
    {
      type: 'category',
      label: '平台与账户',
      collapsed: false,
      items: [
        'platform',
        'platform/register-and-sign-in',
        'platform/dashboard',
        'platform/api-keys',
        'platform/model-square-and-playground',
        'platform/temporary-assets',
        'platform/usage-and-generation-records',
        'platform/wallet-and-billing',
        'platform/profile-and-security',
      ],
    },
    {
      type: 'category',
      label: '开发指南',
      items: [
        'api-basics',
        'api-basics/authentication',
        'api-basics/base-url',
        'api-basics/async-tasks',
        'api-basics/media-inputs',
        'api-basics/errors-retries',
        'api-basics/billing-and-usage',
      ],
    },
    {
      type: 'category',
      label: '模型与能力',
      items: [
        'models',
        'models/overview',
        'models/seedance-2',
        'models/grok-imagine-image',
        'models/grok-imagine-video',
      ],
    },
    {
      type: 'category',
      label: '进阶指南',
      items: ['guides/seedance-multimodal', 'guides/temporary-assets'],
    },
    {
      type: 'category',
      label: '示例与工具',
      items: [
        'examples',
        'examples/seedance-curl',
        'examples/seedance-python',
        'examples/seedance-typescript',
        'examples/grok-image-curl',
        'examples/grok-video-curl',
        'examples/grok-poll-download',
      ],
    },
    {
      type: 'category',
      label: 'API 参考',
      items: [
        'api-reference/index',
        'api-reference/models',
        'api-reference/images',
        'api-reference/videos',
        'api-reference/files',
        'api-reference/seedance',
        'api-reference/assets',
        'api-reference/errors',
      ],
    },
    {
      type: 'category',
      label: '帮助与更新',
      items: ['help', 'help/troubleshooting', 'help/contact-support', 'changelog'],
    },
  ],
};

export default sidebars;
