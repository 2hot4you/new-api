# 审查记录

## 外部模型

- 已按 CCG 规范并行尝试 antigravity 与 Claude 分析。
- 已按 CCG 规范并行尝试 antigravity 与 Claude 审查。
- 本机缺少 `/Users/naf/.claude/bin/codeagent-wrapper`，两个调用均以 status 127 失败。

## TDD

- RED：Footer 测试因缺少 `/playground` 等新增入口而按预期失败。
- GREEN：实现完整导航、协议徽标与辅助说明后，Footer 定向测试通过。
- RED：可访问性测试因协议徽标使用硬编码英文 aria-label 而按预期失败。
- GREEN：aria-label 改为完整 i18n 后测试通过。

## 人工审查

- Critical：0
- Warning：0
- Info：0

确认项：

- 产品入口均对应现有前端路由。
- 开发者和支持入口均由经过安全校验的 `docsLink` 派生；未配置文档地址时不会生成空链接。
- 所有外链继续使用 `noopener noreferrer`。
- 协议徽标、品牌能力说明和底栏兼容性说明均已国际化。
- Footer 没有固定高度或内部滚动，内容增多时自然增高；现有移动端两列布局不变。
- 自定义 Footer HTML 分支未改变。
