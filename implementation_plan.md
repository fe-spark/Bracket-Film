# 影片采集链路改造方案 (深度重构版)

## 1. 深度问题分析 (基于代码审计)

### 1.1 `MID` (Crawler ID) 导致的重复问题
- **现状**：`repository/film_repo.go` 中的 `BatchSaveOrUpdate` 使用采集站提供的 `mid` 作为唯一冲突键。
- **后果**：不同站点即便提供同一部电影，其 `mid` 往往不同，导致 `search_infos` 表中出现大量重复记录，前端搜索结果“闹双胞”。

### 1.2 分类树碰撞问题
- **现状**：`repository/category_repo.go` 将整个分类树序列化为 JSON 存储在 `category_persistents` 表中，且读取时仅取 ID 最大的第一条。
- **后果**：多主站并存时，后执行的分类采集会彻底“覆盖”（覆盖式生效）前者的分类结构。由于 `SearchInfo.cid` 与分类 ID 绑定，一旦分类树切换，旧数据的分类索引将全部失效。

### 1.3 `TRUNCATE` 的滥用与风险
- **现状**：`repository.MasterFilmZero` 执行 `TRUNCATE` 操作，这在主站 URI 变更或等级提升时被静默调用。
- **后果**：缺乏事务保护和任务状态校验（虽然 Cron 有校验，但 API/Service 层没有），极易在采集中途误删全库。

---

## 2. 核心技术改造方案

### A. 全局标识符与去重逻辑 (Global Movie ID)
- **引入 Mapping 表**：建立 `movie_source_mappings` 表，存储 `(source_id, source_mid) -> global_id`。
- **基于内容的去重**：保存主站数据前，优先通过 `dbid` (非0时) 匹配，无 `dbid` 时通过 `name` 匹配。
- **元数据合并**：如果已存在该影片，更新详情并多源合并，而不是插入新记录。

### B. 动态分类对齐 (Dynamic Category Alignment)
- **主站决定树**：保留主站切换时全量采集并覆盖分类树的逻辑。
- **基于名称的重关联**：利用 `SearchInfo` 中存有的 `CName` (分类名称)，在新分类树就绪后，执行一次全量刷新，通过名称模糊匹配自动更新 `Cid` 和 `Pid`。
- **智能匹配算法**：内置简体化归一化、双向包含匹配及核心大类正则兜底（如“电影/动作”自动对齐到“影片/动作片”）。

### C. 强制单主站约束与资源锁
- **单主站强制互斥**：在 `Update/SaveFilmSource` 时，若设置新 Master，系统自动将现有 Master 降级为 Slave，确保全局唯一性。
- **Service 语义锁**：在执行数据清理或主站重置前，强制检查 `IsAnyTaskRunning`，若有任务则拦截。
- **原子切换流程**：停止所有任务 -> 降级旧主 -> 升级新主 -> 清理/重置主站元数据。

---

## 3. 落地步骤 (已完成)

1. [x] **稳定性补丁**：实现 `IsAnyTaskRunning` 与 `DemoteExistingMaster` 互斥逻辑。
2. [x] **存储层重构**：引入 `ContentKey` 去重与 `MovieSourceMapping` 全局映射。
3. [x] **ID 归约同步**：实现 `SearchInfo` 与 `MovieDetailInfo` 的 Global ID 强制对齐。
4. [x] **分类对齐**：实现智能算法驱动的 `ReMapCategoryByName`。
5. [x] **自动清理**：实现主站升级时的旧播放列表自动物理删除。

---

## 4. 验证结果

- ✅ **去重验证**：不同站点同名资源已成功合并至同一 `GlobalMid`。
- ✅ **切换验证**：主站切换后，存量附属站资源通过动态分类对齐成功挂载到新分类树。
- ✅ **稳定性验证**：采集中途切换主站被系统正确拦截并报错提示。
