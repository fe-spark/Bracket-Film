package repository

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"server/internal/config"
	"server/internal/infra/db"
	"server/internal/model"

	"gorm.io/gorm"
)

// 简易内存映射：仅用于爬虫入库等批量场景的高频查找，避免过多的数据库 IO
var (
	idToPid = make(map[int64]int64)
	catMu   sync.RWMutex
)

// RefreshCategoryCache 用于重新加载基础映射映射到内存
func RefreshCategoryCache() {
	var all []model.Category
	db.Mdb.Find(&all)

	newPidMap := make(map[int64]int64)

	for _, c := range all {
		item := c
		newPidMap[item.Id] = item.Pid
		SetCategoryNameCache(item.Id, item.Name)
	}

	catMu.Lock()
	idToPid = newPidMap
	catMu.Unlock()
}

// GetRootId 获取分类的顶级根 ID (通过内存递归映射)
func GetRootId(id int64) int64 {
	if id == 0 {
		return 0
	}

	catMu.RLock()
	if len(idToPid) == 0 {
		catMu.RUnlock()
		RefreshCategoryCache()
		catMu.RLock()
	}
	defer catMu.RUnlock()

	curr := id
	// 为防止循环引用死循环，最多查找 5 层 (目前本项目只有 2 层)
	for range [5]int{} {
		p, ok := idToPid[curr]
		if !ok || p == 0 {
			return curr
		}
		curr = p
	}
	return curr
}

// IsRootCategory 判断是否为根分类 (Pid 为 0 的大类)
func IsRootCategory(id int64) bool {
	if id == 0 {
		return false
	}

	catMu.RLock()
	if len(idToPid) == 0 {
		catMu.RUnlock()
		RefreshCategoryCache()
		catMu.RLock()
	}
	defer catMu.RUnlock()

	p, ok := idToPid[id]
	return ok && p == 0
}

// GetParentId 获取父类 ID
func GetParentId(id int64) int64 {
	if id == 0 {
		return 0
	}

	catMu.RLock()
	if len(idToPid) == 0 {
		catMu.RUnlock()
		RefreshCategoryCache()
		catMu.RLock()
	}
	defer catMu.RUnlock()

	return idToPid[id]
}

// GetRootIdBySourcePid 通过采集站大类原始 ID 在本地大类列表中按顺序匹配
// 仅用于子分类尚未落库时的兜底，优先级低于 GetRootId(cid)
// SaveCategoryTree 批量保存并同步分类树（方案B: 100% 结构化映射精简版）
func SaveCategoryTree(sourceId string, tree *model.CategoryTree) error {
	if tree == nil {
		return nil
	}

	err := db.Mdb.Transaction(func(tx *gorm.DB) error {
		// 1. 清理旧映射，准备重建
		tx.Where("source_id = ?", sourceId).Delete(&model.CategoryMapping{})

		// 2. 遍历采集站大类，建立本地映射
		for _, node := range tree.Children {
			// 推断标准大类名（探测模式）
			standardName := GetCategoryBucketRole(node.Name)
			if standardName == model.BigCategoryOther && len(node.Children) > 0 {
				for _, sub := range node.Children {
					if role := GetCategoryBucketRole(sub.Name); role != model.BigCategoryOther {
						standardName = role
						break
					}
				}
			}

			// 获取或创建本地大类
			var localMain model.Category
			tx.Where("pid = 0 AND name = ?", standardName).FirstOrCreate(&localMain, model.Category{Pid: 0, Name: standardName, Show: true})

			// 3. 记录映射：如果来源大类名称与标准大类不一致，将其作为子分类处理，实现“日本动漫” -> “动漫”下的具体标签
			targetId := localMain.Id
			if node.Name != standardName {
				var localSub model.Category
				tx.Where("pid = ? AND name = ?", localMain.Id, node.Name).FirstOrCreate(&localSub, model.Category{Pid: localMain.Id, Name: node.Name, Show: true})
				targetId = localSub.Id
			}
			tx.Create(&model.CategoryMapping{SourceId: sourceId, SourceTypeId: node.Id, CategoryId: targetId})

			// 4. 处理来源子类 (继续挂载到本地标准大类下，平铺结构)
			for _, sub := range node.Children {
				var localSub model.Category
				tx.Where("pid = ? AND name = ?", localMain.Id, sub.Name).FirstOrCreate(&localSub, model.Category{Pid: localMain.Id, Name: sub.Name, Show: true})

				// 记录子类映射 (100% 绑定)
				tx.Create(&model.CategoryMapping{SourceId: sourceId, SourceTypeId: sub.Id, CategoryId: localSub.Id})
			}
		}
		return nil
	})

	// 同步完成后刷新内存缓存，确保采集立即可用
	InitMappingEngine()
	RefreshCategoryCache()
	return err
}

