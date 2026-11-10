# 实施裁决与代价（按发生顺序）

1. 按用户CCG要求分文件并行，而非SDD默认串行；核心runtime仍单owner。判断错误的代价：跨模块集成返工，由整差异双路审查和完整测试缓解。
2. 用户已批准推荐A，直接在本地细化计划并执行，不重复请求方案批准；不扩展分布式managed协议。判断错误的代价：本地返工；不具备任何生产变更授权。
3. 外部antigravity/Claude wrapper缺失，以两路独立native代理替代并明示。代价：不能声称获得指定外部模型的审查认证。
4. 同revision FX变化需要持久RuntimeFingerprint，不能仅本地冻结。代价：增量schema列及部署协调；不支持混旧新二进制滚动发布承诺。
5. 从既有media price helper提取候选绑定派生方法，不修改计算/default。代价：新增内部接口及其回归维护，不是改图片/视频价格。
6. 错误码真实位于独立relaykit模块，最小延伸到request billing/Responses/task路径。代价：增加独立模块验证与传播契约维护。
7. reconciliation只缓存最后实际成功的完整输入指纹，保留原backfill，不复制第二套约30字段planner。代价：必须严格绑定操作后像，不能采样任意后续epoch。
8. 所有真实完整安装中央批次发布FX（包括Recover(nil)），不依赖可选loader。代价：增加一个内部安装边界，通用直接candidate与no-op不得产生额外副作用。
9. 不搁置审查发现的changed-policy真实竞态，增加原owner最小round3并重验。代价：超出技能偏好的一次最终修复波次，增加测试时间；不扩展业务范围。
10. 完整验收发现public Init真实缺口，采用bounded恢复入口，不以fixture强制ready掩盖。代价：旧void API可同步等待至30秒，必须安全日志/早退；增加fresh owner升级复审与全套重验。
11. 保留本次ignored原始RED/GREEN日志而非删除SDD scratch，用户要求可核验证据优先。代价：占用本地磁盘，路径明确；不保留或提交生产凭据，不清理相邻任务。
12. 最后FX修正只改独立测试函数：保留运行中race，并在受影响组编译前完成修正，另重验该组normal/race；fresh完整normal从最终源启动。代价：race不是整命令启动时单一测试源快照，需公开时点，不能把旧失败normal称为通过；生产源全程未再变。

原始逐次记录位于`.superpowers/sdd/plan-diagnose-catalog-publication-stall/progress.md`。本地提交不包含用户原有草稿/删除，不推送、不合并、不部署。
