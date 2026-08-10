import type { SidebarsConfig } from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docsSidebar: [
    {
      type: 'category',
      label: '开始使用',
      items: ['getting-started/quickstart', 'getting-started/video-workflow'],
    },
    {
      type: 'category',
      label: 'API 基础',
      items: [
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
      label: '模型指南',
      items: ['models', 'models/overview', 'models/seedance-2'],
    },
    {
      type: 'category',
      label: '使用指南',
      items: ['guides/seedance-multimodal', 'guides/temporary-assets'],
    },
    {
      type: 'category',
      label: '用户平台操作',
      items: [
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
      label: '示例与工具',
      items: [
        'examples',
        'examples/seedance-curl',
        'examples/seedance-python',
        'examples/seedance-typescript',
      ],
    },
    'quick-start',
    'platform',
    'api-basics',
    'api-reference',
    'help',
    'changelog',
  ],
};

export default sidebars;
