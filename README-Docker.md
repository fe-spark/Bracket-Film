# Bracket-Film Docker 部署详细指南

本文档提供使用 Docker Compose 进行生产级部署的详细步骤。本系统采用前后端分离架构，通过 Docker 编排实现一键启动。

---

## 🏗️ 架构概览

- **前端 (web)**: 基于 Next.js 15 (React 19) 构建，默认监听 `3000` 端口。
- **后端 (film)**: 基于 Go (Gin) 构建，默认监听 `3601` 端口。
- **网络**: 容器间通过 `film-network` 内部网络通信，前端通过内置代理访问后端 API。

---

## 🛠️ 环境准备

1. **Docker**: 推荐版本 20.10.x+
2. **Docker Compose**: 推荐版本 2.x+
3. **外部依赖**: 
   - **MySQL**: 需创建数据库 `FilmSite`。
   - **Redis**: 用于缓存采集数据。
   > **Note**: 如果您希望用一个 Compose 文件启动所有服务（包括数据库），可自行在 `docker-compose.yml` 中添加 mysql 和 redis 服务。

---

## 📝 配置步骤

### 1. 环境变量配置
复制模板并根据实际情况修改：
```bash
cp .env.example .env
```

**`.env` 核心参数说明：**

| 变量名 | 必填 | 默认值 | 说明 |
| :--- | :---: | :--- | :--- |
| `PORT` | 否 | `3601` | 后端服务端口 |
| `MYSQL_HOST` | 是 | `host.docker.internal` | 数据库地址（指向宿主机） |
| `MYSQL_USER` | 是 | `root` | 数据库用户名 |
| `MYSQL_DBNAME` | 是 | `FilmSite` | 数据库名 |
| `REDIS_HOST` | 是 | `host.docker.internal` | Redis 地址 |
| `JWT_SECRET` | 否 | `film-secret` | 登录鉴权密钥，建议修改 |

### 2. 构建参数说明
在 `docker-compose.yml` 中，`web` 服务有一个 `args`:
- `API_URL`: 指向后端服务的地址（内部网络默认为 `http://film:3601`）。

---

## 🚀 启动与运行

### 核心部署命令
```bash
# 构建并后台运行
docker compose up --build -d
```

### 验证运行状态
```bash
# 查看容器状态
docker compose ps

# 查看实时日志
docker compose logs -f
```

---

## 📁 持久化与存储

为保证数据不丢失，建议在 `docker-compose.yml` 中配置以下挂载点：

```yaml
services:
  film:
    volumes:
      - ./server/uploads:/app/uploads  # 影片海报与上传附件
      - ./server/logs:/app/logs        # 后端运行日志
```

---

## 🔍 故障排查 (Troubleshooting)

### 1. 内存不足（OOM）
Next.js 构建时非常耗内存。如果您的服务器内存少于 2G，可能会构建失败。
**方案**：在本地构建完成后将镜像推送到私有仓库，或者在服务器上临时开启 `Swap`。

### 2. 数据库连接失败
- **Windows/Mac**: `host.docker.internal` 通常有效。
- **Linux**: 如果 `host.docker.internal` 无效，请查看机器内网 IP，或在 `docker-compose.yml` 中为 `film` 服务添加：
  ```yaml
  extra_hosts:
    - "host.docker.internal:host-gateway"
  ```

### 3. 前端无法获取数据
检查 `web` 服务的 `API_URL` 是否正确设置。进入容器执行 `curl http://film:3601/index` 验证后端连通性。

---

## 🔒 生产建议

1. **Nginx 反向代理**: 建议在宿主机使用 Nginx 监听 `80/443` 并转发到 `3000` 端口。
2. **SSL**: 使用 Let's Encrypt 配置证书。
3. **防火墙**: 仅对外开放 `80/443` 端口，将 `3601` 端口对公网关闭。
