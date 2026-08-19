export type GrokSampleLanguage = 'curl' | 'python' | 'typescript' | 'javascript'
export type GrokOperation =
  | 'generate'
  | 'edit'
  | 'extend'
  | 'reference'
  | 'status'
  | 'download'

type Context = {
  baseUrl: string
  modelName: string
  operation: GrokOperation
}

function endpoint(ctx: Context): {
  method: 'GET' | 'POST'
  path: string
  body?: object
} {
  const image = ctx.modelName.includes('-image')
  if (ctx.operation === 'status') {
    return { method: 'GET', path: '/v1/videos/task_xxx' }
  }
  if (ctx.operation === 'download') {
    return { method: 'GET', path: '/v1/videos/task_xxx/content' }
  }
  if (image) {
    const editing = ctx.operation === 'edit'
    return {
      method: 'POST',
      path: editing ? '/v1/images/edits' : '/v1/images/generations',
      body: {
        model: ctx.modelName,
        prompt: editing
          ? 'Preserve the subject and change the scene to a neon city.'
          : 'An orange cat in a neon city, cinematic lighting.',
        ...(editing ? { image: { file_id: 'file_image_xxx' } } : {}),
        aspect_ratio: '16:9',
        resolution: '1k',
        ...(ctx.modelName === 'grok-imagine-image-2.0'
          ? { quality: 'medium' }
          : {}),
        n: 1,
      },
    }
  }

  if (ctx.operation === 'extend') {
    return {
      method: 'POST',
      path: '/v1/videos/extensions',
      body: {
        model: 'grok-imagine-video',
        prompt: 'Continue the camera movement as the sun slowly sets.',
        video: { file_id: 'file_video_xxx' },
        duration: 6,
      },
    }
  }

  if (ctx.operation === 'reference') {
    return {
      method: 'POST',
      path: '/v1/videos/generations',
      body: {
        model: 'grok-imagine-video-1.5',
        prompt: 'Place the referenced subject in the referenced scene.',
        reference_images: [
          { file_id: 'file_reference_xxx' },
          { url: 'https://example.com/reference-scene.png' },
        ],
        duration: 5,
        aspect_ratio: '16:9',
        resolution: '720p',
      },
    }
  }
  const editing = ctx.operation === 'edit'
  return {
    method: 'POST',
    path: editing ? '/v1/videos/edits' : '/v1/videos',
    body: editing
      ? {
          model: ctx.modelName,
          prompt: 'Add gentle rain and cinematic lighting.',
          video: { file_id: 'file_video_xxx' },
        }
      : {
          model: ctx.modelName,
          prompt:
            'An orange cat runs through a neon city, cinematic camera movement.',
          image: { file_id: 'file_image_xxx' },
          duration: 5,
          aspect_ratio: '16:9',
          resolution: '720p',
        },
  }
}

export function buildGrokApiSample(
  lang: GrokSampleLanguage,
  ctx: Context
): string {
  const request = endpoint(ctx)
  const url = `${ctx.baseUrl}${request.path}`
  const body = request.body ? JSON.stringify(request.body, null, 2) : undefined
  if (lang === 'curl') {
    const hasFollowingLine = Boolean(body) || ctx.operation === 'download'
    return [
      `curl --location --request ${request.method} '${url}' \\`,
      `  --header 'Authorization: Bearer $MOLII_API_KEY'${hasFollowingLine ? ' \\' : ''}`,
      ...(body
        ? [
            `  --header 'Content-Type: application/json' \\`,
            `  --data '${body.replaceAll('\n', '\n          ')}'`,
          ]
        : []),
      ...(ctx.operation === 'download' ? ['  --output grok-result.mp4'] : []),
    ].join('\n')
  }
  if (lang === 'python') {
    return [
      'import os',
      'import requests',
      '',
      `response = requests.${request.method === 'GET' ? 'get' : 'post'}(`,
      `    "${url}",`,
      `    headers={"Authorization": f"Bearer {os.environ['MOLII_API_KEY']}"},`,
      ...(body ? [`    json=${body},`] : []),
      ')',
      'response.raise_for_status()',
      ctx.operation === 'download'
        ? 'open("grok-result.mp4", "wb").write(response.content)'
        : 'print(response.json())',
    ].join('\n')
  }
  const typed = lang === 'typescript' ? ': Record<string, string>' : ''
  return [
    `const headers${typed} = { Authorization: 'Bearer ' + process.env.MOLII_API_KEY${body ? ", 'Content-Type': 'application/json'" : ''} }`,
    `const response = await fetch('${url}', {`,
    `  method: '${request.method}',`,
    '  headers,',
    ...(body ? [`  body: JSON.stringify(${body}),`] : []),
    '})',
    'if (!response.ok) throw new Error(await response.text())',
    ctx.operation === 'download'
      ? 'const video = await response.arrayBuffer()'
      : 'console.log(await response.json())',
  ].join('\n')
}

export function getGrokOperations(modelName: string): GrokOperation[] {
  if (modelName.includes('-image')) {
    return ['generate', 'edit']
  }
  if (modelName === 'grok-imagine-video-1.5') {
    return ['generate', 'reference', 'status', 'download']
  }
  return ['generate', 'edit', 'extend', 'status', 'download']
}
