# Bracket-Film Server v2

影视站后端服务，基于 Go + Gin + GORM 构建，提供影视内容采集、管理、检索及对外输出能力。

## 技术栈

| 组件 | 版本 | 用途 |
|---|---|---|
| Go | 1.22 | 运行时 |
| Gin | 1.9 | HTTP 路由框架 |
| GORM + MySQL | 8.x | 持久化存储 |
| go-redis | 9.x | 缓存 & JWT 存储 |
| robfig/cron | 3.x | 定时采集任务 |
| gocolly/colly | 2.x | HTML 爬虫内核 |
| golang-jwt | 5.x | 身份鉴权 |

## 项目结构

```
server-v2/
├── main.go                     # 程序入口，等待 DB 就绪后启动
├── config/config.go            # 全局常量 & 环境变量加载
├── internal/
│   ├── handler/                # HTTP 处理层，仅做参数绑定与响应封装
│   ├── service/                # 业务逻辑层
│   ├── repository/             # 数据访问层（MySQL + Redis）
│   ├── model/                  # 数据模型定义
│   ├── middleware/             # Cors / JWT / XML 中间件
│   ├── router/router.go        # 路由注册
│   └── spider/                 # 爬虫核心 & 定时任务
└── pkg/
    ├── db/                     # MySQL & Redis 连接初始化
    ├── conver/                 # 数据格式转换工具
    ├── param/                  # 请求参数工具
    ├── response/               # 统一响应封装
    └── utils/                  # 通用工具（JWT、Hash、文件、字符串）
```

## 环境变量

启动前必须配置以下环境变量：

| 变量名 | 说明 | 示例 |
|---|---|---|
| `PORT` / `LISTENER_PORT` | 服务监听端口 | `8080` |
| `MYSQL_HOST` | MySQL 主机 | `127.0.0.1` |
| `MYSQL_PORT` | MySQL 端口 | `3306` |
| `MYSQL_USER` | 数据库用户名 | `root` |
| `MYSQL_PASSWORD` | 数据库密码 | `password` |
| `MYSQL_DBNAME` | 数据库名 | `bracket_film` |
| `REDIS_HOST` | Redis 主机 | `127.0.0.1` |
| `REDIS_PORT` | Redis 端口 | `6379` |
| `REDIS_PASSWORD` | Redis 密码（可选） | - |
| `REDIS_DB` | Redis DB 编号（可选，默认 0） | `0` |

## 快速启动

```bash
# 编译
go build -o bracket-film .

# 运行
PORT=8080 \
MYSQL_HOST=127.0.0.1 MYSQL_PORT=3306 MYSQL_USER=root MYSQL_PASSWORD=pass MYSQL_DBNAME=bracket_film \
REDIS_HOST=127.0.0.1 REDIS_PORT=6379 \
./bracket-film
```

首次启动会自动完成建表、初始化管理员账号及默认配置，无需手动执行 SQL。

## API 概览

### 公开接口（无需认证）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/index` | 首页数据（推荐、热门、分类） |
| GET | `/api/navCategory` | 导航分类树 |
| GET | `/api/config/basic` | 站点基础配置（供前端读取） |
| GET | `/api/filmDetail` | 影片详情 |
| GET | `/api/filmPlayInfo` | 播放信息（含多源） |
| GET | `/api/searchFilm` | 关键词搜索 |
| GET | `/api/filmClassify` | 分类列表 |
| GET | `/api/filmClassifySearch` | 标签筛选 |
| GET | `/api/proxy/video` | 视频代理转发 |
| POST | `/api/login` | 登录 |
| GET | `/api/provide/vod` | 对外 VOD 数据输出（苹果CMS 协议） |
| GET | `/api/provide/config` | VOD 配置信息 |

### 管理接口（Bearer Token 认证）

**系统配置** `/api/manage/config`

| 路径 | 说明 |
|---|---|
| `GET /basic` | 获取站点配置 |
| `POST /basic/update` | 更新站点配置 |

**轮播图** `/api/manage/banner`：list / find / add / update / del

