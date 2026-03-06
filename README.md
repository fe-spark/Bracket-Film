# Bracket-Film

**Bracket-Film** 是一个前后端分离的影视聚合系统，当前仓库由 Go 后端（`server`）与 Next.js 前端（`web`）组成。

> [!WARNING]
> **免责声明**：本项目仅供学习与技术交流使用，作者不存储、不上传任何影视资源。请在使用前阅读 [DISCLAIMER.md](./DISCLAIMER.md)。

## 核心特性

- Go + Next.js 全栈架构，支持 SSR 与管理后台。
- **采集链路去重**：基于 `ContentKey` (内容指纹) 的全局归一化存储，杜绝重复数据。
- **主从动态对齐**：强制单主站隔离，支持主站无感切换与分类名称自动智能对齐。
- **Global ID 映射**：全站 ID 归约，确保主站切换后播放历史与书签的持久稳定性。
- 视频播放器支持、定时任务、失败重试与孤儿数据清理。

## 目录结构

- `server`：Go API 服务（Gin + GORM + Redis + Cron）
- `web`：Next.js 16 前端（前台 + 管理后台）
- `docker-compose.yml`：容器编排
- `README-Docker.md`：Docker 部署说明
- `README-FAQ.md`：常见问题与排障

## 技术栈

### 前端（`web`）

- Next.js 16
- React 19
- Ant Design 6
- TypeScript

### 后端（`server`）

- Go 1.24
- Gin
- GORM + MySQL
- go-redis
- robfig/cron
- gocolly

## 本地开发

### 1) 启动后端

进入 `server` 目录，配置环境变量后启动：

```bash
cd server
go run ./cmd/server
```

关键环境变量：

- `PORT` 或 `LISTENER_PORT`（服务端口）
- `MYSQL_HOST` `MYSQL_PORT` `MYSQL_USER` `MYSQL_PASSWORD` `MYSQL_DBNAME`
- `REDIS_HOST` `REDIS_PORT` `REDIS_PASSWORD` `REDIS_DB`

### 2) 启动前端

```bash
cd web
npm install
npm run dev
```

默认访问地址：

- 前台：`http://localhost:3000`
- 后台：`http://localhost:3000/manage`

## Docker 部署

```bash
docker compose up --build -d
```

默认端口：Web `3000`，API `3601`。详细说明见 [README-Docker.md](./README-Docker.md)。

## 默认管理员

首次启动会初始化管理员账号：

- 用户名：`admin`
- 密码：`admin`

> [!IMPORTANT]
> 请首次登录后立即修改默认密码。

## TVBox / MacCMS

- 配置接口：`http://<server_ip>:3601/api/provide/config`
- 数据接口：`http://<server_ip>:3601/api/provide/vod`

## 文档索引

- Docker 说明：[README-Docker.md](./README-Docker.md)
- 常见问题：[README-FAQ.md](./README-FAQ.md)
- 后端文档：[server/README.md](./server/README.md)
- 前端文档：[web/README.md](./web/README.md)

## License

MIT License
