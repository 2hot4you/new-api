# StarAI 跨 Key 素材复用需求

- StarAI 已实测支持同一上游素材 ID 跨 API Key 查询并用于 Seedance 生成。
- 当前用户引用自己创建且仍有效的素材时，应向当前选中的 StarAI 生成渠道透传 `asset://<upstream_id>`，不得仅因渠道 ID 或 Key 指纹不同而回退为源 URL。
- 必须继续通过本地 `user_id + asset_id` 绑定校验素材归属；用户 A 不得查询或引用用户 B 的素材。
- 素材状态刷新优先使用原创建渠道；原渠道不可用时，可安全回退到任一启用的 StarAI 渠道查询同一上游素材 ID。
- 旧 `asset-molii-*` 映射继续兼容；公网 URL 与 COS URL 的既有行为保持不变。
- 不修改素材 168 小时有效期、计费、用户体系或其他渠道。
