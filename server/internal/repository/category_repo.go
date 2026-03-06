package repository

import (
	"fmt"
	"server/internal/infra/db"
	"server/internal/model"

	"sync"
	"sync/atomic"

	"gorm.io/gorm"
)

// catCache 用于加速分类名到 ID 的查找，避免大规模入库时的频繁 DB 查询
var catCache atomic.Value // 存储 map[string]int64
var cacheMu sync.Mutex

func refreshCache() {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	var all []model.Category
	db.Mdb.Find(&all)
	m := make(map[string]int64)
	for _, c := range all {
		// 子类具有更高的查找优先级 (Pid > 0)
		if c.Pid > 0 {
			m[c.Name] = c.Id
		} else if _, ok := m[c.Name]; !ok {
			// 如果该名称还没存入子类 ID，则存入大类 ID (Pid = 0)
			m[c.Name] = c.Id
		}
	}
	catCache.Store(m)
}

// GetCidByName 根据分类名称查找对应的类 ID (优先从内存缓存中获取)
func GetCidByName(name string) int64 {
	val := catCache.Load()
	if val == nil {
		refreshCache()
		val = catCache.Load()
	}
	m := val.(map[string]int64)
	if id, ok := m[name]; ok {
		return id
	}
	return 0
}

// SaveCategoryTree 批量保存并同步分类树 (内存指针对齐 + 二阶段批量入库)
// 这种方式将采集站分类与本地数据库分类通过 Name 进行对齐，确保 ID 永久稳定，不随采集站变化。
func SaveCategoryTree(tree *model.CategoryTree) error {
	if tree == nil {
		return nil
	}

	return db.Mdb.Transaction(func(tx *gorm.DB) error {
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
	return nil
}

// GetCategoryTree 获取完整分类树 (从数据库行重建)
func GetCategoryTree() model.CategoryTree {
	var categories []model.Category
	db.Mdb.Find(&categories)

	root := model.CategoryTree{
		Category: &model.Category{Id: 0, Pid: -1, Name: "分类信息", Show: true},
		Children: make([]*model.CategoryTree, 0),
	}

	if len(categories) == 0 {
		return root
	}

	// 1. 构建节点 Map
	nodes := make(map[int64]*model.CategoryTree)
	for i := range categories {
		nodes[categories[i].Id] = &model.CategoryTree{
			Category: &categories[i],
			Children: make([]*model.CategoryTree, 0),
		}
	}

	// 2. 建立层级关系
	for _, node := range nodes {
		if node.Pid == 0 {
			// 一级大类，挂载到统一虚拟根
			root.Children = append(root.Children, node)
		} else if parent, ok := nodes[node.Pid]; ok {
			// 二级子类，挂载到对应的一级大类下
			parent.Children = append(parent.Children, node)
		} else {
			// 如果找不到父级，兜底挂载到虚拟根
			root.Children = append(root.Children, node)
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

// GetChildrenTree 根据影片Id获取对应分类的子分类信息
func GetChildrenTree(pid int64) []*model.CategoryTree {
	var categories []model.Category
	db.Mdb.Where("pid = ?", pid).Find(&categories)

	res := make([]*model.CategoryTree, 0)
	for i := range categories {
		res = append(res, &model.CategoryTree{
			Category: &categories[i],
			Children: nil,
		})
	}
	return res
}

// GetParentId 获取指定分类的父级 ID
func GetParentId(id int64) int64 {
	var category model.Category
	if err := db.Mdb.Where("id = ?", id).First(&category).Error; err != nil {
		return 0
	}
	return category.Pid
}
