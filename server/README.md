# Bracket-Film Server

后端服务基于 Go + Gin + GORM，提供影视聚合、采集、管理与对外提供接口能力。

## 技术栈

| 组件 | 版本（当前） | 用途 |
|---|---|---|
| Go | 1.24.x | 运行时 |
| Gin | 1.9.x | HTTP 路由框架 |
| GORM + MySQL | 1.25.x / 8.x | 持久化存储 |
| go-redis | 9.x | 缓存 |
| robfig/cron | 3.x | 定时任务 |
| gocolly/colly | 2.x | 采集引擎 |
| golang-jwt | 5.x | 登录鉴权 |

## 项目结构

```text
server/
├── cmd/server/main.go      # 程序入口
└── internal/
    ├── config              # 环境变量与常量
    ├── handler             # HTTP handler
    ├── infra/db            # MySQL / Redis 初始化
    ├── middleware          # Cors / JWT / XML
    ├── model               # 数据模型
    ├── repository          # 数据访问层
    ├── router              # 路由注册
    ├── service             # 业务逻辑
    ├── spider              # 采集与定时任务
    └── utils               # 通用工具
```

## 环境变量

| 变量名 | 必填 | 说明 | 示例 |
|---|---|---|---|
| `PORT` / `LISTENER_PORT` | 是 | 服务监听端口 | `3601` |
| `MYSQL_HOST` | 是 | MySQL 主机 | `127.0.0.1` |
| `MYSQL_PORT` | 是 | MySQL 端口 | `3306` |
| `MYSQL_USER` | 是 | 数据库用户 | `root` |
| `MYSQL_PASSWORD` | 否 | 数据库密码 | `password` |
| `MYSQL_DBNAME` | 是 | 数据库名 | `FilmSite` |
| `REDIS_HOST` | 是 | Redis 主机 | `127.0.0.1` |
| `REDIS_PORT` | 是 | Redis 端口 | `6379` |
| `REDIS_PASSWORD` | 否 | Redis 密码 | - |
| `REDIS_DB` | 否 | Redis DB（默认 0） | `0` |

## 本地启动

```bash
cd server
go run ./cmd/server
```

首次启动会自动执行：

- 数据表初始化（新库）
- 默认管理员创建（`admin` / `admin`）
- 基础系统配置、默认采集源与默认定时任务初始化

## API 概览

### 公共接口

- `GET /api/index`
- `GET /api/navCategory`
- `GET /api/config/basic`
- `GET /api/filmDetail`
- `GET /api/filmPlayInfo`
- `GET /api/searchFilm`
- `GET /api/filmClassify`
- `GET /api/filmClassifySearch`
- `GET /api/proxy/video`
- `POST /api/login`
- `GET /api/logout`
- `GET /api/provide/vod`
- `GET /api/provide/config`

### 管理接口（需 Bearer Token）

- `/api/manage/config`：站点配置
- `/api/manage/banner`：轮播图管理
- `/api/manage/user`：用户管理
- `/api/manage/collect`：采集站、采集中断、失败记录与重试
- `/api/manage/cron`：定时任务管理
- `/api/manage/spider`：手动采集、清空、单片刷新、分类同步
- `/api/manage/film`：影片与分类管理
- `/api/manage/file`：文件上传与列表

## 采集与任务模型

- 主站（`grade=0`）：写入基础影片信息
- 附属站（`grade=1`）：写入播放列表并关联主站影片
- 自动采集（默认任务 `Model=0`）按“主站阶段 → 附属站阶段”执行

默认任务（初始化后默认关闭）：

- `0 */20 * * * ?`：每 20 分钟自动采集
- `0 0 4 * * 0`：每周日 4 点失败记录恢复
- `0 0 0 * * *`：每天 0 点孤儿播放列表清理

## 对外接口（TVBox / MacCMS）

- `GET /api/provide/config`
- `GET /api/provide/vod`

## 常见问题

见 [../README-FAQ.md](../README-FAQ.md)
