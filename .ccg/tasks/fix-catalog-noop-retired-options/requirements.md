# Approved follow-up — dev only

用户明确要求继续本地补修遗留键遗漏、补齐回归并双路复审，通过后重新推dev完整验收；**main和生产环境先不动**。该最新边界取代上层发布任务此前可能自动继续main的授权，本轮不作生产推广。

已批准设计依据为上层release任务review.md以及runtime-telemetry-review-a.md/b.md：LegacyAccessTokenRetireAt、theme.frontend由真实启动保留在DB，但故意不发布到OptionMap。optionsOnly预检错误地把它们的缺失视为有效配置变化，每轮申请W。R→W控制哨兵能正确恢复，但真实writer intent可短暂拒绝新定价读。根因不在DB业务行，不允许删改它们或绕过writer保护。

选定最窄代码修复：统一精确两键的运行态排除语义，供预检及原publisher共同使用。其他缺失/变化/需规范化的有效设置仍必须W；价格/FX/插件/路由变更继续完整proof与失败关闭。保留数据库deadline/theme原值及现有auth/兼容行为，不改TTL、验证流程、schema或业务价格。不只改日志颜色，不增加新生产测试钩子。

备选“删除DB键”会破坏服务器管理的认证/兼容状态，禁止；备选“所有缺失键都跳过/放松writer intent”会隐藏有效变更或引入饥饿，拒绝。通用遥测哨兵的标签改进不是本次根因修复，暂不扩大范围。

测试必须真实SQLite/MySQL/PostgreSQL：无遗留键、仅deadline、仅theme、两者同时；调用真实初始化函数产生行，稳定后多次no-op不进入W、不写选项/目录、不改变状态/fingerprint、遗留键仍不暴露到OptionMap且DB原值保留。并发测试在真实loader处协调，持有既有R时无变化同步应完成且新定价读可用；正控制真实有效设置变化仍进入W、发布生效，原价格/FX/pending故障门禁保持。新增测试进入现有mandatory PG normal/race选择，不减少库存/跳过要求。

复审与发布：外部codeagent-wrapper仍缺失，沿用透明native双路独立分析和复审，不冒称antigravity/Claude认证。只暂存自己的3个源/测试文件及任务记录。推送前fresh核develop仍精确5fb、main仍d922；外部前进先审查问方向。推后等待完整Verify/Release development及真实目标版本，再安全观察多轮同步与用户计费记录。没有live数据就保持待验收，不主动付费或写业务探针。不得因任务归档再推main。
