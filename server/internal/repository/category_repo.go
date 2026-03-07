package repository

import (
	"fmt"
	"server/internal/infra/db"
	"server/internal/model"

	"gorm.io/gorm"
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
	for i := 0; i < 5; i++ {
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
	// 事务提交成功后，同步更新内存临时映射
	RefreshCategoryCache()
	return err
}

// buildTreeHelper 内部辅助函数：直接从列表构建树形结构内存模型
func buildTreeHelper(all []model.Category) model.CategoryTree {
	nodes := make(map[int64]*model.CategoryTree)
	for _, c := range all {
		item := c
		nodes[item.Id] = &model.CategoryTree{
			Category: &item,
			Children: make([]*model.CategoryTree, 0),
		}
	}

	root := model.CategoryTree{
		Category: &model.Category{Id: 0, Pid: -1, Name: "分类信息", Show: true},
		Children: make([]*model.CategoryTree, 0),
	}

	for _, c := range all {
		node := nodes[c.Id]
		if c.Pid == 0 {
			root.Children = append(root.Children, node)
		} else if parent, ok := nodes[c.Pid]; ok {
			parent.Children = append(parent.Children, node)
		}
	}
	return root
}

// GetCategoryTree 获取完整分类树副本 (实时查库，不走长期缓存)
func GetCategoryTree() model.CategoryTree {
	var all []model.Category
	db.Mdb.Find(&all)
	return buildTreeHelper(all)
}

// GetActiveCategoryTree 获取仅包含有影视内容的分类树副本 (实时查库)
func GetActiveCategoryTree() model.CategoryTree {
	// 1. 获取所有配置显示且未删除的分类
	var all []model.Category
	db.Mdb.Where("`show` = ?", true).Find(&all)

	// 2. 实时查出哪些 Pid 和 Cid 下面真的存有视频记录 (DISTINCT)
	var activeIds []int64
	db.Mdb.Raw("SELECT DISTINCT pid FROM search_info UNION SELECT DISTINCT cid FROM search_info").Pluck("pid", &activeIds)

	activeMap := make(map[int64]bool)
	for _, id := range activeIds {
		activeMap[id] = true
	}

	// 3. 内存构建树
	nodes := make(map[int64]*model.CategoryTree)
	for _, c := range all {
		item := c
		nodes[item.Id] = &model.CategoryTree{
			Category: &item,
			Children: make([]*model.CategoryTree, 0),
		}
	}

	root := model.CategoryTree{
		Category: &model.Category{Id: 0, Pid: -1, Name: "分类信息", Show: true},
		Children: make([]*model.CategoryTree, 0),
	}

	// 4. 第一遍：挂载子类
	for _, c := range all {
		if c.Pid != 0 {
			if parent, ok := nodes[c.Pid]; ok && activeMap[c.Id] {
				parent.Children = append(parent.Children, nodes[c.Id])
			}
		}
	}

	// 5. 第二遍：挂载有数据或有子类的大类到根部
	for _, c := range all {
		if c.Pid == 0 {
			node := nodes[c.Id]
			if activeMap[c.Id] || len(node.Children) > 0 {
				root.Children = append(root.Children, node)
			}
		}
	}

	return root
}

// ExistsCategoryTree 查询分类信息是否存在
func ExistsCategoryTree() bool {
	var count int64
	db.Mdb.Table(model.TableCategory).Count(&count)
	return count > 0
}

// GetChildrenTree 获取对应主分类下的子分类列表 (实时查库)
func GetChildrenTree(pid int64) []*model.CategoryTree {
	var all []model.Category
	db.Mdb.Find(&all)
	tree := buildTreeHelper(all)

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
