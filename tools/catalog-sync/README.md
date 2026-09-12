# 模型广场与定价目录同步

`catalog-sync` 只同步厂商、模型广场元数据，以及模型、图片、视频和工具调用价格。它不会同步分组、渠道、渠道能力、API 密钥、用户、余额、日志、任务、临时素材、Redis 或站点品牌。

目标环境没有配置对应启用渠道时，同步后的模型不会出现在 `/pricing`，这是正常行为。

## 安全机制

- `export` 只读取源数据库。
- `plan` 只读取目标数据库，默认行为不会写入。
- `apply` 必须携带最近一次 `plan` 返回的 `confirmation_digest`。
- 目标状态发生变化后，旧摘要立即失效。
- `apply` 在事务内自动保存目标目录和价格快照；备份失败则不写数据库。
- 厂商和模型只新增或更新，不删除目标记录；目标独有模型及其价格项会保留。
- 数据库连接串只能通过环境变量提供，命令行参数和输出中不会出现连接串。

## 方式一：在已部署容器中运行

发布包含本功能的新镜像后，容器内会提供 `/catalog-sync`。以下命令中的容器名称分别为当前 CI/CD 约定的示例，请先用 `docker ps` 核对。

### 1. 从开发环境导出

```bash
sudo -u molii-deploy install -d -m 0700 \
  /opt/molii/development/data/catalog-sync

sudo -u molii-deploy docker exec \
  --user "$(id -u molii-deploy):$(id -g molii-deploy)" \
  molii-development \
  /catalog-sync export \
  --dsn-env SQL_DSN \
  --output /data/catalog-sync/development-catalog.json
```

导出命令会输出快照路径、厂商数、模型数、价格配置数和 `content_digest`。

先从开发服务器取回快照，再分别安全复制到两台生产服务器：

```bash
scp -P <开发环境 SSH端口> \
  molii-deploy@<开发环境服务器>:/opt/molii/development/data/catalog-sync/development-catalog.json \
  ./development-catalog.json

chmod 600 ./development-catalog.json

ssh -p <Molii SSH端口> molii-deploy@<Molii服务器> \
  'install -d -m 0700 /opt/molii/production/data/catalog-sync'

ssh -p <iXiaozu SSH端口> molii-deploy@<iXiaozu服务器> \
  'install -d -m 0700 /opt/ixiaozu/production/data/catalog-sync'

scp -P <Molii SSH端口> development-catalog.json \
  molii-deploy@<Molii服务器>:/opt/molii/production/data/catalog-sync/development-catalog.json

scp -P <iXiaozu SSH端口> development-catalog.json \
  molii-deploy@<iXiaozu服务器>:/opt/ixiaozu/production/data/catalog-sync/development-catalog.json

ssh -p <Molii SSH端口> molii-deploy@<Molii服务器> \
  'chmod 600 /opt/molii/production/data/catalog-sync/development-catalog.json'

ssh -p <iXiaozu SSH端口> molii-deploy@<iXiaozu服务器> \
  'chmod 600 /opt/ixiaozu/production/data/catalog-sync/development-catalog.json'
```

三个位置的快照文件权限均应为 `0600`。

### 2. 在目标环境预览差异

Molii：

```bash
sudo -u molii-deploy docker exec \
  --user "$(id -u molii-deploy):$(id -g molii-deploy)" \
  molii-production \
  /catalog-sync plan \
  --dsn-env SQL_DSN \
  --input /data/catalog-sync/development-catalog.json \
  --output /data/catalog-sync/molii-plan.json
```

iXiaozu：

```bash
sudo -u molii-deploy docker exec \
  --user "$(id -u molii-deploy):$(id -g molii-deploy)" \
  ixiaozu-production \
  /catalog-sync plan \
  --dsn-env SQL_DSN \
  --input /data/catalog-sync/development-catalog.json \
  --output /data/catalog-sync/ixiaozu-plan.json
```

检查输出中的新增、更新和特殊价格清理项。记录每个目标各自返回的 `confirmation_digest`，两个环境的摘要通常不同，不能混用。

### 3. 显式应用

Molii：

```bash
sudo -u molii-deploy docker exec \
  --user "$(id -u molii-deploy):$(id -g molii-deploy)" \
  molii-production \
  /catalog-sync apply \
  --dsn-env SQL_DSN \
  --input /data/catalog-sync/development-catalog.json \
  --confirm 'sha256:<molii-plan返回的摘要>' \
  --backup-dir /data/catalog-sync/backups \
  --target-name molii-production
```

iXiaozu：

```bash
sudo -u molii-deploy docker exec \
  --user "$(id -u molii-deploy):$(id -g molii-deploy)" \
  ixiaozu-production \
  /catalog-sync apply \
  --dsn-env SQL_DSN \
  --input /data/catalog-sync/development-catalog.json \
  --confirm 'sha256:<ixiaozu-plan返回的摘要>' \
  --backup-dir /data/catalog-sync/backups \
  --target-name ixiaozu-production
```

应用完成后重启对应应用容器，使进程内价格缓存与数据库完全一致：

```bash
sudo -u molii-deploy docker restart molii-production
sudo -u molii-deploy docker restart ixiaozu-production
```

最后检查：

```bash
curl --fail-with-body --silent --show-error https://molii.co/api/pricing >/dev/null
curl --fail-with-body --silent --show-error https://aigc.ixiaozu.cn/api/pricing >/dev/null
```

并分别打开两个站点的 `/pricing`，核对厂商图标、模型能力、币种、单位和价格。没有目标渠道支持的模型不会显示。

## 方式二：从源码构建独立工具

```bash
CGO_ENABLED=0 go build -o catalog-sync ./cmd/catalog-sync
```

运行时将连接串放入环境变量，不要直接写在命令参数中：

```bash
export CATALOG_SYNC_SQL_DSN='postgresql://...'
./catalog-sync plan \
  --dsn-env CATALOG_SYNC_SQL_DSN \
  --input development-catalog.json
unset CATALOG_SYNC_SQL_DSN
```
