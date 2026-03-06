# Bracket-Film Docker 部署指南

本文档描述当前仓库（`server` + `web`）的 Docker Compose 部署方式。

> [!IMPORTANT]
> 根目录 `Dockerfile` 的后端源码路径应为 `server`，请确保 `COPY` / `ADD` 使用该路径。

## 架构概览

- `web`：Next.js 16（容器端口 `3000`）
- `film`：Go API 服务（容器端口 `3601`）
- `film-network`：前后端内部通信网络

## 环境准备

1. Docker 20.10+
2. Docker Compose 2.x+
3. 可访问的 MySQL 与 Redis（默认通过宿主机地址连接）

## 配置

### 1) 复制环境变量

```bash
cp .env.example .env
```

### 2) 修改 `.env`

必填项：

- `PORT`（默认 `3601`）
- `MYSQL_HOST` `MYSQL_PORT` `MYSQL_USER` `MYSQL_PASSWORD` `MYSQL_DBNAME`
- `REDIS_HOST` `REDIS_PORT` `REDIS_PASSWORD` `REDIS_DB`

### 3) 前端代理目标

`docker-compose.yml` 中 `web.build.args.API_URL` 默认是：

```text
http://film:3601
```

## 启动

```bash
docker compose up --build -d
```

常用检查命令：

```bash
docker compose ps
docker compose logs -f
```

## 访问地址

- 前端：`http://localhost:3000`
- 后端健康检查：`http://localhost:3601/api/index`

## 可选持久化挂载

当前后端图片目录为 `./static/upload/gallery`，可按需挂载：

```yaml
services:
  film:
    volumes:
      - ./server/static/upload/gallery:/app/static/upload/gallery
```

## 常见问题

### 1) Linux 下 `host.docker.internal` 不可用

可在 `film` 服务添加：

```yaml
extra_hosts:
  - "host.docker.internal:host-gateway"
```

### 2) 前端请求不到后端

确认 `web` 构建参数 `API_URL` 与 `film` 服务名一致，且后端接口是 `/api/*` 路径。

### 3) 构建内存不足

Next.js 构建较吃内存，建议至少 2G 可用内存，低内存机器可临时启用 Swap。

## 生产建议

1. 使用 Nginx 反代 `3000` 端口并配置 HTTPS。
2. 对公网仅开放 `80/443`，关闭 `3601` 外网访问。
3. 修改默认管理员密码。