// buildTreeHelper 内部辅助函数：直接从列表构建树形结构内存模型
func buildTreeHelper() model.CategoryTree {
	var allList []model.Category
	db.Mdb.Where("`show` = ?", true).Order("pid ASC, sort DESC, id ASC").Find(&allList)

	nodes := make(map[int64]*model.CategoryTree)
	root := model.CategoryTree{
		Id: 0, Pid: -1, Name: "分类信息", Show: true,
		Children: make([]*model.CategoryTree, 0),
	}

	for _, c := range allList {
		item := c
		node := &model.CategoryTree{
			Id:        item.Id,
			Pid:       item.Pid,
			Name:      item.Name,
			Alias:     item.Alias,
			Show:      item.Show,
			Sort:      item.Sort,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
			Children:  make([]*model.CategoryTree, 0),
		}
		nodes[item.Id] = node

		if item.Pid == 0 {
			root.Children = append(root.Children, node)
		} else if parent, ok := nodes[item.Pid]; ok {
			parent.Children = append(parent.Children, node)
		}
	}

	return root
}

// GetCategoryTree 获取完整分类树副本 (实时查库，不走长期缓存)
func GetCategoryTree() model.CategoryTree {
	return buildTreeHelper()
}

// GetActiveCategoryTree 获取仅包含有影视内容的分类树副本 (实时查库 + Redis 缓存)
func GetActiveCategoryTree() model.CategoryTree {
	// 1. 尝试从 Redis 获取
	if data, err := db.Rdb.Get(db.Cxt, config.ActiveCategoryTreeKey).Result(); err == nil && data != "" {
		var tree model.CategoryTree
		if json.Unmarshal([]byte(data), &tree) == nil {
			return tree
		}
	}

	// 2. 获取活跃的 Pid (MainCategory) 和 Cid (Category)
	var activeCids []int64
	db.Mdb.Table(model.TableSearchInfo).Distinct("cid").Pluck("cid", &activeCids)
	activeCidMap := make(map[int64]bool)
	for _, id := range activeCids {
		activeCidMap[id] = true
	}

	var activePids []int64
	db.Mdb.Table(model.TableSearchInfo).Distinct("pid").Pluck("pid", &activePids)
	activePidMap := make(map[int64]bool)
	for _, id := range activePids {
		activePidMap[id] = true
	}

	// 3. 构建树
	var allList []model.Category
	db.Mdb.Where("`show` = ?", true).Order("pid ASC, sort DESC, id ASC").Find(&allList)

	nodes := make(map[int64]*model.CategoryTree)
	root := model.CategoryTree{
		Id: 0, Pid: -1, Name: "分类信息", Show: true,
		Children: make([]*model.CategoryTree, 0),
	}

	// 第一遍：创建所有节点
	for _, c := range allList {
		node := &model.CategoryTree{
			Id:        c.Id,
			Pid:       c.Pid,
			Name:      c.Name,
			Alias:     c.Alias,
			Show:      c.Show,
			Sort:      c.Sort,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
			Children:  make([]*model.CategoryTree, 0),
		}
		nodes[c.Id] = node
	}

	// 第二遍：处理子类并更新父大类的活跃状态
	for _, c := range allList {
		if activeCidMap[c.Id] {
			if c.Pid == 0 {
				// 本身就是大类，直接标记活跃
				activePidMap[c.Id] = true
			} else if parent, ok := nodes[c.Pid]; ok {
				parent.Children = append(parent.Children, nodes[c.Id])
				activePidMap[c.Pid] = true
			}
		}
	}

	// 第三遍：收集活跃的大类到根节点下
	for _, c := range allList {
		if c.Pid != 0 {
			continue
		}
		node := nodes[c.Id]
		if activePidMap[c.Id] || len(node.Children) > 0 {
			root.Children = append(root.Children, node)
		}
	}

	// 7. 写入 Redis 缓存 (1小时)
	if data, err := json.Marshal(root); err == nil {
		db.Rdb.Set(db.Cxt, config.ActiveCategoryTreeKey, string(data), time.Hour)
	}

	return root
}

