# Review

## Root cause

Molii Grok 专用的 `channelFormSchema` 无条件要求 API 密钥非空，没有区分新增与编辑。编辑表单出于安全原因不会回填已保存密钥，因此 Zod 在请求发送前错误拦截了合法更新。

通用编辑提交与后端均已有正确语义：空密钥不会出现在更新 payload 中，数据库保留原密钥。

## Fix

- Grok API 密钥只在新增渠道时必填。
- `is_editing: true` 时允许密钥留空。
- 更新 payload 继续省略空密钥，不发送空字符串覆盖原值。

## TDD evidence

- 新增回归测试后，观察到预期失败：`AssertionError: API key is required`。
- 加入编辑状态条件后，回归测试通过，并确认 payload 中不存在 `key`。

## Fresh verification

- 渠道模块测试：22 PASS，0 FAIL。
- TypeScript typecheck：PASS。
- 相关文件 oxlint：PASS。
- 相关文件 oxfmt：PASS。
- `git diff --check`：PASS。
- 3000 开发页面重新加载后，编辑已有 Grok 渠道并点击更新，不再出现 `API key is required`，API 密钥输入框保持空白。

## Result

Approved. No production build was run. No commit or push was performed.
