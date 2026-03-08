# ⚙️ Bracket-Film Server (Go Backend)

本目录包含 **Bracket-Film** 的核心服务端实现。后端基于 **Go + Gin + GORM** 架构，提供全自动影视采集、内容指纹去重、多维联动检索及全平台标准接口支持。

---

## 🛠️ 技术选型

| 组件 | 版本 | 核心用途 |
| :--- | :--- | :--- |
| **Go** | 1.24+ | 强类型、高性能运行时 |
| **Gin** | 1.10+ | 轻量级 HTTP 路由框架 |
| **GORM** | 1.25+ | 关系型数据库 ORM 框架 |
| **Redis** | 7.0+ | 多维联动筛选的分页与结果缓存 |
| **robfig/cron** | 3.0+ | 分层自动采集任务调度 |
| **gocolly** | 2.1+ | 灵活的 HTML/JSON 采集引擎 |

---

## 🏗️ 内部架构 `server/`

```text
server/
├── cmd/server/             # 编译入口 (main.go)
├── internal/
│   ├── config/             # 强类型环境变量注入与常量配置
│   ├── handler/            # 业务层 HTTP 控制器 (Request Parsing)
│   ├── service/            # 核心业务逻辑 (Transaction Management)
│   ├── repository/         # 数据持久层 (SQL Builder & Redis Wrapper)
│   ├── model/              # 数据库 Schema 与 API DTO 定义
│   ├── spider/             # 采集站爬虫逻辑与解析器
│   ├── middleware/         # JWT 鉴权、CORS 与分布式限流
│   ├── infra/db/           # 基础设施初始化 (MySQL/Redis)
│   ├── router/             # 树形化路由注册
│   └── utils/              # 加密、脱敏与辅助工具
```

---

## 🚀 部署与初始化

### 环境依赖
1. MySQL 8.0+
2. Redis 7.0+
3. Go 1.24+ (用于编译)

### 快速启动
```bash
cd server
go mod download
go run ./cmd/server
```

**首次启动特性：**
- **Auto Migrations**：自动根据 `internal/model` 同步数据库表结构。
- **Seed Data**：自动初始化默认管理员 (`admin`/`admin`) 及预备采集源。
- **Cron Setup**：自动注册全量采集与孤儿数据清理任务。

---

## 📡 API 接口清单

### 1. 客户端/公共接口 (Public APIs)
| 路径 | 方法 | 功能说明 |
| :--- | :--- | :--- |
| `/api/index` | GET | 获取首页配置、分类树与推荐列表 |
| `/api/searchFilm` | GET | 关键字搜索 |
| `/api/filmDetail` | GET | 影片详情与多站播放列表聚合 |
| `/api/filmClassifySearch` | GET | **核心接口**：支持多维联动筛选 |
| `/api/navCategory` | GET | 获取顶部导航分类 |

### 2. 标准开放接口 (Provision)
| 路径 | 方法 | 功能说明 |
| :--- | :--- | :--- |
| `/api/provide/vod` | GET | 兼容 MacCMS/TVBox 数据标准 |
| `/api/provide/config` | GET | 获取 TVBox 规格化配置 |

### 3. 后台管理接口 (Admin - Auth Required)
- `/api/manage/collect/*`：采集源生命周期与等级管理 (Master/Slave)。
- `/api/manage/spider/*`：手动触发主站全量/分类同步。
- `/api/manage/cron/*`：动态定时任务控制。
- `/api/manage/film/*`：元数据编辑与分类调整。

---

## 🛡️ 主从等级 (Grade) 模型

- **Master (Grade 0)**：元数据权威源。负责构建影片的标题、演职员、海报及剧情介绍。
- **Slave (Grade 1)**：内容补充源。负责提供多个不同线路的播放列表，并通过指纹自动挂载至 Master 骨架。

---

## 📜 更多文档
- [主从一致性 FAQ](../README-FAQ.md)
- [Docker 部署说明](../README-Docker.md)
- [前端 web 指南](../web/README.md)
