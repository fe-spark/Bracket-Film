package repository

import (
	"fmt"
	"server/internal/infra/db"
	"server/internal/model"

	"sync"
	"sync/atomic"

	"gorm.io/gorm"
)

type catCacheData struct {
	nameToId map[string]int64
	idToPid  map[int64]int64
	tree     *model.CategoryTree
}

// catCache 用于加速分类数据查找
var catCache atomic.Value // 存储 *catCacheData
var cacheMu sync.Mutex

// RefreshCategoryCache 用于重新加载分类缓存到内存
func RefreshCategoryCache() {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	var all []model.Category
	db.Mdb.Find(&all)

	data := &catCacheData{
		nameToId: make(map[string]int64),
		idToPid:  make(map[int64]int64),
	}

	nodes := make(map[int64]*model.CategoryTree)
	root := &model.CategoryTree{
		Category: &model.Category{Id: 0, Pid: -1, Name: "分类信息", Show: true},
		Children: make([]*model.CategoryTree, 0),
	}

	for _, c := range all {
		item := c
		data.idToPid[item.Id] = item.Pid
		// 子类具有更高的查找优先级 (Pid > 0)
		if item.Pid > 0 {
			data.nameToId[item.Name] = item.Id
		} else if _, ok := data.nameToId[item.Name]; !ok {
			// 如果该名称还没存入子类 ID，则存入大类 ID (Pid = 0)
			data.nameToId[item.Name] = item.Id
		}

		nodes[item.Id] = &model.CategoryTree{
			Category: &item,
			Children: make([]*model.CategoryTree, 0),
		}
	}

	for _, c := range all {
		node := nodes[c.Id]
		if node.Pid == 0 {
			root.Children = append(root.Children, node)
		} else if parent, ok := nodes[node.Pid]; ok {
			parent.Children = append(parent.Children, node)
		} else {
			root.Children = append(root.Children, node)
		}
	}
	data.tree = root
	catCache.Store(data)
}

// GetCidByName 根据分类名称查找对应的类 ID (优先从内存缓存中获取)
func GetCidByName(name string) int64 {
	val := catCache.Load()
	if val == nil {
		RefreshCategoryCache()
		val = catCache.Load()
	}
	data := val.(*catCacheData)
	if id, ok := data.nameToId[name]; ok {
		return id
	}
	return 0
}

// GetRootId 获取分类的顶级根 ID (通过缓存递归或一次性查出)
func GetRootId(id int64) int64 {
	if id == 0 {
		return 0
	}
	val := catCache.Load()
	if val == nil {
		RefreshCategoryCache()
		val = catCache.Load()
	}
	data := val.(*catCacheData)

	curr := id
	// 为防止循环引用死循环，最多查找 5 层 (目前本项目只有 2 层)
	for i := 0; i < 5; i++ {
		p, ok := data.idToPid[curr]
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
	val := catCache.Load()
	if val == nil {
		RefreshCategoryCache()
		val = catCache.Load()
	}
	data := val.(*catCacheData)
	p, ok := data.idToPid[id]
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

		// 2. 指针映射表：节点指针 -> 真实数据库 ID (用于在后续层级中找父 ID)
		pointerToId := make(map[*model.CategoryTree]int64)
		pointerToId[tree] = 0 // 树根 (Pid=-1) 的逻辑 ID 对应 0

		// 3. 第一阶段：处理树的第一层子节点 (通常是“电影”、“电视剧”等根分类)
		var newRoots []*model.Category
		for _, node := range tree.Children {
			key := fmt.Sprintf("0_%s", node.Name)
			if id, ok := cache[key]; ok {
				pointerToId[node] = id
				// 关键点：回填真实 ID 到内存对象中，供后续流程使用
				node.Id = id
			} else {
				// 准备批量入库 (不带 ID，交由 DB 自增)
				c := &model.Category{Name: node.Name, Pid: 0, Show: true}
				newRoots = append(newRoots, c)
				node.Category = c
			}
		}
		if len(newRoots) > 0 {
			if err := tx.Create(&newRoots).Error; err != nil {
				return err
			}
		}
		// 回填新入库大类的真实 ID 到映射表和 node 对象
		for _, node := range tree.Children {
			if _, ok := pointerToId[node]; !ok {
				pointerToId[node] = node.Category.Id
				node.Id = node.Category.Id
			}
		}

		// 4. 第二阶段：处理树的第二层 (子类)
		var newSubs []*model.Category
		for _, rootNode := range tree.Children {
			realPid := pointerToId[rootNode]
			for _, subNode := range rootNode.Children {
				key := fmt.Sprintf("%d_%s", realPid, subNode.Name)
				if id, ok := cache[key]; ok {
					pointerToId[subNode] = id
					subNode.Id = id
					subNode.Pid = realPid
				} else {
					subC := &model.Category{Name: subNode.Name, Pid: realPid, Show: true}
					newSubs = append(newSubs, subC)
					subNode.Category = subC
				}
			}
		}
		if len(newSubs) > 0 {
			if err := tx.Create(&newSubs).Error; err != nil {
				return err
			}
			// 回填子类 ID
			for _, rootNode := range tree.Children {
				for _, subNode := range rootNode.Children {
					if subNode.Category != nil && subNode.Id == 0 {
						subNode.Id = subNode.Category.Id
						subNode.Pid = pointerToId[rootNode]
					}
				}
			}
		}

		return nil
	})
	// 事务提交成功后，从库中重新加载缓存
	RefreshCategoryCache()
	return err
}

