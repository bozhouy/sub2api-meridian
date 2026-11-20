# Sub2API Meridian

Meridian 是基于 [Sub2API](https://github.com/Wei-Shaw/sub2api) 的社区定制版。`0.2.23-meridian.2` 基于上游 `v0.2.23`，提供 Linux amd64 Docker 镜像，可在 x86-64 VPS 或使用 Linux 容器的 Docker Desktop 上运行。自此版本起，后台只检查本仓库的正式发布，并按数字比较 Meridian 修订号。

| 功能 | 使用方式 |
| --- | --- |
| 请求时区 | 在 OpenAI 账号的添加或编辑页选择固定时区、跟随出口 IP 或保留客户端时区。仅调整独立环境上下文和已有搜索工具的位置时区，保留历史日期和普通对话。 |
| 请求语言头 | OpenAI 网关把已有的 `Accept-Language` 统一为 `en-US,en;q=0.9`，缺失的头保持缺失。 |
| 模型指纹诊断 | 在账号菜单选择“降智检测”。使用自己的有效 API Key 并消耗额度。结果是指纹匹配参考，模型必须在该账号可用范围内。 |

## 已有 VPS 如何更新

1. 打开 [Releases](https://github.com/bozhouy/sub2api-meridian/releases)，选定目标版本。Git 标签带 `v`，镜像标签不带。例如 `v0.2.23-meridian.2` 对应 `0.2.23-meridian.2`。
2. 在原 Compose 项目目录执行 `docker compose config --services`，确认应用服务名称。**仅修改原应用服务的 `image` 行**，沿用原来的目录、端口、环境变量、PostgreSQL、Redis 和反向代理。

   ```yaml
   image: ghcr.io/bozhouy/sub2api-meridian:${SUB2API_VERSION:?set SUB2API_VERSION in .env}
   ```

3. 在原 `.env` 中新增或更新这一行。以后升级只改版本号。

   ```dotenv
   SUB2API_VERSION=0.2.23-meridian.2
   ```

4. 先拉取镜像，再备份 PostgreSQL（`pg_dump -Fc`）、原 Compose、`.env`、应用配置和持久化文件。已有备份脚本的部署可以直接运行原脚本。下面以应用服务名 `app` 为例，使用本仓库模板时服务名为 `sub2api`。

   ```bash
   docker compose config --quiet
   docker compose pull app
   # 完成备份并确认可以恢复后，替换应用容器
   docker compose up -d --no-deps app
   docker compose ps app
   docker compose logs --tail=50 app
   ```

5. 等待服务健康，访问部署地址的 `/health`，再用已有 Key 调用 `/v1/models` 并发一条实际请求。管理页保留原访问方式。涉及数据库迁移时，回退应使用相配的旧镜像、配置和升级前数据库备份；先保存升级后的新增数据。请勿用 `docker compose down -v` 更新。

## 新机器首次运行

以下命令适用于 Bash，需要 Git、Docker Compose v2 和 OpenSSL。在空目录运行。

```bash
git clone --depth=1 --branch v0.2.23-meridian.2 https://github.com/bozhouy/sub2api-meridian.git
cd sub2api-meridian/deploy
cp docker-compose.local.yml compose.yaml
umask 077
read -r -p '管理员邮箱: ' admin_email
{
  printf 'SUB2API_VERSION=0.2.23-meridian.2\nBIND_HOST=127.0.0.1\n'
  printf 'ADMIN_EMAIL=%s\n' "$admin_email"
  for key in POSTGRES_PASSWORD REDIS_PASSWORD JWT_SECRET TOTP_ENCRYPTION_KEY ADMIN_PASSWORD; do
    printf '%s=%s\n' "$key" "$(openssl rand -hex 32)"
  done
} > .env
docker compose up -d
docker compose ps
```

打开 `http://127.0.0.1:8080`，使用 `.env` 中的管理员邮箱和密码登录。远程 VPS 可通过现有 HTTPS 反向代理或 SSH 本地转发访问。后续继续在同一目录维护；迁移原有账号还需要恢复原数据库、应用配置和加密密钥。

## 镜像和源码

Google OAuth 的客户端密钥由部署者自行配置。使用 Gemini CLI 或 Antigravity 登录时，分别设置 `GEMINI_CLI_OAUTH_CLIENT_SECRET` 或 `ANTIGRAVITY_OAUTH_CLIENT_SECRET`；如使用自己的 Gemini OAuth 客户端，配置配套的客户端 ID 和密钥。

```bash
docker pull ghcr.io/bozhouy/sub2api-meridian:0.2.23-meridian.2
```

每个成功发布的版本也在 [Releases](../../releases) 提供 Docker 离线归档和 SHA-256。将两个文件下载到同一目录，执行 `sha256sum -c SHA256SUMS`，然后 `docker load -i sub2api-meridian-linux-amd64.tar.gz`。使用已加载的版本启动时可以运行 `docker compose up -d --pull never`。

维护者升级上游时，应先合入上游变更并回归测试三个定制功能，再更新 `backend/cmd/server/VERSION` 和 `deploy/Dockerfile.meridian` 的上游运行时镜像。推送匹配的 `vX.Y.Z-meridian.N` 标签会运行测试、构建、检查程序版本，并发布对应标签的镜像及离线包。后续部署以 Releases 中的版本号为准。

本项目保留上游 [LGPL 许可证](LICENSE)，时区、语言和诊断功能移植自 [Sub4API](https://github.com/MACOS-DO/sub4api)。模型指纹资源保留[原 MIT 声明](backend/internal/service/modeltrace_LICENSE)。更多实现细节见[功能说明](ENHANCEMENTS.md)和[上游文档](README_UPSTREAM.md)。
