# 需求

- 控制台概览卡片标题改为“API 接入地址”。
- 说明改为“请选择距离业务部署区域最近的 Base URL，以获得更稳定的请求体验。各节点使用相同的 API Key、接口规范与账户额度。”
- API 接入地址卡片中的三个节点及其其他内容保持不变。
- 不采用下载目录中黑色大写 `M` 的 SVG；favicon 必须与 Molii 官方字标一致。
- favicon 从现有 `molii-wordmark.png` 精确提取原始粉色小写 `m`，不生成新字形。
- 首页主标题中的 `Molii` 复用 Header/Footer 的 `MoliiWordmark` 组件，不再使用普通字体逐字着色。
- 更新浏览器缓存版本，使用精确裁切的 PNG、Apple Touch Icon 与 ICO；删除不一致的手绘 SVG。
- 不构建生产前端，只运行本地测试、类型检查、格式与本地页面验证。

# 约束

- 保留用户已有修改。
- 不调用 antigravity 或 Claude。
- 不提交、不推送，除非用户后续明确要求。
