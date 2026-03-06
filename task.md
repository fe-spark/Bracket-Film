# 任务列表：影片采集链路改造

- [x] **Phase 0: 方案设计与审批**
    - [x] 理解现有 Master-Slave 架构与风险 (README-FAQ.md)
    - [x] 调研后端代码实现 (service/repository)
    - [x] 编写改造方案 (implementation_plan.md)
    - [x] 获取用户审批

- [x] **Phase 1: 防御性改造 (安全性增强)**
    - [x] 实现 `repository.DemoteExistingMaster` (自动降级旧主)
    - [x] 在 `CollectService` 中实现单主站强制互斥
    - [x] 实现 `IsAnyTaskRunning` 全局采集状态检测
    - [x] 在 `CollectService.UpdateFilmSource` 中增加任务锁拦截

- [x] **Phase 2: 存储层重构 (去重与 ID 归一化)**
    - [x] 创建 `movie_source_mappings` 关联表
    - [x] 修改 `SearchInfo` 与 `MovieDetailInfo` 增加唯一性辅助索引
    - [x] 重构 `repository.BatchSaveOrUpdate`，引入基于内容的去重逻辑
    - [x] 实现 `GlobalMid` 跨表同步与来源映射记录

- [x] **Phase 3: 分类体系优化 (动态名称对齐)**
    - [x] 实现 `repository.ReMapCategoryByName` 逻辑
    - [x] 优化匹配算法，摆脱硬编码同义词映射 (简体归一化+模糊包含+正则)
    - [x] 实现主站分类采集后的全量自动触发机制

- [x] **Phase 4: 验证与文档**
    - [x] 完成代码实现与安全性加固
    - [x] 增加站点升级时的 `movie_playlist` 自动清理逻辑
    - [x] 完成全链路模型字段与逻辑审计
    - [x] 同步方案文档与 FAQ 至项目根目录
    - [x] 整理并发布 Walkthrough 记录成果
