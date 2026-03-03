# Bracket-Film 需求文档

> 最后更新：2026-03-03

---

## 目录

1. [存储层迁移：Redis → MySQL](#1-存储层迁移redis--mysql)
2. [采集模块](#2-采集模块)
3. [数据一致性](#3-数据一致性)
4. [播放页](#4-播放页)
5. [管理后台](#5-管理后台)
6. [代码质量](#6-代码质量)

---

## 1. 存储层迁移：Redis → MySQL

### 背景
原始实现将大量业务数据（采集源、定时任务、网站配置、图片队列、分类树等）存储在 Redis 中，导致重启后数据丢失、无法持久化。

### 需求

| 模块 | 旧存储 | 新存储 | 说明 |
|------|--------|--------|------|
| 采集源列表 `FilmSource` | Redis ZSet | MySQL `film_sources` | `grade` 字段排序改为 SQL `ORDER BY grade` |
| 定时任务 `FilmCollectTask` | Redis Hash | MySQL `crontab_records` | `Cid`（cron.EntryID）为运行时字段，不持久化 |
| 网站基本配置 `BasicConfig` | Redis String | MySQL `site_config_records` | Redis 保留 30 min 短期缓存 |
| 轮播图 `Banners` | Redis String | MySQL `banners_records` | Redis 保留 30 min 短期缓存 |
| 分类树 `CategoryTree` | Redis String | MySQL `category_persistents` | Redis 保留 10 min 短期缓存 |
| 待同步图片队列 `VirtualPicture` | Redis ZSet | MySQL `virtual_picture_queues` | `mid` 唯一索引防重复 |
| 原始采集详情 `FilmDetail` | Redis String | **废弃**，不再持久化原始数据 | 采集后直接转换写入目标表 |

### 缓存策略
- Redis 仅作**短期读缓存**，不再承担持久化职责
- 程序启动时调用 `RedisOnlyFlush()` 清空 Redis，确保所有数据从 MySQL 加载，避免启动时读到脏缓存
- `CategoryTreeKey` TTL：10 分钟；`SiteConfigBasic` TTL：30 分钟

### nil 安全
所有从 MySQL/Redis 返回列表的函数，当结果为空时统一返回 `make([]T, 0)` 而非 `nil`，避免前端收到 JSON `null` 报错。

---

## 2. 采集模块

### 2.1 HTTP 反检测

**问题：** 部分资源站会对爬虫 UA 和请求头进行识别拦截（如 LZ 站点因 `Sec-Fetch-Site: same-origin` 返回 400，Brotli 压缩导致解码失败）。

**需求：**
- 维护一个包含 10 个现代浏览器 UA 的池（Chrome / Firefox / Edge），每次请求随机选取
- 设置标准浏览器请求头：`Accept`、`Accept-Language`、`Accept-Encoding: gzip, deflate`（不声明 `br`，Go 标准库无法解压 Brotli）、`Connection`、`Cache-Control`
- Referer 仅在目标 URL 与上一个 URL 同主机时注入，跨站不注入
- **移除** 所有 `Sec-Fetch-*` 系列请求头

### 2.2 播放列表解析

**原始格式：**
```
源A播放列表 $$$ 源B播放列表
       ↓ 按 VodPlayNote 分割
第1集$http://a.m3u8#第2集$http://b.m3u8#...
       ↓ 按 # 分割每集
集名$链接
```

**需求：**
- 使用 `strings.Cut(p, "$")` 替代 `strings.Split` + `Contains`，更安全地处理 URL 中含 `$` 的情况
- 过滤空字符串条目（末尾多余的 `#` 会产生空串）
- `$` 后面为空（即 `Link` 为空）时跳过该条目，不入库
- 没有 `$` 的裸 URL 为单集内容，`Episode` 按实际位置自动编号为「第 1 集」「第 2 集」……
- 只保留包含 `.m3u8` 或 `.mp4` 的播放源（`GenFilmPlayList`），过滤云播等其他格式

### 2.3 采集源状态变更

**需求：** 管理后台变更采集源的 `State`（启用/停用）或 `SyncPictures` 时，保留原有的 `Interval`（采集间隔）字段不被覆盖为零值。

---

## 3. 数据一致性

### 3.1 重复采集覆盖

**需求：** 对同一影片（相同 `mid`）重复采集时，执行 upsert（ON CONFLICT DO UPDATE），更新以下字段：

```
cid, pid, name, sub_title, c_name, class_tag,
area, language, year, initial, score,
update_stamp, hits, state, remarks, release_stamp,
picture, actor, director, blurb, updated_at, deleted_at
```

- `deleted_at` 列入更新字段：若记录曾被管理员软删除，重新采集时自动恢复（置 NULL）
- `mid` 具备数据库唯一索引（`uniqueIndex:idx_mid`）保证不产生重复行

### 3.2 数据库索引初始化

**问题：** `CreateSearchTable` 函数原先包含 `if !ExistSearchTable()` 的守卫，导致表已存在时 `AutoMigrate` 不执行，唯一索引无法创建。

**需求：**
- 始终执行 `AutoMigrate`，确保索引与结构体定义同步
- 表已存在时，`AutoMigrate` 前先去重：删除 `mid` 重复的记录，保留 `id` 最大的一条（最新数据）

### 3.3 附属站点多轨播放列表

**问题：** 附属站点（Slave）的 `SaveSitePlayList` 仅存储 `PlayList[0]`（第一条轨迹），丢失多轨数据；`GetMovieDetailByDBID` 将 `[]MovieUrlInfo` JSON 反序列化为 `MoviePlaySource{}` 导致类型不匹配，结果始终为空。

**需求：**
- `SaveSitePlayList`：存储完整的 `[][]MovieUrlInfo`（所有轨迹）到 `movie_playlists`
- `GetMovieDetailByDBID`：按 `[][]MovieUrlInfo` 反序列化，每个轨迹对应一个 `MoviePlaySource` 条目

### 3.4 数据清空

**需求：** "清空数据"操作应物理删除以下表的全部数据（`TRUNCATE`）：

- `movie_details`
- `search_infos`
- `movie_playlists`
- `category_persistents`
- `virtual_picture_queues`
- 同时清空 Redis 相关缓存键

**不清空：** `film_sources`（采集源）、`crontab_records`（定时任务）、`site_config_records`（站点设置）、`banners_records`（轮播图）、`file_infos`（本地图库）

---

## 4. 播放页

### 4.1 播放器 autoPlayback

**问题：** 进入任何视频都会弹出「是否从上次位置继续？」提示，包括从未播放过的片源。

**根本原因：** 播放器设置了固定 `id: "bracket-player"`，导致所有集数共用同一个 localStorage 键，串了播放进度。

**需求：**
- 移除固定 `id`，让 Artplayer 按 URL 生成唯一键
- 每次播放器初始化前调用 `localStorage.removeItem("bracket-player")` 清理旧残留键
- 当业务层传入 `initialTime > 0`（来自历史记录）时，设置 `autoPlayback: false`，由业务层控制跳转，避免双重弹窗；`initialTime <= 0` 时保持 `autoPlayback: true`

### 4.2 播放页数据异常（播放器消失）

**问题：** 重新采集后，采集源 ID 发生变化，前端历史记录中的 `playFrom`（旧 ID）无法匹配新的 `detail.list`，导致 `current` 返回零值，播放器无内容展示。

**需求（`FilmPlayInfo` 接口）：**

1. **过滤空 Link**：遍历所有播放源的 `LinkList`，移除 `ep.Link == ""` 的条目，避免前端渲染不可播放的集数按钮
2. **校验 playFrom 有效性**：检查 `playFrom` 是否存在于 `detail.List` 中（而非仅判断字符串长度），不存在则回退到 `detail.List[0].Id`
3. **episode 越界保护**：`episode >= len(v.LinkList)` 时回退到索引 0，避免 panic

---

## 5. 管理后台

### 5.1 影片删除行为

| 操作 | 方式 | 说明 |
|------|------|------|
| 管理员删除单片 | 软删除（设 `deleted_at`） | 重新采集时 upsert 自动恢复，不产生重复行 |
| 分类隐藏 | 软删除该分类所有影片 | 可通过「分类显示」操作一键恢复 |
| 分类恢复显示 | 将 `deleted_at` 置 NULL | |
| 清空数据 | `TRUNCATE`（物理删除） | 全量重采场景下使用 |

### 5.2 错误信息透出

**需求：** 采集源接口测试失败时，直接将具体错误信息返回给前端（原来只返回固定文案），便于用户判断问题原因：

```go
system.Failed(fmt.Sprint("资源接口测试失败: ", err.Error()), c)
```

---

## 6. 代码质量

### 6.1 日志规范

- 所有 `log.Println(err)` 替换为 `log.Printf("FunctionName Error: %v", err)`，包含函数名便于定位
- 移除调试残留日志（如 `Collect.go` 中的 `xml.Marshal` + `log.Println`）
- 移除无意义的成功日志（如 `"Spider Task Exercise Success"`）
- 错误已通过 `return error` 传递时，不再重复打印日志

### 6.2 冗余代码清理

- 移除注释掉的分页统计代码块（`GetHotMovieByPid` / `GetHotMovieByCid`）
- 修正错误注释（`ShieldFilmSearch` / `RecoverFilmSearch` 注释与实际行为不符）
- `strings.Split` + `Contains` + index 访问模式 → 改用 `strings.Cut`，更安全简洁

### 6.3 nil 安全

- 所有返回切片类型的函数，空结果统一返回 `make([]T, 0)` 而非 `nil`
- `tree.Children != nil` 判空保护，避免空分类树时 panic
