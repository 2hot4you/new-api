# Volcengine 渠道重命名与 Grok 可达性测试设计

## 目标

将渠道类型 61 的全部用户可见品牌从 `Molii AIGC` 统一为 `Molii Volcengine Imagine API`，同时把渠道类型 62 的“配置检查”升级为零费用的网络可达性测试。

## 设计选择

### 类型 61：展示层完整重命名

后端渠道类型映射与前端渠道目录都使用新名称。计费页、素材上传流程、错误响应、脱敏后的上游错误、任务轮询日志和生成记录标签同步更新，避免同一功能出现两个品牌名称。

内部兼容标识保持不变：`ChannelTypeStarAI`、类型编号 61、`starai` 包与函数名、数据库记录以及模型 owner `molii-aigc` 均不重命名。这些名称属于实现或 API 稳定标识，不直接展示给最终用户；保留它们可降低与上游同步时的冲突范围。

### 类型 62：安全的可达性测试

复用类型 61 已有的网络检查模式，但使用独立的通用 TCP 检查函数：

1. 校验渠道对象、Key 和固定 Base URL。
2. 仅接受 `http` 或 `https` URL。
3. 从 URL 推导主机和端口，默认 HTTPS 443、HTTP 80。
4. 使用 5 秒上下文超时建立 TCP 连接并立即关闭。
5. 成功时返回“可达性测试通过，未发送付费请求”；失败时返回不包含密钥的可诊断错误。

该流程不做 TLS 握手、不带 Authorization Header、不调用任何图片或视频端点，因此不会产生上游费用。

## 组件影响

- `constant/channel.go`：渠道类型 61 的标准展示名称。
- `controller`、`relay/channel/task/starai`、`service`：用户可见错误、提示、脱敏品牌及网络测试逻辑。
- `web/src/features/channels`：渠道选项、动作名称与提示。
- `web/src/features/system-settings`、`temporary-assets`、`usage-logs`：计费、素材和生成记录文案。
- `web/src/i18n`：翻译键和值同步更新。
- `Dockerfile`：镜像描述中的产品能力名称。

## 测试设计

- 使用本地 TCP listener 验证 Grok 可达性成功，关闭端口验证失败；不访问真实上游。
- 验证缺少 Key、非法 URL 和不可达地址均返回安全错误。
- 前端验证类型 61 的新名称、类型 62 的“可达性测试”动作及配置提示。
- 更新品牌脱敏、素材错误、视频代理、轮询与注册测试的预期文案。

## 非目标

- 不修改渠道类型编号或数据库迁移。
- 不修改模型 owner、模型 ID、路由、计费或请求协议。
- 不访问真实 Grok/Volcengine 生成接口。
