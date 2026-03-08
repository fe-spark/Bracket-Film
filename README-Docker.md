# 🐳 Bracket-Film Docker 部署指南

方案采用 **Docker Compose** 实现一键化全栈部署，包含 Next.js 前端、Go 后端及内部网络隔离。

---

## 🏗️ 容器组件

- **`web`** (Frontend): Next.js 15 容器，暴露端口 `3000`。
- **`film`** (API Server): Go 后端容器，暴露端口 `3601`。
- **Network**: `film-network` (Bridge 模式) 确保前后端内部高效互联。

---

## 🚀 部署流程

### 1. 环境准备
确保已安装：
- **Docker** 20.10+
- **Docker Compose** 2.x+
- 可用的 **MySQL 8.0** 与 **Redis 7.0** 实例。

### 2. 环境变量配置
复制并编辑配置文件：
```bash
cp .env.example .env
```
**必填参数清单：**
- `MYSQL_HOST` / `MYSQL_PORT` / `MYSQL_USER` / `MYSQL_PASSWORD` / `MYSQL_DBNAME`
- `REDIS_HOST` / `REDIS_PORT` / `REDIS_PASSWORD`

### 3. 一键启动
```bash
docker compose up --build -d
```

---

## 🔍 验证与排障

### 常用命令
- **查看状态**：`docker compose ps`
- **查看日志**：`docker compose logs -f film` (后端) | `docker compose logs -f web` (前端)
- **重启服务**：`docker compose restart`

### 常见网络问题
- **host.docker.internal 不生效**：
  在 Linux 环境下，若容器无法访问宿主机数据库，请在 `docker-compose.yml` 中为 `film` 服务添加：
  ```yaml
  extra_hosts:
    - "host.docker.internal:host-gateway"
  ```
- **前端无法请求后端**：
  构建时会通过 `ARG API_URL` 注入后端地址。确保 `next.config.ts` 中的重写逻辑指向正确的容器名（默认为 `film`）。

---

## 💾 持久化挂载 (Persistence)

默认情况下，海报与缓存图片存储在容器内部。建议生产环境进行卷挂载以防数据丢失：
```yaml
services:
  film:
    volumes:
      - /path/to/your/gallery:/app/static/upload/gallery
```

---

## 🔐 生产安全建议
1. **反向代理**：建议使用 Nginx/Caddy 对 `3000` 端口进行反带并配置 SSL。
2. **端口隔离**：在防火墙中关闭 `3601` 端口的公网访问，仅暴露 `80/443`。
3. **敏感信息**：建立生产环境专用的 `.env`，切勿泄露默认数据库密码。
