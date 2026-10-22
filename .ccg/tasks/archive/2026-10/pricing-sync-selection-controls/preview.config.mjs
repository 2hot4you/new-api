// Local-only visual QA: no backend proxy and no pricing write API.
import path from 'node:path'
import { createRequire } from 'node:module'
import { fileURLToPath } from 'node:url'

const require = createRequire(path.resolve('package.json'))
const { pluginReact } = await import(require.resolve('@rsbuild/plugin-react'))
const { pluginTailwindcss } = await import(require.resolve('@rsbuild/plugin-tailwindcss'))

export default {
  plugins: [pluginReact(), pluginTailwindcss({ optimize: false })],
  source: { entry: { index: fileURLToPath(new URL('./preview.tsx', import.meta.url)) } },
  resolve: { alias: { '@': path.resolve('src') } },
  tools: { rspack: { resolve: { modules: [path.resolve('node_modules'), 'node_modules'] } } },
  html: { templateContent: '<!doctype html><html><head><meta name="viewport" content="width=device-width, initial-scale=1"><title>定价同步选择预览</title></head><body><div id="root"></div></body></html>' },
  server: { host: '127.0.0.1', port: 38175, strictPort: true },
}
