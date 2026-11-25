# Sub2API Meridian

基于 Sub2API 的定制版，支持按账号设置请求时区、统一 OpenAI 请求语言头和模型指纹检测。当前版本为 `0.2.24-meridian.2`，镜像适用于 Linux amd64。

## 拉取镜像

```bash
docker pull ghcr.io/bozhouy/sub2api-meridian:0.2.24-meridian.2
```

## 首次安装

准备一台装有 Docker、Docker Compose v2、curl 和 OpenSSL 的 Linux x86-64 机器。以下命令使用 Bash，会启动 Meridian、PostgreSQL 和 Redis。

**1. 下载 Compose 文件**

```bash
mkdir sub2api-meridian
cd sub2api-meridian
curl -fL https://raw.githubusercontent.com/bozhouy/sub2api-meridian/v0.2.24-meridian.2/deploy/docker-compose.local.yml -o compose.yaml
```

**2. 配置管理员账号**

在新建的目录中运行下面这段命令。将 `admin@example.com` 换成自己的邮箱，管理员密码和服务密钥会自动生成并保存在 `.env`。

```bash
umask 077
cat > .env <<EOF
SUB2API_VERSION=0.2.24-meridian.2
ADMIN_EMAIL=admin@example.com
ADMIN_PASSWORD=$(openssl rand -hex 24)
POSTGRES_PASSWORD=$(openssl rand -hex 32)
REDIS_PASSWORD=$(openssl rand -hex 32)
JWT_SECRET=$(openssl rand -hex 32)
TOTP_ENCRYPTION_KEY=$(openssl rand -hex 32)
EOF
```

**3. 启动并登录**

```bash
docker compose up -d
docker compose ps
```

等三个服务都显示 `healthy` 后，在本机浏览器打开 `http://127.0.0.1:8080`，用 `.env` 中的 `ADMIN_EMAIL` 和 `ADMIN_PASSWORD` 登录。需要看启动日志时，运行 `docker compose logs --tail=100 sub2api`。

如果安装在远程 VPS，默认端口只监听服务器本机。在自己电脑上执行以下命令，将 `user@your-vps` 换成你的 SSH 登录地址。保持 SSH 连接，打开 `http://127.0.0.1:18080`。

```bash
ssh -L 18080:127.0.0.1:8080 -N user@your-vps
```

## 从现有 Sub2API 切换

先备份数据库、应用数据和 `.env`。在原部署目录打开 Compose 文件，将应用服务的 `image` 改为下面这一行。保留原服务名、端口、数据卷以及原有的 JWT 和 TOTP 密钥。

```yaml
image: ghcr.io/bozhouy/sub2api-meridian:${SUB2API_VERSION:?set SUB2API_VERSION in .env}
```

在原 `.env` 中设置 `SUB2API_VERSION=0.2.24-meridian.2`，再执行下面的更新命令。完成后用原管理员账号登录原网址。

## 已有 VPS 如何更新

在原部署目录，将 `.env` 的 `SUB2API_VERSION` 改为目标镜像标签（例如 `0.2.24-meridian.2`），然后拉取镜像并重建应用容器。模板中的应用服务名是 `sub2api`；如果自己的服务名是 `app`，将命令中的 `sub2api` 换成 `app`。使用自定义 `-f` 或 `-p` 的部署继续使用原参数。

```bash
docker compose pull sub2api && docker compose up -d --no-deps sub2api
docker compose ps sub2api
```

[发布版本](https://github.com/bozhouy/sub2api-meridian/releases) · [回退与离线安装](docs/docker.md) · [定制功能](ENHANCEMENTS.md)
