# Provider 与模型 API 文档设计

## 目标

以 Development 的公开 `https://dev.molii.co/api/pricing` 为权威目录，在现有 Docusaurus 文档站中按 Provider 分类，为当前每个公开模型 ID 生成可直接调用的 API 文档。当前基线为 10 个 Provider、35 个模型。

## 数据流

目录更新由显式的 `catalog:sync` 命令触发。同步脚本只访问固定的 Development 公开定价端点，校验 `success`、Provider、模型、排序和端点类型后，仅保留文档所需的公开字段，并原子写入版本化 JSON 快照。普通 `dev`、`build` 和测试不访问网络。

`catalog:generate` 从快照确定性生成 `docs/providers/`：每个 Provider 一个分类与总览页，每个模型 ID 一个 MDX 页面。生成前只清理该脚本拥有的 `docs/providers/`，不得触碰其他手写文档。开发与构建前自动运行生成器，因此已提交快照始终能够离线启动。

## 导航结构

侧边栏新增“Provider 与模型”主分类，内部顺序使用 Provider 的 `display_order`；Provider 内模型顺序使用模型的 `display_order`。现有“模型与能力”中的 Grok、Seedance 深度指南继续保留，并由对应生成页链接过去。

路由结构：

- `/providers`：Provider 与模型总览
- `/providers/{provider-slug}`：Provider 介绍与模型清单
- `/providers/{provider-slug}/{model-id}`：模型调用文档

Slug 只由公开名称/模型 ID 规范化得到；冲突、空 slug、未知 Provider 引用均导致同步或生成失败。

## 模型页面

每页使用 Docusaurus 默认 MDX 元素，包含：

1. 模型 ID、Provider、中文简介。
2. 输入/输出模态、能力、支持参数和限制。
3. Development 声明的全部兼容协议。
4. 每种协议一份可复制的安全 curl 示例。
5. 认证、同步/异步行为、下载和避免重复付费请求的提示。
6. 对应通用 API 参考及既有深度指南链接。

端点映射：

| endpoint type | 公开端点 |
| --- | --- |
| `openai` | `POST /v1/chat/completions` |
| `openai-response` | `POST /v1/responses` |
| `anthropic` | `POST /v1/messages` |
| `gemini` | `POST /v1beta/models/{model}:generateContent` |
| `image-generation` | `POST /v1/images/generations`，具备编辑能力时补充 `POST /v1/images/edits` |
| `openai-video` | `POST /v1/videos`、`GET /v1/videos/{task_id}`、`GET /v1/videos/{task_id}/content` |

未知 endpoint type 必须失败，不能生成猜测性示例。

## OpenAPI 与安全

公开 OpenAPI 补齐 Chat Completions、Responses、Anthropic Messages 和 Gemini GenerateContent。所有操作继续使用公开鉴权方式，不加入渠道、管理、系统配置或内部路由信息。

快照字段采用白名单，不包含分组倍率、渠道、管理员配置、密钥、内部地址或上游映射。示例只使用环境变量和保留域名，不出现真实 API Key、素材 URL 或用户数据。

## 验证

- 同步脚本针对畸形响应、未知端点、重复 slug 和非公开字段有失败测试。
- 生成器测试 Provider/模型一对一覆盖、排序、协议示例和离线确定性。
- OpenAPI 契约测试验证新增操作、Bearer/兼容鉴权、安全响应和唯一 operation ID。
- 内容契约验证侧边栏可达、无重复路由、无遗留模型遗漏。
- 完成后运行 `bun test`、`bun run api:lint`、`bun run build`、内部链接检查、密钥检查和差异检查。

