# Bracket-Film

**Bracket-Film** 是一款高性能、现代化的全栈影视聚合系统。它基于 **Go (Gin)** 后端与 **Next.js 16 (React 19)** 前端构建，旨在提供极速的 Web 观影体验，并支持高效的自动化采集与多端适配。

> [!WARNING]
> **免责声明**：本项目仅供学习及技术交流使用，作者不存储、不上传任何影视资源。使用本项目产生的任何后果由使用者自行承担，详情请阅读 [DISCLAIMER.md](./DISCLAIMER.md)。

### 核心特性

- 🎬 **全栈架构**：基于 **Go (Gin)** 后端与 **Next.js 16 (React 19)** 前端构建，响应极速，SEO 友好。
- ⚙️ **自动化采集**：内置灵活的爬虫引擎与定时任务系统，支持从多种 MacCMS 资源站自动同步影片数据。
- 📱 **多端适配**：完美兼容 **TVBox / 影视仓** 等各类播放器收录，提供标准 MacCMS 兼容 API 及一键配置功能。
- 🛠️ **管理后台**：直观的控制面板，支持采集源维护、影片信息编辑、轮播图管理及系统参数热配置。
- 🚀 **极致播放**：深度集成的多线路播放引擎，支持自动切换源、多分辨率适配及 HLS 播放优化。
- 📦 **云原生部署**：全面支持 Docker 与 Docker Compose，一行命令即可完成整站部署。

本项目基于 [ProudMuBai/GoFilm](https://github.com/ProudMuBai/GoFilm) 进行二次开发，在原有架构基础上进行了 Next.js 16 深度重构、TVBox 适配优化及 UI 全面升级。

---

## 🏗️ 架构概览

- `web`：Next.js 前端应用（包含前台用户界面与后台管理系统）
- `server`：Go API 服务，处理数据持久化、鉴权、采集及定时任务
- `docker-compose.yml`：基于 Docker 的全栈一键部署配置
- `README-Docker.md`：详细的容器化部署说明方案

---

## 技术栈

### 前端（`web`）

- Next.js 16 (App Router)
- React 19
- Ant Design 6
- TypeScript
- Artplayer & hls.js

### 服务端（`server`）

- Gin Web Framework
- GORM (MySQL)
- Redis (Caching)
- gocolly (Scraping engine)
- robfig/cron (Task scheduling)

---

## 快速开始（本地开发）

### 1) 启动后端

进入 `server` 目录，配置环境变量（可参考根目录 `.env.example`），然后执行：

```bash
go run main.go
```

**关键环境变量：**

- `PORT`: 服务监听端口（默认 `3601`）
- `MYSQL_HOST`, `MYSQL_USER`, `MYSQL_PASSWORD`, `MYSQL_DBNAME`
- `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD`

### 2) 启动前端

进入 `web` 目录：

```bash
npm install
npm run dev
```

默认访问地址：
- **前台**: `http://localhost:3000`
- **后台**: `http://localhost:3000/manage`

---

## 📦 Docker 部署

项目支持使用 Docker Compose 构建镜像并一键启动。

```bash
docker compose up --build -d
```

启动后，Web 默认运行在 `3000` 端口，API 默认运行在 `3601` 端口。详细说明请参考 [README-Docker.md](./README-Docker.md)。

---

## 🛠️ 初始化与管理

### 默认管理员账户
系统启动时会自动初始化管理员账号：
- **账号**: `admin`
- **密码**: `admin`

> [!IMPORTANT]
> 强烈建议在首次登录后进入系统设置修改默认密码。

---

## 📺 TVBox / MacCMS 兼容性

Bracket-Film 对第三方播放器提供了深度支持：

- **一键配置接口**: `http://<server_ip>:3601/api/provide/config`
- **MacCMS 兼容接口**: `http://<server_ip>:3601/api/provide/vod`
    - 支持全量分类、搜索及高级筛选（地区、年份、语言、排序）。
    - 筛选数据与 Web 端保持实时一致。

---

## 🤝 作者与联系方式

- **项目作者**: spark
- **联系邮箱**: [spark.xiaoyu@qq.com](mailto:spark.xiaoyu@qq.com)
- **GitHub**: [Bracket-Film](https://github.com/fe-spark/Bracket-Film)

---

## 📚 常见问题

- 常见问题与排障指南见 [README-FAQ.md](./README-FAQ.md)

---

## 开源协议

本项目基于 **MIT License** 开源。
