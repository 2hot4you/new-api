# Review

## 根因

- 上一轮上传的 32×32 PNG 是完整 `molii` 字标，纵向有效内容很小，浏览器标签栏缩放后辨识度不足。
- `web/index.html` 的初始 `<title>` 仍是 `New API`，React 启动和 `/api/status` 返回后才更新标题。
- 旧的浏览器缓存还可能保存 `New API` 或 `NEWAPI`，缓存优先初始化会再次短暂覆盖标题。

## 修复

- 恢复上传 PNG 之前的粉色 `m` favicon、Apple Touch Icon 和 ICO。
- 使用 `v=4` 缓存版本，避免继续命中上一版小图。
- 初始 HTML 标题和 meta title 改为 `Molii Gateway`。
- 默认系统名称改为 `Molii Gateway`，并将历史 `New API` / `NEWAPI` 缓存归一为新品牌名；其他明确配置的系统名称仍保留。

## 验证

- 11 项 favicon 与系统名称测试通过，包含 RED→GREEN 回归过程。
- 前端类型检查通过。
- 定向 lint 无错误；`use-system-config.ts` 仍有 4 条本次未引入的既有事件监听写法警告。
- 格式检查和 `git diff --check` 通过。
- 3000、3001 的首屏 HTML 均直接返回 `<title>Molii Gateway</title>` 和 v4 favicon。
- 实际 3000 页面运行后标题、meta title 均为 `Molii Gateway`。
- 未执行生产构建，未调用 antigravity 或 Claude。

## 结论

无 Critical 或新增 Warning，可以交付本地开发环境验证。
