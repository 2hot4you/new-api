# 审查结果

## 结论

- Critical：无。
- Warning：无。
- Info：参考视频与参考音频的数量、单项时长和同类媒体累计时长限制，已在模型广场参数、Seedance API 参考、多模态指南、模型页、ByteDance Provider 页面及公开 OpenAPI 中统一说明。

## 验证

- Web Seedance 定价/API 详情测试：5 通过，0 失败。
- Web 国际化完整性：通过。
- Web TypeScript 类型检查：通过。
- Web 格式检查：通过。
- Docusaurus Seedance 内容契约：12 通过，0 失败，236 次断言。
- 模型目录生成一致性：通过。
- Docusaurus 生产构建与 sitemap 生成：通过。
- 文档禁用词、密钥扫描和 OpenAPI lint：通过。
- `git diff --check`：通过。

## 交叉审查状态

按项目规范并行尝试调用 antigravity 与 Claude 审查；本机缺少 `/Users/naf/.claude/bin/codeagent-wrapper`，两个调用均以 127 结束。已改用本地完整差异检查和上述自动化验证补充审查。
