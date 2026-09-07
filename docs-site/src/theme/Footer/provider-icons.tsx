import useBaseUrl from '@docusaurus/useBaseUrl';

// Static exports from @lobehub/icons 5.14.0 (MIT), already listed in
// THIRD-PARTY-LICENSES.md. Keeping the SVGs local avoids loading its UI peers.
const iconFiles = {
  Alibaba: 'alibaba.svg',
  Anthropic: 'anthropic.svg',
  ByteDance: 'bytedance.svg',
  DeepSeek: 'deepseek.svg',
  Google: 'google.svg',
  MiniMax: 'minimax.svg',
  MoonShot: 'moonshot.svg',
  OpenAI: 'openai.svg',
  xAI: 'xai.svg',
  Zhipu: 'zhipu.svg',
} as const;

export type ProviderName = keyof typeof iconFiles;

interface ProviderIconProps {
  name: ProviderName;
}

export function ProviderIcon({ name }: ProviderIconProps) {
  const fileName = iconFiles[name];
  const src = useBaseUrl(`/img/providers/${fileName}`);

  return <img alt='' aria-hidden='true' src={src} />;
}
