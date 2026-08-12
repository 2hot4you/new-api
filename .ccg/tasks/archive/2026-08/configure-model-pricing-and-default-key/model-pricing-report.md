# 三模型计费与模型广场报告

状态：完成（本工作流范围内）。未执行 production build、commit、push 或服务重启。

## 修改文件

- `setting/billing_setting/tiered_billing.go`：为 `minimax-m3`、`qwen3.5-flash` 和 `qwen3.5-plus` 提供默认 `tiered_expr` 模式与表达式。
- `setting/billing_setting/model_tiers_test.go`：先行添加输入窗口边界测试，并用低 `p`、高 `cr` 验证阶梯依据完整 `len` 而非扣除缓存后的输入。
- `model/pricing.go`：只为三个精确 ID 暴露模型广场元数据（1M 上下文、Qwen 65,536 最大输出、Text → Text、CNY 计费）。不暴露未经网关验证的多模态、工具或思考能力。
- `model/pricing_catalog_test.go`：验证上述精确 ID 元数据和范围隔离。
- `web/src/features/pricing/types.ts`：接收 `billing_currency: 'CNY'`。
- `web/src/features/pricing/lib/dynamic-price.ts`：CNY 表达式系数直接显示为人民币，不经过站点 USD 汇率/充值率转换。
- `web/src/features/pricing/components/model-details.tsx`：分组阶梯价格表保留 `billing_currency`。
- `web/src/features/pricing/lib/__tests__/dynamic-cny-pricing.test.ts`：验证 0.2 / 2 / 0.02 原样显示为 ¥0.2 / ¥2 / ¥0.02。

未修改认证、Token、Key 或部署迁移相关文件；未改动其他模型。

## 表达式与边界语义

DSL 中 `len` 是完整输入上下文长度；`p` 是扣除已单列计价子类别（本次为 `cr` 缓存命中）后的输入。因此，全部阶梯条件使用 `len`，费用本体使用 `p + c + cr`：

```text
minimax-m3
len <= 512000
  ? tier("up_to_512k", p * 2.1 + c * 8.4 + cr * 0.42)
  : tier("over_512k", p * 4.2 + c * 16.8 + cr * 0.84)

qwen3.5-flash
len <= 128000
  ? tier("up_to_128k", p * 0.2 + c * 2 + cr * 0.02)
  : len <= 256000
    ? tier("128k_to_256k", p * 0.8 + c * 8 + cr * 0.08)
    : tier("256k_to_1m", p * 1.2 + c * 12 + cr * 0.12)

qwen3.5-plus
len <= 128000
  ? tier("up_to_128k", p * 0.8 + c * 4.8 + cr * 0.08)
  : len <= 256000
    ? tier("128k_to_256k", p * 2 + c * 12 + cr * 0.2)
    : tier("256k_to_1m", p * 4 + c * 24 + cr * 0.4)
```

已验证的精确边界：MiniMax 512,000 / 512,001；两个 Qwen 模型均为 128,000 / 128,001、256,000 / 256,001、1,000,000。Qwen 的 1M 以内均落入第三档；模型目录声明的 1M 上下文限制是调用上界。

## 本地 PostgreSQL 配置

通过 Docker 中本地 PostgreSQL 直接事务写入，原因是未使用已有管理员设置 API（本工作流未取得管理员会话，也不读取密钥）。

事务前备份/检查：

- `options` 中 `billing_setting.billing_mode` 与 `billing_setting.billing_expr` 均不存在。
- `models` 中三个精确模型 ID 均不存在。

事务后：

- 两个 option key 仅包含且只包含 `minimax-m3`、`qwen3.5-flash`、`qwen3.5-plus`；均通过 PostgreSQL `jsonb` 解析验证。
- 三条模型目录记录已创建，均启用、关闭官方同步，端点为 `{"openai":"/v1/chat/completions"}`，卡片简介与标签明确为当前网关的 OpenAI-compatible Chat Completions 能力。
- 未读取、输出或修改任何真实密钥。

运行中的本地 `/api/pricing` 已在 option 同步周期后返回三条 `tiered_expr` 与相同表达式。新编译的目录字段（`billing_currency`、上下文/输出元数据）需要主代理按既有流程重启开发服务后才会由当前旧二进制返回；本工作流未自行重启或停止服务。

## 官方依据

- MiniMax [M3 官方模型页](https://www.minimax.io/models/text/m3)：1M context、M3 API 模型名与缓存说明；广场未转述其多模态/工具能力，因为本地网关当前只验证了 Chat Completions。
- MiniMax [llms.txt / API 索引](https://platform.minimaxi.com/docs/llms.txt)：列出 OpenAI-compatible Chat Completions、缓存和 M3 工具资料。
- 阿里云百炼 [Qwen3.5 Plus](https://help.aliyun.com/zh/model-studio/qwen3-5-plus)：1M 上下文、65,536 最大输出，以及北京区 0.8/4.8、2/12、4/24 与缓存命中价格。
- 阿里云百炼 [文本模型目录](https://help.aliyun.com/zh/model-studio/text-generation-model/)：两个 Qwen3.5 精确 ID 均为 1M 上下文。
- 阿里云百炼 [模型定价](https://help.aliyun.com/zh/model-studio/model-pricing)：Flash 0.2/2、0.8/8、1.2/12；Plus 0.8/4.8、2/12、4/24 的分段价格。
- 阿里云百炼 [深度思考](https://help.aliyun.com/zh/model-studio/deep-thinking)：Qwen3.5 商业版的思考模式来源。该能力未作为本地网关承诺写入目录。
- 提供的截图：MiniMax M3 512K 分界及三个模型的缓存命中人民币单价。

## 验证结果

通过：

```text
go test ./model ./setting/billing_setting -run 'Test(ThreeModelCatalogMetadataIsConservativeAndExact|DefaultTieredModelExpressionsUseFullInputLengthBoundaries)' -count=1
go test ./relay/helper ./service -run 'Test(ModelPriceHelper|TryTieredSettle|Tiered)' -count=1
go test ./model ./setting/billing_setting ./relay/helper ./service -count=1
cd web && bun test src/features/pricing/lib/__tests__ src/features/pricing/components/__tests__
git diff --check
```

Web 定向测试共 35 通过、0 失败；新增 CNY 动态价格测试通过。并行默认 Key 工作流完成其接口更新后，`cd web && bun run typecheck` 通过（0 错误）。

## 风险与后续

- 阶梯第三档刻意不为超过 1M 另设价格：三个模型的目录上下文上界是 1M，需求也只定义至 1M。
- 计费系数按需求和截图是人民币/百万 tokens；`billing_currency: CNY` 防止模型广场把它们当 USD 再换算。内部额度换算仍遵循现有 tiered-expression 路径；上线前应由主代理结合站点额度单位复核一次真实请求结算。
- 需主代理重启本地开发 API，使新编译的目录 metadata 生效；不需要为数据库中的表达式配置重启（现有同步已经生效）。
