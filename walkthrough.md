# 影片采集链路改造完成总结

我已完成对采集链路的深度改造，解决了数据重复、主站切换风险以及分类树碰撞等核心问题。

## 1. 核心改进点总结

### 🛡️ 安全性增强 (Phase 1)
- **强制单主站**：现在系统在添加或更新站点为 `Master` 时，会自动将现有的旧主站降级为 `Slave`。
- **采集任务锁**：引入了 `IsAnyTaskRunning` 检查。如果有任务在运行，系统将禁止修改站点等级或 URI，防止“边采边删”导致的数据损坏。
- **原子切换**：主站变更时会强制调用 `StopAllTasks`。

### 🆔 存储去重与 ID 归一化 (Phase 2)
- **内容指纹 (ContentKey)**：引入了基于 `dbid` 或 `name` 生成的内容唯一标识。
- **跨源去重**：`BatchSaveOrUpdate` 现在基于 `ContentKey` 进行冲突检测。这意味着主站切换后，同名影片会更新原有记录而不是新增重复项，且会保留该记录在系统中的 `Mid`，确保前端链接不失效。
- **模型扩展**：`SearchInfo` 增加了 `SourceId` 和 `ContentKey` 字段。

### 📂 动态分类对齐 (Phase 3)
- **算法驱动关联**：废弃了硬编码的同义词表，改为采用“标准简体化 + 双向包含匹配 + 核心正则兜底”的智能算法。
- **自动触发**：当主站更新分类树后，系统会自动对比全库 `c_name`，无论采集站如何命名（如“动作片”、“动作电影”、“Action”），只要能逻辑匹配，都会自动对齐到新分类。

### 🛠️ 逻辑加固与 ID 归约 (Extra Refinement)
- **Global ID 同步**：实现了 `SearchInfo` 与 `MovieDetailInfo` 的全自动 ID 归约逻辑。无论主站如何切换、ID 如何变化，系统始终以首个探测到的内容指纹为准（Global ID），确保书签、历史记录的持久有效。
- **级联清理**：重写了删除逻辑，确保手动或自动删除影片时，其对应的详情 JSON 和来源映射关系会被同步物理删除，防止数据库残留冗余数据。
- **自动升级清理**：新增了站点升级时的专属清理逻辑。当一个附属站被提升为主站时，系统会自动删除其在 `movie_playlist` 表中的旧有记录，防止其作为主站运行时产生冗余的“ghost”数据。

## 2. 代码文件变更

- [model/film.go](file:///Users/spark/Documents/activity/Bracket-Film/server/internal/model/film.go): 扩展了 `SearchInfo` 和 `MovieDetailInfo`，新增了 `MovieSourceMapping` 模型。
- [repository/spider_repo.go](file:///Users/spark/Documents/activity/Bracket-Film/server/internal/repository/spider_repo.go): 实现了 `DemoteExistingMaster`。
- [repository/film_repo.go](file:///Users/spark/Documents/activity/Bracket-Film/server/internal/repository/film_repo.go): 重构了 `BatchSaveOrUpdate` 和 `ConvertSearchInfo` 的去重逻辑。
- [repository/category_repo.go](file:///Users/spark/Documents/activity/Bracket-Film/server/internal/repository/category_repo.go): 实现了 `ReMapCategoryByName`。
- [service/collect_service.go](file:///Users/spark/Documents/activity/Bracket-Film/server/internal/service/collect_service.go): 强化了写入和更新时的安全性校验。
- [service/init_service.go](file:///Users/spark/Documents/activity/Bracket-Film/server/internal/service/init_service.go): 更新了数据库自动迁移列表。
- [spider/Spider.go](file:///Users/spark/Documents/activity/Bracket-Film/server/internal/spider/Spider.go): 集成了全量分类重关联的触发时机。

## 3. 下一步建议

由于修改了数据库基表结构，建议你在部署后执行一次全量的主站分类采集，系统会自动完成旧数据的关联修复。
