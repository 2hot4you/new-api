# new-api rc.46 升级评估

评估日期：2026-10-22（北京时间）。结论：**建议升级，但必须在 Molii 定制代码上集成并重新构建，不能直接替换成官方镜像。按六个版本审查，一次集成到 rc.46，先 dev，验收后再按授权发布生产。**

## 1. 已核实的基线与证据边界

- 官方最新 release：`v1.0.0-rc.46`，commit `7d8b91268dab69d7150f345c448b5e250163146a`，官方发布时间 2026-10-21 16:25:54 UTC，即北京时间 **2026-10-22 00:25:54**。官方声明修复 CVE-2026-111943，源码构建需要 Go1.26.10；这里引用官方说明，未独立审计 CVE。
- 最新已 fetch 的业务基线：develop `c2442919e29403123cf8e5bcd15eea4db4dcc3e6`，main `936aade39dcbb79071473a20cc16df96fcdd3e8a`；develop 是 main 的祖先。共同上游基线为 rc.40 `0aec08fee811ec6136828fda790551b49e410301`。
- 工作树 HEAD 仍是旧 `e05a25706`，未切换或覆盖用户文件。所有新版本影响均以最新远端 develop 比较，而非以旧工作区推断线上状态。业务基线当前 Go1.26.5，目标需提升为1.26.10。
- 新上游范围：150 commits，805 个变更文件。`git merge-tree --write-tree --name-only origin/develop v1.0.0-rc.46` 在 Git 对象层模拟合并，退出1，**92 个冲突路径（含29个 web 路径）**。没有进入真实 merge 或修改索引/业务工作树。路径数不等于修复工时或全部语义冲突数。
- 最新 develop CI [39738055037](https://github.com/2hot4you/new-api/actions/runs/39738055037) 已成功；前次 main CI [39459833878](https://github.com/2hot4you/new-api/actions/runs/39459833878) 已成功。本轮查看时 main 的 [39742533810](https://github.com/2hot4you/new-api/actions/runs/39742533810) 正在验证。没有干预该运行，也没有将这些 rc.40 定制代码的结果说成 rc.46 验收结果。
- 历史 `cached plan must not change result type` 问题已由 `5d611beb905ab32d60aa7b5c8289aeeadb3b2965` 修复，最新 develop 保留修复；旧任务的 blocked 状态不是当前升级阻塞。

## 2. 严格逐版对比

下表每一行都是相邻 tag 差异，不是把 rc.46 的说明当作整个升级范围；rc.41 发布说明虽然链接 rc.39..41，实际审查只取 rc.40..41。

| 目标版 | 与上一版相比 | 对本平台的关键影响 |
| --- | --- | --- |
| [rc.41](https://github.com/QuantumNous/new-api/releases/tag/v1.0.0-rc.41) | 22 commits /174 files。命名和权限化面板访问令牌、管理员用户操作二次验证；Sobek 改 moejs；Seedream 任务化及5.0 Flash；旧价格在 usage profile 收窄后保持可编辑 | 管理脚本的旧面板 token 仅有升级后30天过渡期，需替换；不是要求重建模型调用 API keys。插件运行时整体迁移，保留本地目录同步源码哈希和 generation pin。Seedance/Seedream 菜单及表达式生成必须兼容 |
| [rc.42](https://github.com/QuantumNous/new-api/releases/tag/v1.0.0-rc.42) | 26 commits /174 files。搜索编码和上游搜索次数结算；新 tokenkit 模块；json-order@1；xAI视频插件；Claude/Gemini→Responses 转换；Gemini流 usage 修正 | 搜索费用与文本token分开验收；新增模块必须进入 Docker、源码包和独立测试清单；官方 xAI 插件不能取代本地 MoliiGrok 的路由和历史任务身份 |
| [rc.43](https://github.com/QuantumNous/new-api/releases/tag/v1.0.0-rc.43) | 11 commits /335 files。后端错误/日志/通知多语言；DEFAULT_LANGUAGE；渠道 TLS 设置；utils.fetch；duration-auto@1；Seedance/Wan智能时长 | 中文默认可在后续部署配置 `DEFAULT_LANGUAGE=zh-CN`，未设置为英文；保留本地原生 Seedance `-1` 路径，同时纳入官方插件最长时长预留和实测结算；不能通过关闭证书校验绕过安全升级 |
| [rc.44](https://github.com/QuantumNous/new-api/releases/tag/v1.0.0-rc.44) | 41 commits /130 files。普通日志CSV导出和最长查询范围（默认365天）；账户隔离 Playground；缓存价与历史tier展示；Claude/Gemini工具和JSON Schema修复 | 提醒用户旧 Playground 浏览器聊天记录会清空一次；合并本地任务/视频日志、历史币种快照，不能只采用新通用日志UI；管理员确认导出权限及时间范围 |
| [rc.45](https://github.com/QuantumNous/new-api/releases/tag/v1.0.0-rc.45) | 31 commits /234 files。DeepSeek Flash/Pro、Search/Fetch；渠道优先级拖动；通知分类开关；模型按输入/输出/按次价排序；Vidu实测计费、Claude显式0及纯缓存结算修复 | 保留人民币报价、汇率及法定节假日峰谷逻辑；官方内置 DeepSeek 表达式以USD计价且只按周几/时间判断，不能覆盖现有配置；混合币种价格排序须先统一比较单位；敏感词开关默认变化需复核 |
| [rc.46](https://github.com/QuantumNous/new-api/releases/tag/v1.0.0-rc.46) | 19 commits /84 files。Go1.26.10安全修复；任务换渠道重定价和补预扣；任务请求头覆盖及重定向凭据保护；Claude保活；缓存统计；Resend；阿里多模态向量；Kling/DeepSeek余额修复 | Go工具链、builder和CI一起升级；任务更贵渠道必须先补预留、余额不足不得发送；保留CNY/USD快照及完成/退款幂等；既有请求头覆盖和跨域轮询要联调；新增缓存列和审计索引需验证迁移 |

文件数量是逐版本重复统计，不应相加当成独立文件数。

## 3. 升级的不可退化项

1. **已有渠道不能被重新解释。** 本地61/62是 MoliiVolcengine、MoliiGrok，63是 TaskPlugin；官方61是TaskPlugin、62/63是VLLM/SGLang。本地还保留64–66等扩展。直接启动官方镜像会错误理解持久化渠道类型。必须保留本地 constant、分发器、数据库身份和前端枚举一致；不重编号已有行。
2. **币种与扣费仍按已有规则。** 模型元数据 CNY 使用原人民币数值，USD 按配置汇率转人民币扣费；展示的原币/人民币切换不改变结算。请求冻结币种/汇率/表达式/插件版本，完成或退款不能读取后来变动的价格。新内置价格只作为未配置默认，不覆盖管理员已保存表达式、元数据、供应商。
3. **计费展示保留定制语义。** 单 tier 固定单价不能再显示成“动态计费/阶梯”；厂商显示顺序、同厂商聚合、发布日期排序、推荐和筛选、图像输出语义、混合币种排序必须回归。官方新排序提供新用户选择，不应悄悄改掉默认厂商排序。
4. **DeepSeek峰谷日历与Seedance菜单保留。** 日历覆盖边界和节假日回退、苹果日历导出、菜单生成表达式均保留。不要拿官方工作周USD默认式替代已维护的CNY含节假日表达式。Search/Fetch是独立按次服务，另验默认价和币种。
5. **受管目录同步不能降级。** 保留供应商/元数据/价格的changelog预览、单次确认、并发版本冲突、引用/删除保护、跨进程状态、编译源码哈希及插件generation租约。moejs移植不允许只替换engine而丢弃这些身份约束；不把发布当作自动执行目录apply。
6. **视频与图片自定义链路保留。** MoliiGrok、原生Seedance、上游任务、图片私密代理、临时素材、清理和持久化结算job不能被官方插件覆盖。智能时长、提交重试、轮询、立即完成、取消和退款均要覆盖。
7. **部署保留本地链路。** 保留品牌构建、APP_VERSION/OCI标签、内嵌日历、catalog-sync二进制、现有环境文件、immutable SHA/digest、健康检查及既有回滚；不能用上游ci.yml的广义make test覆盖PostgreSQL专用normal/race门槛。

## 4. 推荐实施顺序（尚未执行）

1. 以最新develop创建隔离升级分支，锁定官方rc.46 tag；留存当前定制镜像、PostgreSQL主库/独立日志库及options、插件、模型/供应商/价格目录备份。备份必须验证可恢复，不仅生成文件。
2. 一次集成rc.46全部依赖和相关提交，但按依赖分批解冲突：Go1.26.10/tokenkit/Docker → moejs及插件API/本地渠道身份 → token与管理API/数据迁移 → 计费与目录一致性 → i18n/前端/品牌。不是六次上线，也不手工挑150个提交以免漏依赖。
3. 在隔离PostgreSQL数据副本做全新初始化、旧版数据升级、第二次启动幂等、主库与日志库迁移；覆盖user_access_tokens表与旧token过渡option、quota_data两项缓存统计列、audit_logs时间/事件索引。账户余额、已存价格、供应商/元数据和在途任务不得丢失。只有镜像回滚不代表数据/插件/新凭据回退安全。
4. 执行Go1.26.10下root/relaykit/tokenkit独立vet/build及适用测试；扩展既有`make test-postgres`/`make test-postgres-race`的实际必测清单，不接受跳过。前端frozen安装、typecheck、测试、i18n、三品牌真实构建；用表驱动样例校验USD/CNY等价收费、纯缓存/显式0、节假日、跨渠道补预留、任务多次完成/退款、目录并发及快照。
5. 验收通过再按后续授权合入develop触发dev CI/CD。在dev上跑真实通道、小额结算和目录预览演练；验证通过后，再按授权合入main走四站现有CI/CD。价格和目录apply是独立写操作，不包含在版本升级中。

## 5. 本轮未做的事

未安装依赖、未编译升级版本、未运行数据库矩阵、未真正合并、未切换业务分支、未push、未部署、未改模型价格或服务器环境。合并模拟不是升级通过，原dev绿灯不是rc.46通过。外部模型执行器未调用，只用了两个原生Codex只读分域研究。

补充证据见同目录 `frontend-ops-research.md` 和 `backend-contract-research.md`。评估完成后仅归档并本地提交本任务记录，不推送、不触发任何CI/CD。