**用户管理** `/api/manage/user`：info / list / add / update / del

**采集站管理** `/api/manage/collect`

| 路径 | 说明 |
|---|---|
| `GET /list` | 采集站列表 |
| `POST /add` / `update` | 新增 / 修改 |
| `POST /change` | 启停切换 |
| `POST /test` | API 连通性测试 |
| `GET /collecting/state` | 当前正在采集的站点 |
| `GET /stop` | 中断采集 |
| `GET /record/list` | 失败记录 |
| `GET /record/retry` | 单条重试 |
| `GET /record/retry/all` | 全部重试 |

**定时任务** `/api/manage/cron`：list / find / add / update / change / del

**数据采集** `/api/manage/spider`

| 路径 | 说明 |
|---|---|
| `POST /start` | 手动触发采集 |
| `GET /zero` | 清空并重采 |
| `GET /clear` | 仅清空数据 |
| `GET /update/single` | 单影片刷新 |
| `GET /class/cover` | 同步影片分类 |
| `GET /master/status` | 主站是否已有数据 |

**影视管理** `/api/manage/film`：add / 检索列表 / del / 分类树 CRUD

**文件管理** `/api/manage/file`：上传 / 图片墙 / 删除

## 采集站设计

### 主站 / 从站模型

- **主站（grade=0）**：提供影片基础信息（名称、简介、封面、分类等），写入 `search_infos` 和 `movie_detail_infos`
- **从站（grade=1）**：仅提供播放列表，写入 `movie_playlists`，通过影片名 Hash 与主站数据关联

约束：
- 从站采集前必须存在已启用的主站且主站已有数据（接口 `/master/status` 提供状态查询）
- 定时任务按主站优先顺序执行；手动采集页面在主站未就绪时隐藏从站按钮

### 定时任务

支持两种内置 Cron 规格：
- `0 */20 * * * ?`：每 20 分钟增量采集（`DefaultUpdateTime=3h` 内更新的影片）
- `0 0 4 * * 0`：每周日凌晨 4 点孤儿数据清理

## 缓存策略

所有读缓存以 Redis 优先、MySQL 兜底，按更新频率分两类：

| 类型 | TTL | 失效方式 |
|---|---|---|
| 配置类（站点配置、Banner、分类树） | 24h | write-through（写时直接更新） |
| 影片数据类（列表、详情、热门、搜索、播放源） | 2h | 主站采集完成后 `ClearCache()` 主动清除 |

从站采集完成后仅精准清除 `Cache:Play:*`，避免无效地清理主站列表缓存。

## 📺 TVBox / MacCMS 兼容性

Bracket-Film 提供原生的 TVBox 接口支持，可以用作家庭影院或第三方聚合平台的数据源：

- **一键配置接口**: `http://<server_ip>:<PORT>/api/provide/config`
- **MacCMS 兼容接口**: `http://<server_ip>:<PORT>/api/provide/vod`
  - 自动将 Web 端的详细多维度筛选关联至 TVBox 客户端。
  - 支持多线路整合及完整剧集渲染输出。

## 数据库表说明

| 表名 | 说明 |
|---|---|
| `search_infos` | 影片检索宽表，承载所有列表、搜索、标签过滤查询 |
| `movie_detail_infos` | 影片完整详情（JSON 存储） |
| `movie_playlists` | 从站播放列表（按 source_id + movie_key 索引） |
| `search_tag_items` | 搜索标签持久化（剧情 / 地区 / 语言 / 年份等） |
| `film_sources` | 采集站配置 |
| `crontabs` | 定时任务配置 |
| `category_persistents` | 分类树快照 |
| `virtual_picture_queues` | 待同步封面队列 |
| `site_config_records` | 站点配置 |
| `banners_records` | 轮播图配置 |
| `users` | 用户表（初始 ID 10000） |
| `files` | 上传文件记录 |
| `failure_records` | 采集失败记录 |

## 默认账号

首次启动自动创建管理员账号：

| 字段 | 值 |
|---|---|
| 用户名 | `admin` |
| 密码 | `admin` |

> 请在首次登录后修改默认密码。
