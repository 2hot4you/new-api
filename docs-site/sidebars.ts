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
    'quick-start',
    'platform',
    'api-basics',
    'models',
    'api-reference',
    'examples',
    'help',
    'changelog',
  ],
};

export default sidebars;
