package repository

import (
	"fmt"
	"server/internal/infra/db"
	"server/internal/model"

	"gorm.io/gorm"
)

// SaveCategoryTree 批量保存并同步分类树 (内存指针对齐 + 二阶段批量入库)
// 这种方式完全不依赖硬编码的虚拟 ID，而是通过节点的内存地址 (Pointer) 在处理过程中建立映射。
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

		// 3. 第一阶段：处理树的第一层子节点 (通常是“电影”、“电视剧”等虚拟大类或原始大类)
		var newRoots []*model.Category
		for _, node := range tree.Children {
			key := fmt.Sprintf("0_%s", node.Name)
			if id, ok := cache[key]; ok {
				pointerToId[node] = id
			} else {
				// 准备批量入库
				c := &model.Category{Name: node.Name, Pid: 0, Show: true}
				newRoots = append(newRoots, c)
				// 预存，tx.Create 之后会自动填入真实 ID
				node.Category = c
			}
		}
		if len(newRoots) > 0 {
			if err := tx.Create(&newRoots).Error; err != nil {
				return err
			}
		}
		// 回填第一层真实 ID 到映射表
		for _, node := range tree.Children {
			if _, ok := pointerToId[node]; !ok {
				pointerToId[node] = node.Category.Id
			}
		}

		// 4. 第二阶段：处理树的第二层及以后 (递归或双层遍历)
		// 考虑到目前大部分采集站是两层结构，我们先实现二级批量入库。
		var newSubs []*model.Category
		for _, rootNode := range tree.Children {
			realPid := pointerToId[rootNode]
			for _, subNode := range rootNode.Children {
				key := fmt.Sprintf("%d_%s", realPid, subNode.Name)
				if id, ok := cache[key]; ok {
					pointerToId[subNode] = id
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
		}

		return nil
	})
}

// GetCategoryTree 获取完整分类树 (从数据库行重建)
func GetCategoryTree() model.CategoryTree {
	var categories []model.Category
	db.Mdb.Find(&categories)

	if len(categories) == 0 {
		return model.CategoryTree{
			Category: &model.Category{Id: 0, Pid: -1, Name: "分类信息", Show: true},
			Children: make([]*model.CategoryTree, 0),
		}
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
	var root *model.CategoryTree
	for _, node := range nodes {
		if node.Pid == -1 {
			root = node
			continue
		}
		if parent, ok := nodes[node.Pid]; ok {
			parent.Children = append(parent.Children, node)
		} else {
			// 如果找不到父级，挂载到根节点 (ID 0)
			if rootNode, ok := nodes[0]; ok && node.Id != 0 {
				rootNode.Children = append(rootNode.Children, node)
			}
		}
	}

	if root != nil {
		return *root
	}

	// 最终兜底，如果没找到 root (Pid -1)
	if rootNode, ok := nodes[0]; ok {
		return *rootNode
	}

	return model.CategoryTree{
		Category: &model.Category{Id: 0, Pid: -1, Name: "分类信息", Show: true},
		Children: make([]*model.CategoryTree, 0),
	}
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
