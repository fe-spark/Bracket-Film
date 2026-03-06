package repository

import (
	"server/internal/infra/db"
	"server/internal/model"

	"gorm.io/gorm"
)

// SaveCategoryTree 批量保存分类树 (层级平铺入库)
func SaveCategoryTree(tree *model.CategoryTree) error {
	var categories []model.Category
	flattenTree(tree, &categories)

	return db.Mdb.Transaction(func(tx *gorm.DB) error {
		// 清空旧分类数据
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.Category{}).Error; err != nil {
			return err
		}
		// 批量插入新数据
		if len(categories) > 0 {
			if err := tx.Create(&categories).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// flattenTree 递归平铺分类树
func flattenTree(node *model.CategoryTree, list *[]model.Category) {
	if node == nil || node.Category == nil {
		return
	}
	*list = append(*list, *node.Category)
	for _, child := range node.Children {
		flattenTree(child, list)
	}
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