// ClearCategoryCache 清除分类相关的所有缓存 (Redis + 内存映射)
func ClearCategoryCache() {
	db.Rdb.Del(db.Cxt, config.ActiveCategoryTreeKey)
	ClearAllSearchTagsCache()
	RefreshCategoryCache()
}

// UpdateCategoryStatus 仅更新分类的显示状态或名称，并清除缓存
func UpdateCategoryStatus(id int64, updates map[string]any) error {
	if err := db.Mdb.Model(&model.Category{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return err
	}
	ClearCategoryCache()
	return nil
}

// ExistsCategoryTree 查询分类信息是否存在
func ExistsCategoryTree() bool {
	var count int64
	db.Mdb.Table(model.TableCategory).Count(&count)
	return count > 0
}

// GetChildrenTree 获取对应主分类下的子分类列表 (实时查库)
func GetChildrenTree(pid int64) []*model.CategoryTree {
	tree := buildTreeHelper()

	if pid == 0 {
		return tree.Children
	}
	for _, c := range tree.Children {
		if c.Id == pid {
			return c.Children
		}
	}
	return nil
}

// InitMainCategories 启动时刷新映射引擎与分类缓存
func InitMainCategories() {
	fmt.Println("[Init] 正在确保标准大类并刷新分类缓存...")

	// 1. 确保标准大类存在 (电影, 电视剧, 动漫, 综艺, 纪录片, 短剧, 其他)
	standards := []string{
		model.BigCategoryMovie,
		model.BigCategoryTV,
		model.BigCategoryAnimation,
		model.BigCategoryVariety,
		model.BigCategoryDocumentary,
		model.BigCategoryShortFilm,
		model.BigCategoryOther,
	}
	// 设置每个大类的默认匹配正则 (Alias)
	aliases := map[string]string{
		model.BigCategoryAnimation:   "动漫,动画,番剧,日漫,国漫,美漫",
		model.BigCategoryVariety:     "综艺,脱口秀,真人秀,选秀",
		model.BigCategoryDocumentary: "纪录片,历史,文化,自然",
		model.BigCategoryShortFilm:   "短剧,爽剧,微电影",
		model.BigCategoryMovie:       "电影,动作,喜剧,爱情,科幻,恐怖,剧情,战争,惊悚",
		model.BigCategoryTV:          "电视剧,国产,美剧,韩剧,日剧,港剧,台剧,泰剧,海外",
		model.BigCategoryOther:       "其他,其它,解说,福利",
	}

	for i, name := range standards {
		// 计算优先级 (越靠前优先级越高)
		priority := len(standards) - i

		// 1. 先查找是否存在该记录
		var cat model.Category
		err := db.Mdb.Model(&model.Category{}).Where("pid = 0 AND name = ?", name).First(&cat).Error

		if err == nil {
			// 2. 存在则直接修改对象字段并使用 Save() 强制同步到数据库 (Save 会更新所有字段)
			cat.Alias = aliases[name]
			cat.Show = true
			cat.Sort = priority
			if saveErr := db.Mdb.Save(&cat).Error; saveErr != nil {
				fmt.Printf("[Error] 更新大类 %s 失败: %v\n", name, saveErr)
			} else {
				// 立即从数据库回读，确保更新真正生效
				var check model.Category
				db.Mdb.First(&check, cat.Id)
				fmt.Printf("[Init] 已对齐标准大类: %s (ID: %d, Sort: %d, DB实际Sort: %d)\n", name, cat.Id, priority, check.Sort)
			}
		} else {
			// 3. 不存在则创建
			db.Mdb.Create(&model.Category{
				Pid:   0,
				Name:  name,
				Alias: aliases[name],
				Show:  true,
				Sort:  priority,
			})
			fmt.Printf("[Init] 已创建标准大类: %s (Sort: %d)\n", name, priority)
		}
	}

	// 2. 刷新映射引擎（加载顶级大类到内存缓存）
	InitMappingEngine()

	// 3. 重建分类内存映射
	RefreshCategoryCache()

	// 4. 清理 Redis 过期缓存
	db.Rdb.Del(db.Cxt, config.ActiveCategoryTreeKey, config.IndexPageCacheKey)
	ClearAllSearchTagsCache()

	fmt.Println("[Init] 缓存刷新与标准大类对齐完成。")
}
