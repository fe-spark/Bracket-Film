# Bracket Film Server

## 🚀 简介

`server` 是 Bracket-Film 项目的后端核心，基于 Go 语言构建。它不仅为前端提供高效、稳定的 RESTful API，还集成了自动化数据采集（爬虫）、数据处理聚合以及针对 TVBox 等第三方客户端的服务端支持。

### 核心特性
- **数据采集**：集成 `go-colly` 爬虫框架，支持从公开影视资源站自动化采集数据。
- **任务调度**：内置定时任务管理，支持自动更新影片库信息。
- **数据聚合**：三级缓存架构（Go Memory / Redis / MySQL），确保高并发下的查询性能。
- **管理后台**：配套完整的管理接口，支持对资源站、采集任务、影视内容的精细化管理。
- **多端集成**：原生支持 MacCMS 10 接口协议，并提供 TVBox/影视仓 聚合配置自动生成接口。

---

## 🛠️ 技术栈

| 类别 | 技术 | 说明 |
| :--- | :--- | :--- |
| **核心框架** | [Gin](https://github.com/gin-gonic/gin) | 高性能 HTTP Web 框架 |
| **数据库 ORM** | [GORM](https://gorm.io/) | 功能强大的 Go 语言 ORM 库 |
| **缓存/存储** | [Redis](https://redis.io/) | 用于高速缓存和临时数据处理 |
| **关系型数据库** | [MySQL](https://www.mysql.com/) | 存储结构化影视元数据和配置信息 |
| **网络爬虫** | [Colly](https://github.com/gocolly/colly) | 灵活、快速且轻量级的爬虫框架 |
| **身份认证** | [JWT](https://github.com/golang-jwt/jwt) | 基于 Token 的无状态身份验证 |

---

## 📂 项目结构

```text
server
├─ config       # 配置信息与静态常量
├─ controller   # API 控制层，处理业务请求
├─ logic        # 核心业务逻辑实现
├─ model        # 数据模型 (Schema) 与数据库交互 (DAO)
├─ plugin       # 插件与工具集
│  ├─ db        # 数据库初始化 (MySQL/Redis)
│  ├─ spider    # 数据采集核心逻辑与定时任务
│  └─ middleware # Gin 拦截器 (CORS, Auth)
├─ router       # 路由定义 (前端 API / 管理后台 API / 外部接口)
├─ main.go      # 程序启动入口
└─ go.mod       # 依赖管理
```

---

## 🚀 快速开始

### 1. 环境准备
- Go 1.22+
- MySQL 5.7+ / 8.0+
- Redis (可选但强烈建议)

### 2. 配置数据库
修改 `/server/plugin/db` 目录下的 `mysql.go` 和 `redis.go` 中的连接信息（或通过环境变量配置）。

### 3. 本地运行
```bash
go mod tidy
go run main.go
```

---

## 📺 外部集成 (TVBox / 影视仓)

本项目支持直接作为数据源对接 TVBox 客户端：

- **CMS 接口**：`http://[YOUR_IP]:8088/provide/vod/`
- **自动聚合配置**：`http://[YOUR_IP]:8088/provide/config` (直接在 TVBox 配置地址中填入即可)

---

## 📄 许可证

本项目遵循 [MIT License](../LICENSE)。