// GetCategoryTree 获取完整分类树 (从缓存重建，返回副本以防外部修改影响缓存)
func GetCategoryTree() model.CategoryTree {
	val := catCache.Load()
	if val == nil {
		RefreshCategoryCache()
		val = catCache.Load()
	}
	data := val.(*catCacheData)

	// 简单的树深复制，目前只有 2 层
	root := model.CategoryTree{
		Category: &model.Category{Id: 0, Pid: -1, Name: "分类信息", Show: true},
		Children: make([]*model.CategoryTree, 0, len(data.tree.Children)),
	}

	for _, c := range data.tree.Children {
		node := &model.CategoryTree{
			Category: c.Category,
			Children: make([]*model.CategoryTree, 0, len(c.Children)),
		}
		for _, sub := range c.Children {
			node.Children = append(node.Children, &model.CategoryTree{
				Category: sub.Category,
				Children: nil,
			})
		}
		root.Children = append(root.Children, node)
	}

	return root
}

// GetActiveCategoryTree 获取仅包含有影视内容的分类树的副本
func GetActiveCategoryTree() model.CategoryTree {
	root := GetCategoryTree() // 先查全量

	// 查出哪些 cid (二级) 和 pid (一级) 下有资源
	var activeCids []int64
	db.Mdb.Model(&model.SearchInfo{}).Select("cid").Group("cid").Find(&activeCids)

	var activePids []int64
	db.Mdb.Model(&model.SearchInfo{}).Select("pid").Group("pid").Find(&activePids)

	// 放进 map 方便查找
	activeMap := make(map[int64]bool)
	for _, id := range activeCids {
		activeMap[id] = true
	}
	for _, id := range activePids {
		activeMap[id] = true
	}

	// 过滤：只有当分类 id 在 activeMap 中，且 show 为 true 时才保留
	// 保留包含有效子分类的一级大类
	filteredRootChildren := make([]*model.CategoryTree, 0)
	for _, rootNode := range root.Children {
		if !rootNode.Show {
			continue
		}

		filteredSubChildren := make([]*model.CategoryTree, 0)
		for _, subNode := range rootNode.Children {
			if subNode.Show && activeMap[subNode.Id] {
				filteredSubChildren = append(filteredSubChildren, subNode)
			}
		}

		rootNode.Children = filteredSubChildren

		// 如果该一级大类自己有视频，或者它包含有效子类，则展示它
		if activeMap[rootNode.Id] || len(rootNode.Children) > 0 {
			filteredRootChildren = append(filteredRootChildren, rootNode)
		}
	}
	root.Children = filteredRootChildren

	return root
}

// ExistsCategoryTree 查询分类信息是否存在
func ExistsCategoryTree() bool {
	var count int64
	db.Mdb.Table(model.TableCategory).Count(&count)
	return count > 0
}

// GetChildrenTree 根据影片Id获取对应分类的子分类信息
func GetChildrenTree(pid int64) []*model.CategoryTree {
	val := catCache.Load()
	if val == nil {
		RefreshCategoryCache()
		val = catCache.Load()
	}
	data := val.(*catCacheData)

	// 查找该 pid 对应的节点，并返回其子分类
	if pid == 0 {
		return data.tree.Children
	}

	for _, c := range data.tree.Children {
		if c.Id == pid {
			return c.Children
		}
	}
	return nil
}

// GetParentId 获取指定分类的父级 ID
func GetParentId(id int64) int64 {
	val := catCache.Load()
	if val == nil {
		RefreshCategoryCache()
		val = catCache.Load()
	}
	data := val.(*catCacheData)
	return data.idToPid[id]
}
