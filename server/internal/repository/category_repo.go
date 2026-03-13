package repository

import (
	"encoding/json"
	"fmt"
	"server/internal/config"
	"server/internal/infra/db"
	"server/internal/model"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 简易内存映射：仅用于爬虫入库等批量场景的高频查找，避免过多的数据库 IO
var nameToId = make(map[string]int64)
var idToPid = make(map[int64]int64)

// RefreshCategoryCache 用于重新加载基础映射映射到内存
func RefreshCategoryCache() {
	var all []model.Category
	db.Mdb.Find(&all)

	newNameMap := make(map[string]int64)
	newPidMap := make(map[int64]int64)

	for _, c := range all {
		item := c
		newPidMap[item.Id] = item.Pid
		// 子类具有更高的查找优先级 (Pid > 0)
		if item.Pid > 0 {
			newNameMap[item.Name] = item.Id
		} else if _, ok := newNameMap[item.Name]; !ok {
			// 如果该名称还没存入子类 ID，则存入大类 ID (Pid = 0)
			newNameMap[item.Name] = item.Id
		}
	}
	nameToId = newNameMap
	idToPid = newPidMap
}

// GetCidByName 根据分类名称查找对应的类 ID (从内存简单映射获取)
func GetCidByName(name string) int64 {
	if len(nameToId) == 0 {
		RefreshCategoryCache()
	}
	return nameToId[name]
}

// GetRootId 获取分类的顶级根 ID (通过内存递归映射)
func GetRootId(id int64) int64 {
	if id == 0 {
		return 0
	}
	if len(idToPid) == 0 {
		RefreshCategoryCache()
	}

	curr := id
	// 为防止循环引用死循环，最多查找 5 层 (目前本项目只有 2 层)
	for range 5 {
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
	if len(idToPid) == 0 {
		RefreshCategoryCache()
	}
	p, ok := idToPid[id]
	return ok && p == 0
}

// SaveCategoryTree 批量保存并同步分类树 (内存指针对齐 + 二阶段批量入库)
// 这种方式将采集站分类与本地数据库分类通过 Name 进行对齐，确保 ID 永久稳定，不随采集站变化。
func SaveCategoryTree(tree *model.CategoryTree) error {
	if tree == nil {
		return nil
	}

	err := db.Mdb.Transaction(func(tx *gorm.DB) error {
		// 1. 加载现有全部分类建立唯一性缓存 (Key: Pid_Name -> ID)
		var total []model.Category
		tx.Find(&total)
		cache := make(map[string]int64)
		for _, c := range total {
			cache[fmt.Sprintf("%d_%s", c.Pid, c.Name)] = c.Id
		}

		// 2. 指针映射表：节点指针 -> 真实维护的 Pid (MainCategory.Id)
		pointerToMainId := make(map[*model.CategoryTree]int64)

		// 3. 第一阶段：处理采集站的第一层 (通常是电影、电视剧等)
		// 我们不直接存这些分类到 Category 表，而是将其作为映射依据
		for _, node := range tree.Children {
			// 识别它是我们的哪个标准大类
			mainId := GetMainCategoryIdByName(node.Name, node.Id)

			// **动态发现逻辑**：如果映射引擎没能识别，则视其为新发现的大类
			if mainId == 0 {
				var newMain model.Category
				// 尝试通过名称查找或创建
				if err := tx.Where("pid = 0 AND name = ?", node.Name).FirstOrCreate(&newMain, model.Category{
					Pid:   0,
					Name:  node.Name,
					Alias: node.Name,
					Show:  true,
					Sort:  0,
				}).Error; err != nil {
					return err
				}
				mainId = newMain.Id
			}
			pointerToMainId[node] = mainId
		}

		// 4. 第二阶段：处理采集站的第二层 (子类)
		var newSubs []*model.Category
		for _, rootNode := range tree.Children {
			realPid := pointerToMainId[rootNode]
			for _, subNode := range rootNode.Children {
				// 剥离名称中的属性 (如 "国产剧" -> "剧集")
				cleanSubName, _ := MapAttributesFromTypeName(subNode.Name)

				key := fmt.Sprintf("%d_%s", realPid, cleanSubName)
				if id, ok := cache[key]; ok {
					subNode.Id = id
					subNode.Pid = realPid
				} else {
					subC := &model.Category{Name: cleanSubName, Pid: realPid, Show: true}
					newSubs = append(newSubs, subC)
					subNode.Category = subC
				}
			}
		}

		if len(newSubs) > 0 {
			// 对于新出现的类型，执行去重入库
			for _, ns := range newSubs {
				// 再次检查防止本次批量内重复 (虽然 tree 一般不会有重复)
				if err := tx.Where("pid = ? AND name = ?", ns.Pid, ns.Name).FirstOrCreate(ns).Error; err != nil {
					return err
				}
			}
			// 回填入库后的 ID
			for _, rootNode := range tree.Children {
				for _, subNode := range rootNode.Children {
					if subNode.Category != nil && subNode.Id == 0 {
						subNode.Id = subNode.Category.Id
						subNode.Pid = subNode.Category.Pid
					}
				}
			}
		}

		return nil
	})
	// 事务提交成功后，同步更新内存临时映射
	RefreshCategoryCache()
	return err
}

// buildTreeHelper 内部辅助函数：直接从列表构建树形结构内存模型
func buildTreeHelper() model.CategoryTree {
	var allList []model.Category
	db.Mdb.Where("`show` = ?", true).Order("pid ASC, sort DESC, id ASC").Find(&allList)

	nodes := make(map[int64]*model.CategoryTree)
	root := model.CategoryTree{
		Category: &model.Category{Id: 0, Pid: -1, Name: "分类信息", Show: true},
		Children: make([]*model.CategoryTree, 0),
	}

	for _, c := range allList {
		item := c
		node := &model.CategoryTree{
			Category: &item,
			Children: make([]*model.CategoryTree, 0),
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
	db.Mdb.Raw("SELECT DISTINCT cid FROM search_info").Pluck("cid", &activeCids)
	activeCidMap := make(map[int64]bool)
	for _, id := range activeCids {
		activeCidMap[id] = true
	}

	var activePids []int64
	db.Mdb.Raw("SELECT DISTINCT pid FROM search_info").Pluck("pid", &activePids)
	activePidMap := make(map[int64]bool)
	for _, id := range activePids {
		activePidMap[id] = true
	}

	// 3. 构建树
	var allList []model.Category
	db.Mdb.Where("`show` = ?", true).Order("pid ASC, sort DESC, id ASC").Find(&allList)

	nodes := make(map[int64]*model.CategoryTree)
	root := model.CategoryTree{
		Category: &model.Category{Id: 0, Pid: -1, Name: "分类信息", Show: true},
		Children: make([]*model.CategoryTree, 0),
	}

	// 第一遍：创建所有节点
	for _, c := range allList {
		item := c
		node := &model.CategoryTree{
			Category: &item,
			Children: make([]*model.CategoryTree, 0),
		}
		nodes[item.Id] = node
	}

	// 第二遍：处理子类并更新父大类的活跃状态
	for _, c := range allList {
		if c.Pid == 0 {
			continue
		}
		if activeCidMap[c.Id] {
			if parent, ok := nodes[c.Pid]; ok {
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

// GetParentId 获取指定分类的父级 ID (从高性能映射获取)
func GetParentId(id int64) int64 {
	if len(idToPid) == 0 {
		RefreshCategoryCache()
	}
	return idToPid[id]
}

// InitMainCategories 初始化标准大类
func InitMainCategories() {
	categories := []model.Category{
		{Pid: 0, Name: "电影", Alias: "电影,影片,电影片"},
		{Pid: 0, Name: "电视剧", Alias: "电视剧,连续剧,系列剧,剧集"},
		{Pid: 0, Name: "动漫", Alias: "动漫,动画,动漫片,动画片"},
		{Pid: 0, Name: "综艺", Alias: "综艺,综艺片,真人秀,脱口秀"},
		{Pid: 0, Name: "纪录片", Alias: "纪录片,记录片"},
		{Pid: 0, Name: "短剧", Alias: "短剧,短剧大全,爽剧"},
	}

	for _, c := range categories {
		// 1. 确保大类存在
		db.Mdb.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "name"}},
			DoNothing:    true,
		}).Create(&c)

		// 2. 如果是新创建或已存在，尝试为其初始化默认排序标签
		// 先查询大类真实 ID (针对 OnConflict 时 ID 不确定的情况)
		var realC model.Category
		if err := db.Mdb.Where("pid = 0 AND name = ?", c.Name).First(&realC).Error; err == nil {
			defaultSorts := []model.SearchTagItem{
				{Pid: realC.Id, TagType: "Sort", Name: "时间", Value: "update_stamp", Score: 10},
				{Pid: realC.Id, TagType: "Sort", Name: "人气", Value: "hits", Score: 10},
				{Pid: realC.Id, TagType: "Sort", Name: "评分", Value: "score", Score: 10},
				{Pid: realC.Id, TagType: "Sort", Name: "最新", Value: "release_stamp", Score: 10},
			}
			for _, s := range defaultSorts {
				db.Mdb.Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "pid"}, {Name: "tag_type"}, {Name: "value"}},
					DoNothing: true,
				}).Create(&s)
			}
		}
	}
}
