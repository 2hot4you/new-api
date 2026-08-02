/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
export type VideoSampleLanguage =
  | 'curl'
  | 'python'
  | 'typescript'
  | 'javascript'

export type VideoSampleContext = {
  baseUrl: string
  apiKeyEnv: string
  modelName: string
  endpointPath: string
}

export function buildVideoSample(
  lang: VideoSampleLanguage,
  ctx: VideoSampleContext
): string {
  const url = `${ctx.baseUrl}${ctx.endpointPath}`
  const body = {
    model: ctx.modelName,
    content: [
      {
        type: 'text',
        text: 'A butterfly flies through a bamboo forest at sunrise, cinematic camera movement.',
      },
    ],
    generate_audio: true,
    resolution: '720p',
    ratio: '16:9',
    duration: 5,
    watermark: false,
  }
  const bodyJson = JSON.stringify(body, null, 2)

  if (lang === 'curl') {
    return [
      `CREATE_RESPONSE="$(curl -sS ${url} \\`,
      `  -H "Authorization: Bearer $${ctx.apiKeyEnv}" \\`,
      `  -H "Content-Type: application/json" \\`,
      `  -d '${bodyJson.replaceAll('\n', '\n     ')}')"`,
      '',
      `printf '%s\\n' "$CREATE_RESPONSE" | jq`,
      `TASK_ID="$(printf '%s' "$CREATE_RESPONSE" | jq -r '.id // .task_id')"`,
      '',
      `curl -sS "${url}/$TASK_ID" \\`,
      `  -H "Authorization: Bearer $${ctx.apiKeyEnv}" | jq`,
    ].join('\n')
  }

  if (lang === 'python') {
    const pythonBody = bodyJson
      .replaceAll(/\btrue\b/g, 'True')
      .replaceAll(/\bfalse\b/g, 'False')
    return [
      'import os',
      'import requests',
      '',
      `base_url = "${url}"`,
      `headers = {"Authorization": f"Bearer {os.environ['${ctx.apiKeyEnv}']}"}`,
      `payload = ${pythonBody}`,
      '',
      'created = requests.post(base_url, headers=headers, json=payload)',
      'created.raise_for_status()',
      'task = created.json()',
      'task_id = task.get("id") or task["task_id"]',
      'print(task)',
      '',
      'result = requests.get(f"{base_url}/{task_id}", headers=headers)',
      'result.raise_for_status()',
      'print(result.json())',
    ].join('\n')
  }

  const typedTask =
    lang === 'typescript' ? ' as { id?: string; task_id?: string }' : ''
  return [
    `const baseUrl = '${url}'`,
    `const headers = {`,
    `  Authorization: 'Bearer ' + process.env.${ctx.apiKeyEnv},`,
    `  'Content-Type': 'application/json',`,
    `}`,
    '',
    `const createResponse = await fetch(baseUrl, {`,
    `  method: 'POST',`,
    `  headers,`,
    `  body: JSON.stringify(${bodyJson}),`,
    `})`,
    `if (!createResponse.ok) throw new Error(await createResponse.text())`,
    `const task = (await createResponse.json())${typedTask}`,
    `const taskId = task.id ?? task.task_id`,
    `console.log(task)`,
    '',
    `const resultResponse = await fetch('${url}/' + taskId, { headers })`,
    `if (!resultResponse.ok) throw new Error(await resultResponse.text())`,
    `console.log(await resultResponse.json())`,
  ].join('\n')
}
