package repository

import (
	"encoding/json"

	"server/internal/infra/db"
	"server/internal/model"
)

// SaveCategoryTree 保存影片分类信息 (MySQL 持久化)
func SaveCategoryTree(tree *model.CategoryTree) error {
	data, _ := json.Marshal(tree)
	if err := db.Mdb.Save(&model.CategoryPersistent{Content: string(data)}).Error; err != nil {
		return err
	}
	return nil
}

// GetCategoryTree 获取影片分类信息（直查 MySQL）
func GetCategoryTree() model.CategoryTree {
	var cp model.CategoryPersistent
	tree := model.CategoryTree{}
	if err := db.Mdb.Order("id DESC").First(&cp).Error; err != nil {
		return tree
	}
	_ = json.Unmarshal([]byte(cp.Content), &tree)
	return tree
}

// ExistsCategoryTree 查询分类信息是否存在（直查 MySQL）
func ExistsCategoryTree() bool {
	var count int64
	db.Mdb.Model(&model.CategoryPersistent{}).Count(&count)
	return count > 0
}

// GetChildrenTree 根据影片Id获取对应分类的子分类信息
func GetChildrenTree(id int64) []*model.CategoryTree {
	tree := GetCategoryTree()
	for _, t := range tree.Children {
		if t.Id == id {
			return t.Children
		}
	}
	return nil
}

// GetParentId 获取指定分类的父级 ID，如果本身是一级分类则返回其自身 ID
func GetParentId(id int64) int64 {
	tree := GetCategoryTree()
	for _, t := range tree.Children {
		if t.Id == id {
			return t.Id
		}
		for _, c := range t.Children {
			if c.Id == id {
				return t.Id
			}
		}
	}
	return id
}
